package platform

import (
	"github.com/AtomsH4/adaptive-ai-gateway/internal/domain"
	"testing"
	"time"
)

func TestCPUAttributionUsesElapsedTimeAllCoresAndStableProcessIdentity(t *testing.T) {
	s := &HostSampler{}
	at := time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC)
	s.cpu(cpuCounters{at: at, total: 100, busy: 50, cores: 6, processes: map[int]processCounter{10: {start: 1, nanoseconds: 100}}})
	current := cpuCounters{at: at.Add(2 * time.Second), total: 1300, busy: 950, cores: 6, processes: map[int]processCounter{10: {start: 1, nanoseconds: 6000000100}}}
	got := s.cpu(current)
	if got.Quality != domain.Available || !got.Value.AttributionReliable || got.Value.TotalPercent != 75 || got.Value.OwnedPercent != 50 || got.Value.OtherPercent != 25 {
		t.Fatalf("incorrect attribution: %+v", got)
	}
	current.at = current.at.Add(2 * time.Second)
	current.total += 1200
	current.busy += 500
	current.processes = map[int]processCounter{10: {start: 2, nanoseconds: 200}}
	if got = s.cpu(current); got.Value.AttributionReliable {
		t.Fatal("reused PID trusted as continuous owned process")
	}
}
