package application

import (
	"context"
	"fmt"
	"github.com/AtomsH4/adaptive-ai-gateway/internal/application/ports"
	"github.com/AtomsH4/adaptive-ai-gateway/internal/domain"
	"time"
)

func (c *Coordinator) startLoad(now time.Time) {
	c.record("model_load_started")
	c.epoch++
	c.instance = domain.InstanceRef{ID: c.opts.OwnerID, Epoch: c.epoch}
	c.loadAt = now
	c.state = "loading"
	ctx, cancel := context.WithCancel(c.workCtx)
	c.loadCancel = cancel
	ref := c.instance
	command := domain.LoadCommand{Instance: ref, Model: c.opts.Model, Profile: c.opts.Profile, ContextTokens: c.opts.ContextTokens, MaxConcurrency: 1}
	for _, p := range c.waiting {
		p.loading = true
		p.last = now
	}
	go func() {
		observation, err := c.backend.Load(ctx, command)
		_ = c.send(c.workCtx, loaded{ref, observation, err})
	}()
}
func (c *Coordinator) startExecution(now time.Time) {
	c.record("execution_started")
	p := c.waiting[0]
	c.waiting = c.waiting[1:]
	p.loading = false
	c.executionSequence++
	ref := domain.ExecutionRef{Instance: c.instance, ID: fmt.Sprintf("execution-%d", c.executionSequence)}
	c.current = &active{pending: p, ref: ref, started: now}
	cmd := domain.ExecutionCommand{Execution: ref, Model: c.opts.Model, Profile: c.opts.Profile, Messages: p.request.Messages, MaxOutputTokens: p.request.MaxOutputTokens, Temperature: p.request.Temperature}
	go func() { run, err := c.backend.Begin(c.workCtx, cmd); _ = c.send(c.workCtx, began{ref, run, err}) }()
}
func (c *Coordinator) pump(ref domain.ExecutionRef, run ports.GenerationRun) {
	for {
		e, err := run.Next(c.workCtx)
		if err != nil {
			break
		}
		if c.send(c.workCtx, generated{ref, e}) != nil {
			return
		}
		if e.Kind == domain.GenerationFinished || e.Kind == domain.GenerationFailed {
			break
		}
	}
	e, err := run.AwaitRelease(c.workCtx)
	_ = c.send(c.workCtx, released{ref, e, err})
}
func (c *Coordinator) cancelRun(run ports.GenerationRun) {
	go func() {
		ctx, cancel := context.WithTimeout(c.workCtx, c.opts.CancelGrace)
		defer cancel()
		_ = run.Cancel(ctx)
	}()
}
func (c *Coordinator) cancelActive(now time.Time, code string) {
	if c.current == nil || !c.current.cancelAt.IsZero() {
		return
	}
	a := c.current
	a.cancelAt = now
	a.cancelCode = code
	c.record(code)
	a.out.end(failure(code))
	if a.run != nil {
		c.cancelRun(a.run)
	}
}
func (c *Coordinator) failWaiting(code string) {
	for _, p := range c.waiting {
		p.out.end(failure(code))
	}
	c.waiting = nil
}
func (c *Coordinator) stop() {
	if c.instance.ID == "" || c.stopInFlight || c.clock.Now().Before(c.nextStop) {
		return
	}
	c.state = "stopping"
	c.stopInFlight = true
	c.record("engine_stop_requested")
	if c.loadCancel != nil {
		c.loadCancel()
	}
	ref := c.instance
	go func() {
		ctx, cancel := context.WithTimeout(c.workCtx, c.opts.CancelGrace)
		defer cancel()
		e, err := c.backend.Stop(ctx, ref)
		_ = c.send(c.workCtx, stopped{ref, e, err})
	}()
}
func (c *Coordinator) recordFailedStart(now time.Time) {
	c.record("backend_start_failed")
	kept := c.failedStarts[:0]
	for _, t := range c.failedStarts {
		if now.Sub(t) < time.Minute {
			kept = append(kept, t)
		}
	}
	c.failedStarts = append(kept, now)
	c.nextLoad = now.Add(time.Duration(len(c.failedStarts)) * time.Second)
}
