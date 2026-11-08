package application

import (
	"context"
	"github.com/AtomsH4/adaptive-ai-gateway/internal/domain"
	"time"
)

func (c *Coordinator) refresh() time.Time {
	oldProtection, oldReason := c.protection, c.reason
	defer func() {
		if oldProtection != c.protection || oldReason != c.reason {
			c.record("protection_" + c.protection)
		}
	}()
	snapshot, err := c.reader.Read(c.workCtx)
	now := c.clock.Now()
	if err != nil {
		snapshot.MemoryPressure = domain.Observation[domain.MemoryPressure]{}
	}
	signal := c.evaluator.Evaluate(snapshot, now)
	c.updateProtection(signal, snapshot.MemoryPressure.ObservedAt, now)
	if c.state == "unloaded" && c.protection == "normal" {
		memory := snapshot.AvailableMemoryBytes
		if memory.Quality != domain.Available || memory.Source == "" || memory.ObservedAt.IsZero() || now.Sub(memory.ObservedAt) > 6*time.Second || memory.ObservedAt.After(now) {
			c.protection, c.reason = "observation_guard", "telemetry_unavailable"
		} else if memory.Value < c.opts.RequiredFreeBytes {
			c.protection, c.reason = "constrained", "memory_pressure"
		}
	}
	kept := c.waiting[:0]
	for _, p := range c.waiting {
		if !p.loading {
			p.queued += now.Sub(p.last)
		}
		p.last = now
		code := ""
		switch {
		case p.ctx.Err() != nil:
			code = "request_cancelled"
		case now.Sub(p.accepted) >= c.opts.RequestTimeout:
			code = "request_timeout"
		case p.loading && c.state == "loading" && now.Sub(c.loadAt) >= c.opts.LoadTimeout:
			code = "model_load_timeout"
		case !p.loading && p.queued >= c.opts.QueueTimeout:
			code = "queue_timeout"
			if c.protection != "normal" && c.reason != "" {
				code = c.reason
			}
		}
		if code != "" {
			p.out.end(failure(code))
		} else {
			kept = append(kept, p)
		}
	}
	c.waiting = kept
	for _, p := range c.waiting {
		p.loading = c.state == "loading"
	}
	if c.current != nil {
		a := c.current
		switch {
		case a.ctx.Err() != nil:
			c.cancelActive(now, "request_cancelled")
		case now.Sub(a.accepted) >= c.opts.RequestTimeout:
			c.cancelActive(now, "request_timeout")
		case now.Sub(a.started) >= c.opts.ExecutionTimeout:
			c.cancelActive(now, "execution_timeout")
		}
		if !a.cancelAt.IsZero() && now.Sub(a.cancelAt) >= c.opts.CancelGrace {
			c.stop()
		}
	}
	return now
}

func (c *Coordinator) step() {
	now := c.refresh()
	if c.state == "ready" && !c.closing && !c.probing && now.Sub(c.probeAt) >= 2*time.Second {
		c.probing = true
		c.probeAt = now
		ref := c.instance
		go func() {
			ctx, cancel := context.WithTimeout(c.workCtx, 600*time.Millisecond)
			defer cancel()
			observation, err := c.backend.InspectInstance(ctx, ref)
			_ = c.send(c.workCtx, inspected{ref, observation, err})
		}()
	}
	if c.closing {
		if c.current == nil {
			c.stop()
		} else if now.Sub(c.shutdownAt) >= c.opts.ShutdownTimeout-c.opts.CancelGrace {
			c.cancelActive(now, "service_stopping")
			c.stop()
		}
	} else if c.reclaim {
		c.failWaiting("memory_pressure")
		c.cancelActive(now, "memory_pressure_interrupted")
		c.stop()
	} else if !c.criticalAt.IsZero() {
		c.failWaiting("memory_pressure")
		if c.current == nil || now.Sub(c.criticalAt) >= c.opts.CriticalGrace {
			c.reclaim = true
			c.wasCritical = true
			c.cancelActive(now, "memory_pressure_interrupted")
			c.stop()
		}
	} else if c.state == "loading" && (len(c.waiting) == 0 || now.Sub(c.loadAt) >= c.opts.LoadTimeout) {
		if now.Sub(c.loadAt) >= c.opts.LoadTimeout {
			c.failWaiting("model_load_timeout")
			c.recordFailedStart(now)
		}
		c.stop()
	} else if c.state == "ready" && c.current == nil && len(c.waiting) == 0 && now.Sub(c.idleAt) >= c.opts.IdleTimeout {
		c.record("idle_unload")
		c.stop()
	}
	if c.state == "stopping" && !c.stopInFlight {
		c.stop()
	}
	if c.closing || c.protection != "normal" {
		return
	}
	if c.state == "unloaded" && len(c.waiting) > 0 && !now.Before(c.nextLoad) {
		c.startLoad(now)
	}
	if c.state == "ready" && c.current == nil && len(c.waiting) > 0 {
		c.startExecution(now)
	}
	for _, p := range c.waiting {
		p.loading = c.state == "loading"
	}
}
