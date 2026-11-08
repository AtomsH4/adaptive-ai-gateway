package policy_test

import (
	"testing"
	"time"

	"adaptive-ai-gateway/internal/domain"
	"adaptive-ai-gateway/internal/policy"
)

func TestRequiredMemoryEvidenceAndFreshness(t *testing.T) {
	now := time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)
	guard, ok := policy.NewResourceGuard(6 * time.Second)
	if !ok {
		t.Fatal("valid freshness limit rejected")
	}
	valid := domain.Observation[domain.MemoryPressure]{Value: domain.PressureNormal, Quality: domain.Available, Source: "test", ObservedAt: now}
	cases := []struct {
		name   string
		change func(*domain.Observation[domain.MemoryPressure])
		kind   domain.SignalKind
		reason domain.ReasonCode
	}{
		{"fresh normal", func(o *domain.Observation[domain.MemoryPressure]) {}, domain.SignalNormal, domain.ReasonNone},
		{"missing", func(o *domain.Observation[domain.MemoryPressure]) { o.Quality = domain.Unavailable }, domain.SignalObservationGuard, domain.ReasonTelemetryUnavailable},
		{"zero timestamp", func(o *domain.Observation[domain.MemoryPressure]) { o.ObservedAt = time.Time{} }, domain.SignalObservationGuard, domain.ReasonTelemetryUnavailable},
		{"future timestamp", func(o *domain.Observation[domain.MemoryPressure]) { o.ObservedAt = now.Add(time.Second) }, domain.SignalObservationGuard, domain.ReasonTelemetryUnavailable},
		{"missing provenance", func(o *domain.Observation[domain.MemoryPressure]) { o.Source = "" }, domain.SignalObservationGuard, domain.ReasonTelemetryUnavailable},
		{"unknown value", func(o *domain.Observation[domain.MemoryPressure]) { o.Value = domain.PressureUnknown }, domain.SignalObservationGuard, domain.ReasonTelemetryUnavailable},
		{"unmapped platform value", func(o *domain.Observation[domain.MemoryPressure]) { o.Value = domain.MemoryPressure(99) }, domain.SignalObservationGuard, domain.ReasonTelemetryUnavailable},
		{"at freshness boundary", func(o *domain.Observation[domain.MemoryPressure]) { o.ObservedAt = now.Add(-6 * time.Second) }, domain.SignalNormal, domain.ReasonNone},
		{"beyond boundary", func(o *domain.Observation[domain.MemoryPressure]) {
			o.ObservedAt = now.Add(-6*time.Second - time.Nanosecond)
		}, domain.SignalObservationGuard, domain.ReasonTelemetryStale},
		{"warning", func(o *domain.Observation[domain.MemoryPressure]) { o.Value = domain.PressureWarning }, domain.SignalWarning, domain.ReasonMemoryPressure},
		{"critical", func(o *domain.Observation[domain.MemoryPressure]) { o.Value = domain.PressureCritical }, domain.SignalCritical, domain.ReasonMemoryPressure},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			obs := valid
			tc.change(&obs)
			snapshot := domain.ResourceSnapshot{MemoryPressure: obs}
			got := guard.Evaluate(snapshot, now)
			if got.Kind != tc.kind || got.Reason != tc.reason {
				t.Fatalf("got %+v, want %s/%s", got, tc.kind, tc.reason)
			}
		})
	}
}

func TestOwnHighCPUAndMissingOptionalGPUDoNotBecomeMemoryPressure(t *testing.T) {
	now := time.Now()
	guard, _ := policy.NewResourceGuard(6 * time.Second)
	snapshot := domain.ResourceSnapshot{
		MemoryPressure: domain.Observation[domain.MemoryPressure]{Value: domain.PressureNormal, Quality: domain.Available, Source: "test", ObservedAt: now},
		CPU:            domain.Observation[domain.CPUUsage]{Value: domain.CPUUsage{TotalPercent: 99, OwnedPercent: 98, OtherPercent: 1, AttributionReliable: true}, Quality: domain.Available, ObservedAt: now, Source: "test"},
	}
	if got := guard.Evaluate(snapshot, now); got.Kind != domain.SignalNormal {
		t.Fatalf("optional/high-own-load evidence changed memory signal: %+v", got)
	}
}
