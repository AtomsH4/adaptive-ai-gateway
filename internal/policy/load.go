package policy

import (
	"github.com/AtomsH4/adaptive-ai-gateway/internal/domain"
	"time"
)

// LoadPolicy adds optional, attributable external CPU contention to memory guard.
type LoadPolicy struct {
	Memory        ResourceGuard
	OtherCPULimit float64
}

func (p LoadPolicy) Evaluate(snapshot domain.ResourceSnapshot, now time.Time) domain.ResourceSignal {
	signal := p.Memory.Evaluate(snapshot, now)
	if signal.Kind != domain.SignalNormal {
		return signal
	}
	cpu := snapshot.CPU
	if p.OtherCPULimit > 0 && cpu.Quality == domain.Available && cpu.Source != "" && !cpu.ObservedAt.After(now) && now.Sub(cpu.ObservedAt) <= 6*time.Second && cpu.Value.AttributionReliable && cpu.Value.OtherPercent >= p.OtherCPULimit && cpu.Value.OtherPercent <= 100 {
		return domain.ResourceSignal{Kind: domain.SignalWarning, Reason: domain.ReasonCode("cpu_busy")}
	}
	return signal
}
