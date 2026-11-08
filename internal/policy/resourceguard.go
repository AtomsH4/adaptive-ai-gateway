// Package policy contains pure resource rules, independently replaceable via ports.
package policy

import (
	"time"

	"github.com/AtomsH4/adaptive-ai-gateway/internal/domain"
)

// ResourceGuard classifies memory observations. Timed hysteresis, admission,
// reservations and emergency reclamation belong to the future coordinator.
type ResourceGuard struct {
	maxAge time.Duration
}

func NewResourceGuard(maxAge time.Duration) (ResourceGuard, bool) {
	if maxAge <= 0 {
		return ResourceGuard{}, false
	}
	return ResourceGuard{maxAge: maxAge}, true
}

func (g ResourceGuard) Evaluate(snapshot domain.ResourceSnapshot, now time.Time) domain.ResourceSignal {
	obs := snapshot.MemoryPressure
	unknown := domain.ResourceSignal{Kind: domain.SignalObservationGuard, Reason: domain.ReasonTelemetryUnavailable}
	if g.maxAge <= 0 || obs.Quality != domain.Available || obs.Source == "" ||
		now.IsZero() || obs.ObservedAt.IsZero() || obs.ObservedAt.After(now) {
		return unknown
	}
	if now.Sub(obs.ObservedAt) > g.maxAge {
		return domain.ResourceSignal{Kind: domain.SignalObservationGuard, Reason: domain.ReasonTelemetryStale}
	}
	switch obs.Value {
	case domain.PressureNormal:
		return domain.ResourceSignal{Kind: domain.SignalNormal}
	case domain.PressureWarning:
		return domain.ResourceSignal{Kind: domain.SignalWarning, Reason: domain.ReasonMemoryPressure}
	case domain.PressureCritical:
		return domain.ResourceSignal{Kind: domain.SignalCritical, Reason: domain.ReasonMemoryPressure}
	default:
		return unknown
	}
}
