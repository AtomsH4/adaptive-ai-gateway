// Package ports declares the application consumers' outbound contracts.
package ports

import (
	"context"
	"time"

	"github.com/AtomsH4/adaptive-ai-gateway/internal/domain"
)

// Sample returns per-field unavailable observations for partial failure.
// A non-nil error invalidates required memory-pressure evidence for the sample.
type ResourceSampler interface {
	Sample(context.Context) (domain.ResourceSnapshot, error)
}

// ResourceReader reads the latest published snapshot, without starting an OS probe.
// Diagnostic queries use this port; periodic collection uses ResourceSampler.
type ResourceReader interface {
	Read(context.Context) (domain.ResourceSnapshot, error)
}

type Clock interface {
	Now() time.Time
}

// SignalEvaluator is pure: it cannot issue execution grants or engine commands.
type SignalEvaluator interface {
	Evaluate(domain.ResourceSnapshot, time.Time) domain.ResourceSignal
}
