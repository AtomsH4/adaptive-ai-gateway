package fake

import (
	"context"
	"errors"
	"io"
	"sync"
	"time"

	"github.com/AtomsH4/adaptive-ai-gateway/internal/application/ports"
	"github.com/AtomsH4/adaptive-ai-gateway/internal/domain"
)

// Backend is an explicit synthetic backend. It never loads model weights.
type Backend struct {
	mu                    sync.Mutex
	instance              domain.InstanceRef
	run                   *fakeRun
	LoadDelay, TokenDelay time.Duration
	IgnoreCancellation    bool
	Loads                 int
}

func NewBackend() *Backend {
	return &Backend{LoadDelay: 40 * time.Millisecond, TokenDelay: 10 * time.Millisecond}
}
func (b *Backend) Description() domain.BackendDescription {
	return domain.BackendDescription{Name: "synthetic", ProtocolVersion: "1", Ownership: domain.OwnershipManaged, Capabilities: domain.BackendCapabilities{CanStopOwnedInstance: true, CanConfirmCancellation: true, CanObserveExecutionEnd: true, CanValidateContextLimit: true}}
}
func (b *Backend) Load(ctx context.Context, cmd domain.LoadCommand) (domain.InstanceObservation, error) {
	b.mu.Lock()
	if b.instance.ID != "" {
		b.mu.Unlock()
		return domain.InstanceObservation{}, errors.New("instance already owned")
	}
	b.instance = cmd.Instance
	b.Loads++
	b.mu.Unlock()
	select {
	case <-ctx.Done():
		return domain.InstanceObservation{}, ctx.Err()
	case <-time.After(b.LoadDelay):
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.instance != cmd.Instance {
		return domain.InstanceObservation{}, errors.New("instance stopped")
	}
	return domain.InstanceObservation{Instance: cmd.Instance, State: domain.InstanceReady}, nil
}
func (b *Backend) InspectInstance(ctx context.Context, ref domain.InstanceRef) (domain.InstanceObservation, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	state := domain.InstanceExited
	if ref == b.instance {
		state = domain.InstanceReady
	}
	return domain.InstanceObservation{Instance: ref, State: state}, ctx.Err()
}
func (b *Backend) Stop(ctx context.Context, ref domain.InstanceRef) (domain.ReleaseEvidence, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.instance == ref {
		if b.run != nil {
			b.run.finish(domain.EvidenceInstanceExited)
		}
		b.instance = domain.InstanceRef{}
		b.run = nil
	}
	return domain.ReleaseEvidence{Instance: ref, Kind: domain.EvidenceInstanceExited, ObservedAt: time.Now()}, nil
}
func (b *Backend) Begin(ctx context.Context, cmd domain.ExecutionCommand) (ports.GenerationRun, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.instance != cmd.Execution.Instance {
		return nil, errors.New("wrong instance")
	}
	r := &fakeRun{ref: cmd.Execution, delay: b.TokenDelay, ignore: b.IgnoreCancellation, done: make(chan struct{})}
	b.run = r
	return r, nil
}
func (b *Backend) ObserveExecution(ctx context.Context, ref domain.ExecutionRef) (domain.ReleaseEvidence, error) {
	b.mu.Lock()
	r := b.run
	b.mu.Unlock()
	if r == nil || r.ref != ref {
		return domain.ReleaseEvidence{}, errors.New("unknown execution")
	}
	return r.AwaitRelease(ctx)
}
func (b *Backend) LoadCount() int { b.mu.Lock(); defer b.mu.Unlock(); return b.Loads }

type fakeRun struct {
	mu       sync.Mutex
	ref      domain.ExecutionRef
	delay    time.Duration
	ignore   bool
	index    int
	done     chan struct{}
	evidence domain.ReleaseEvidence
}

func (r *fakeRun) Reference() domain.ExecutionRef { return r.ref }
func (r *fakeRun) Next(ctx context.Context) (domain.GenerationEvent, error) {
	select {
	case <-ctx.Done():
		return domain.GenerationEvent{}, ctx.Err()
	case <-r.done:
		return domain.GenerationEvent{}, io.EOF
	case <-time.After(r.delay):
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.index++
	if r.index > 2 {
		r.finishLocked(domain.EvidenceExecutionCompleted)
		return domain.GenerationEvent{Execution: r.ref, Kind: domain.GenerationFinished, FinishReason: "stop"}, nil
	}
	return domain.GenerationEvent{Execution: r.ref, Kind: domain.GenerationDelta, Text: "synthetic "}, nil
}
func (r *fakeRun) Cancel(context.Context) error {
	if !r.ignore {
		r.finish(domain.EvidenceCancellationConfirmed)
	}
	return nil
}
func (r *fakeRun) finish(kind domain.EvidenceKind) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.finishLocked(kind)
}
func (r *fakeRun) finishLocked(kind domain.EvidenceKind) {
	if !r.evidence.ObservedAt.IsZero() {
		return
	}
	r.evidence = domain.ReleaseEvidence{Instance: r.ref.Instance, Execution: r.ref.ID, Kind: kind, ObservedAt: time.Now()}
	close(r.done)
}
func (r *fakeRun) AwaitRelease(ctx context.Context) (domain.ReleaseEvidence, error) {
	select {
	case <-ctx.Done():
		return domain.ReleaseEvidence{}, ctx.Err()
	case <-r.done:
		r.mu.Lock()
		defer r.mu.Unlock()
		return r.evidence, nil
	}
}
