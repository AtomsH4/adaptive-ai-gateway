package ports

import (
	"context"

	"adaptive-ai-gateway/internal/domain"
)

// ModelLifecycle is used by the coordinator, never directly by an HTTP handler.
// The caller creates the instance identity before the first side effect.
// An ambiguous error or context timeout never authorizes clearing its reservation.
type ModelLifecycle interface {
	Load(context.Context, domain.LoadCommand) (domain.InstanceObservation, error)
	InspectInstance(context.Context, domain.InstanceRef) (domain.InstanceObservation, error)
	Stop(context.Context, domain.InstanceRef) (domain.ReleaseEvidence, error)
}

type Generator interface {
	Begin(context.Context, domain.ExecutionCommand) (GenerationRun, error)
	ObserveExecution(context.Context, domain.ExecutionRef) (domain.ReleaseEvidence, error)
}

// Begin's context bounds dispatch. The run has an independently managed lifetime.
// Next exposes output completion; AwaitRelease exposes resource completion.
// Cancel acknowledges a cancellation request only. It does not release resources.
type GenerationRun interface {
	Reference() domain.ExecutionRef
	Next(context.Context) (domain.GenerationEvent, error)
	Cancel(context.Context) error
	AwaitRelease(context.Context) (domain.ReleaseEvidence, error)
}

// A concrete adapter may satisfy both ports. Consumers take only what they need.
type Backend interface {
	ModelLifecycle
	Generator
	Description() domain.BackendDescription
}
