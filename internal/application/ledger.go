package application

import (
	"context"
	"github.com/AtomsH4/adaptive-ai-gateway/internal/application/api"
	"github.com/AtomsH4/adaptive-ai-gateway/internal/domain"
	"time"
)

func (c *Coordinator) handle(v any, now time.Time) {
	switch v := v.(type) {
	case submit:
		if c.closing {
			v.reply <- submitReply{err: failure("service_stopping")}
			return
		}
		if c.state == "failed" {
			v.reply <- submitReply{err: failure("engine_unavailable")}
			return
		}
		if c.protection == "reclaiming" {
			v.reply <- submitReply{err: failure("memory_pressure")}
			return
		}
		if len(c.waiting) >= c.opts.QueueLimit {
			c.record("queue_full")
			v.reply <- submitReply{err: failure("queue_full")}
			return
		}
		var out *output
		pendingCtx, cancel := context.WithCancel(v.ctx)
		out = newOutput(func() { cancel(); _ = c.send(context.Background(), cancelRequest{out}) })
		c.record("request_queued")
		c.waiting = append(c.waiting, &pending{request: v.request, ctx: pendingCtx, out: out, accepted: now, last: now, loading: c.state == "loading"})
		v.reply <- submitReply{out: out}
	case query:
		v.reply <- c.status()
	case cancelRequest:
		for i, p := range c.waiting {
			if p.out == v.out {
				p.out.end(context.Canceled)
				c.waiting = append(c.waiting[:i], c.waiting[i+1:]...)
				break
			}
		}
		if c.current != nil && c.current.out == v.out {
			c.cancelActive(now, "request_cancelled")
		}
	case shutdown:
		if !c.closing {
			c.closing = true
			c.shutdownAt = now
			c.failWaiting("service_stopping")
		}
	case loaded:
		if v.ref != c.instance {
			return
		}
		if c.state == "stopping" {
			return
		}
		if now.Sub(c.loadAt) >= c.opts.LoadTimeout {
			c.failWaiting("model_load_timeout")
			c.recordFailedStart(now)
			c.stop()
			return
		}
		if len(c.waiting) == 0 {
			c.stop()
			return
		}
		if v.err != nil || v.observation.Instance != v.ref || v.observation.State != domain.InstanceReady {
			c.failWaiting("model_load_failed")
			c.recordFailedStart(now)
			c.stop()
			return
		}
		c.state = "ready"
		c.record("model_ready")
		c.idleAt = now
	case stopped:
		if v.ref != c.instance {
			return
		}
		c.stopInFlight = false
		if v.err != nil || v.evidence.Instance != v.ref || v.evidence.Kind != domain.EvidenceInstanceExited || v.evidence.ObservedAt.IsZero() {
			c.nextStop = now.Add(time.Second)
			return
		}
		if c.current != nil {
			c.current.out.end(failure("upstream_error"))
			c.current = nil
		}
		c.record("instance_exit_confirmed")
		c.instance = domain.InstanceRef{}
		c.loadCancel = nil
		c.probing = false
		c.probeAt = time.Time{}
		if len(c.failedStarts) >= 3 {
			c.state = "failed"
			c.failWaiting("engine_unavailable")
		} else {
			c.state = "unloaded"
		}
	case began:
		if c.current == nil || v.ref != c.current.ref {
			return
		}
		if v.err != nil || v.run == nil || v.run.Reference() != v.ref {
			c.current.out.end(failure("upstream_error"))
			c.cancelActive(now, "upstream_error")
			return
		}
		c.current.run = v.run
		if !c.current.cancelAt.IsZero() {
			c.cancelRun(v.run)
		}
		go c.pump(v.ref, v.run)
	case generated:
		if c.current == nil || v.ref != c.current.ref || v.event.Execution != v.ref || !c.current.cancelAt.IsZero() {
			return
		}
		if v.event.Kind == domain.GenerationFailed {
			c.current.out.end(failure(v.event.FailureCode))
			c.cancelActive(now, "upstream_error")
			return
		}
		if !c.current.out.put(v.event) {
			c.cancelActive(now, "slow_consumer")
			return
		}
		if v.event.Kind == domain.GenerationFinished {
			c.current.out.end(nil)
		}
	case released:
		if c.current == nil || v.ref != c.current.ref {
			return
		}
		if v.err != nil || !v.evidence.ConfirmsExecution(v.ref) {
			c.cancelActive(now, "upstream_error")
			return
		}
		c.current.out.end(failure("upstream_error"))
		c.current = nil
		c.idleAt = now
	case inspected:
		if v.ref != c.instance {
			return
		}
		c.probing = false
		if c.state == "ready" && v.err == nil && v.observation.Instance == v.ref && v.observation.State == domain.InstanceExited {
			c.failWaiting("engine_unavailable")
			if c.current != nil {
				c.current.out.end(failure("upstream_error"))
			}
			c.recordFailedStart(now)
			c.stop()
		}
	}
}

func (c *Coordinator) status() api.SchedulerStatus {
	s := api.SchedulerStatus{ServiceReady: !c.closing && c.state != "failed", EngineState: c.state, ProtectionState: c.protection, WaitingRequests: len(c.waiting), ReasonCodes: []string{}}
	if c.current != nil {
		s.ActiveExecutions = 1
	}
	if c.reason != "" {
		s.ReasonCodes = append(s.ReasonCodes, c.reason)
	}
	s.CanExecuteNow = s.ServiceReady && c.state == "ready" && c.protection == "normal" && c.current == nil
	s.CanQueue = s.ServiceReady && c.protection != "reclaiming" && len(c.waiting) < c.opts.QueueLimit
	s.RecentDecisions = append([]domain.Decision{}, c.decisions...)
	return s
}

func (c *Coordinator) record(code string) {
	if len(c.decisions) == 128 {
		copy(c.decisions, c.decisions[1:])
		c.decisions = c.decisions[:127]
	}
	c.decisions = append(c.decisions, domain.Decision{At: c.clock.Now(), Code: code})
}
