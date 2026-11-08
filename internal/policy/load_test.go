package policy_test

import (
	"github.com/AtomsH4/adaptive-ai-gateway/internal/domain"
	"github.com/AtomsH4/adaptive-ai-gateway/internal/policy"
	"testing"
	"time"
)

func TestCPUContentionRequiresFreshReliableExternalLoad(t *testing.T) {
	now := time.Now()
	guard, _ := policy.NewResourceGuard(6 * time.Second)
	p := policy.LoadPolicy{Memory: guard, OtherCPULimit: 85}
	s := domain.ResourceSnapshot{MemoryPressure: domain.Observation[domain.MemoryPressure]{Value: domain.PressureNormal, Quality: domain.Available, Source: "test", ObservedAt: now}, CPU: domain.Observation[domain.CPUUsage]{Value: domain.CPUUsage{TotalPercent: 99, OwnedPercent: 98, OtherPercent: 1, AttributionReliable: true}, Quality: domain.Available, Source: "test", ObservedAt: now}}
	if got := p.Evaluate(s, now); got.Kind != domain.SignalNormal {
		t.Fatalf("own load throttled: %+v", got)
	}
	s.CPU.Value.OwnedPercent = 1
	s.CPU.Value.OtherPercent = 98
	if got := p.Evaluate(s, now); string(got.Reason) != "cpu_busy" {
		t.Fatalf("external contention ignored: %+v", got)
	}
	s.CPU.Value.AttributionReliable = false
	if got := p.Evaluate(s, now); got.Kind != domain.SignalNormal {
		t.Fatalf("uncertain attribution treated as proof: %+v", got)
	}
}
