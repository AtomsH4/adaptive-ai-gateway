package application

import (
	"context"
	"errors"
	"time"

	"github.com/AtomsH4/adaptive-ai-gateway/internal/application/api"
	"github.com/AtomsH4/adaptive-ai-gateway/internal/application/ports"
	"github.com/AtomsH4/adaptive-ai-gateway/internal/domain"
)

type Options struct {
	Model                                                                    domain.ModelRef
	Profile                                                                  domain.RuntimeProfileRef
	OwnerID                                                                  string
	ContextTokens                                                            int
	RequiredFreeBytes                                                        uint64
	QueueLimit                                                               int
	QueueTimeout, LoadTimeout, ExecutionTimeout, RequestTimeout              time.Duration
	IdleTimeout, CriticalGrace, CancelGrace, RecoveryWindow, ShutdownTimeout time.Duration
}

type pending struct {
	request        api.GenerateRequest
	ctx            context.Context
	out            *output
	accepted, last time.Time
	queued         time.Duration
	loading        bool
}
type active struct {
	*pending
	ref               domain.ExecutionRef
	run               ports.GenerationRun
	started, cancelAt time.Time
	cancelCode        string
}
type submit struct {
	ctx     context.Context
	request api.GenerateRequest
	reply   chan submitReply
}
type submitReply struct {
	out *output
	err error
}
type query struct{ reply chan api.SchedulerStatus }
type cancelRequest struct{ out *output }
type shutdown struct{}
type loaded struct {
	ref         domain.InstanceRef
	observation domain.InstanceObservation
	err         error
}
type stopped struct {
	ref      domain.InstanceRef
	evidence domain.ReleaseEvidence
	err      error
}
type began struct {
	ref domain.ExecutionRef
	run ports.GenerationRun
	err error
}
type generated struct {
	ref   domain.ExecutionRef
	event domain.GenerationEvent
}
type released struct {
	ref      domain.ExecutionRef
	evidence domain.ReleaseEvidence
	err      error
}
type inspected struct {
	ref         domain.InstanceRef
	observation domain.InstanceObservation
	err         error
}

// Coordinator owns all mutable capacity state on one command loop.
// Adapters report evidence; no other goroutine can release its reservations.
type Coordinator struct {
	opts                                                                            Options
	backend                                                                         ports.Backend
	reader                                                                          ports.ResourceReader
	evaluator                                                                       ports.SignalEvaluator
	clock                                                                           ports.Clock
	inbox                                                                           chan any
	done                                                                            chan struct{}
	workCtx                                                                         context.Context
	workCancel                                                                      context.CancelFunc
	waiting                                                                         []*pending
	current                                                                         *active
	instance                                                                        domain.InstanceRef
	epoch                                                                           uint64
	executionSequence                                                               uint64
	nextStop                                                                        time.Time
	state, protection, reason                                                       string
	loadCancel                                                                      context.CancelFunc
	loadAt, idleAt, criticalAt, normalSince, lastNormalSample, nextLoad, shutdownAt time.Time
	normalSamples                                                                   int
	reclaim, wasCritical, closing, stopInFlight                                     bool
	decisions                                                                       []domain.Decision
	failedStarts                                                                    []time.Time
	probeAt                                                                         time.Time
	probing                                                                         bool
}

func NewCoordinator(opts Options, backend ports.Backend, reader ports.ResourceReader, evaluator ports.SignalEvaluator, clock ports.Clock) (*Coordinator, error) {
	if backend == nil || reader == nil || evaluator == nil || clock == nil || opts.OwnerID == "" || opts.Model.ID == "" || opts.QueueLimit < 1 || opts.QueueLimit > 4 || opts.ContextTokens < 512 || opts.RequiredFreeBytes == 0 {
		return nil, errors.New("invalid coordinator dependencies or capacity")
	}
	for _, d := range []time.Duration{opts.QueueTimeout, opts.LoadTimeout, opts.ExecutionTimeout, opts.RequestTimeout, opts.IdleTimeout, opts.CriticalGrace, opts.CancelGrace, opts.RecoveryWindow, opts.ShutdownTimeout} {
		if d <= 0 {
			return nil, errors.New("all coordinator budgets must be positive")
		}
	}
	d := backend.Description()
	if d.Ownership != domain.OwnershipManaged || !d.Capabilities.CanStopOwnedInstance || !d.Capabilities.CanObserveExecutionEnd || !d.Capabilities.CanValidateContextLimit {
		return nil, errors.New("MVP requires a managed backend with stop, release and context validation capabilities")
	}
	ctx, cancel := context.WithCancel(context.Background())
	c := &Coordinator{opts: opts, backend: backend, reader: reader, evaluator: evaluator, clock: clock, inbox: make(chan any, 128), done: make(chan struct{}), workCtx: ctx, workCancel: cancel, state: "unloaded", protection: "observation_guard"}
	go c.loop()
	return c, nil
}

func (c *Coordinator) send(ctx context.Context, v any) error {
	select {
	case <-c.done:
		return &domain.Failure{Code: "service_stopping", Message: "Gateway is stopping."}
	case <-ctx.Done():
		return ctx.Err()
	case c.inbox <- v:
		return nil
	}
}
func (c *Coordinator) Generate(ctx context.Context, r api.GenerateRequest) (api.Output, error) {
	if r.Model != c.opts.Model.ID {
		return nil, failure("model_not_found")
	}
	if r.ID == "" || len(r.Messages) == 0 || r.MaxOutputTokens < 1 || r.MaxOutputTokens > 512 || !(r.Temperature >= 0 && r.Temperature <= 2) {
		return nil, failure("invalid_request")
	}
	for _, message := range r.Messages {
		if message.Role != "system" && message.Role != "user" && message.Role != "assistant" {
			return nil, failure("invalid_request")
		}
	}
	r.Messages = append([]domain.Message(nil), r.Messages...)
	reply := make(chan submitReply, 1)
	if err := c.send(ctx, submit{ctx, r, reply}); err != nil {
		return nil, err
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-c.done:
		return nil, failure("service_stopping")
	case result := <-reply:
		return result.out, result.err
	}
}
func (c *Coordinator) Status(ctx context.Context) (api.SchedulerStatus, error) {
	reply := make(chan api.SchedulerStatus, 1)
	if err := c.send(ctx, query{reply}); err != nil {
		return api.SchedulerStatus{}, err
	}
	select {
	case <-ctx.Done():
		return api.SchedulerStatus{}, ctx.Err()
	case <-c.done:
		return api.SchedulerStatus{}, failure("service_stopping")
	case s := <-reply:
		return s, nil
	}
}
func (c *Coordinator) Close(ctx context.Context) error {
	if err := c.send(ctx, shutdown{}); err != nil {
		select {
		case <-c.done:
			return nil
		default:
			return err
		}
	}
	select {
	case <-c.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
func failure(code string) *domain.Failure {
	return &domain.Failure{Code: code, Message: "Request ended: " + code + "."}
}
func (c *Coordinator) loop() {
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	defer c.workCancel()
	defer close(c.done)
	for {
		select {
		case v := <-c.inbox:
			c.refresh()
			c.handle(v, c.clock.Now())
		case <-ticker.C:
		}
		c.step()
		if c.closing && c.instance.ID == "" && c.current == nil {
			return
		}
	}
}
