package platform

import (
	"context"
	"sync"
	"time"

	"github.com/AtomsH4/adaptive-ai-gateway/internal/application/ports"
	"github.com/AtomsH4/adaptive-ai-gateway/internal/domain"
)

// Cache is published by one periodic collector; reads never probe the OS.
type Cache struct {
	mu       sync.RWMutex
	snapshot domain.ResourceSnapshot
}

// HostSampler is called by one collector. CPU deltas share one sampling window.
type HostSampler struct {
	Owned    ports.OwnedProcesses
	previous cpuCounters
}

type processCounter struct{ start, nanoseconds uint64 }
type cpuCounters struct {
	at          time.Time
	total, busy uint64
	processes   map[int]processCounter
	cores       int
}

func (s *HostSampler) cpu(now cpuCounters) domain.Observation[domain.CPUUsage] {
	previous := s.previous
	s.previous = now
	obs := domain.Observation[domain.CPUUsage]{ObservedAt: now.at, Source: "darwin.mach_and_proc_rusage.window_delta"}
	if previous.at.IsZero() || now.at.Sub(previous.at) <= 0 || now.total <= previous.total || now.busy < previous.busy || now.cores < 1 {
		return obs
	}
	obs.Quality = domain.Available
	obs.Value.TotalPercent = 100 * float64(now.busy-previous.busy) / float64(now.total-previous.total)
	if len(now.processes) != len(previous.processes) || len(now.processes) == 0 {
		return obs
	}
	var nanos uint64
	for pid, current := range now.processes {
		old, ok := previous.processes[pid]
		if !ok || old.start != current.start || current.nanoseconds < old.nanoseconds {
			return obs
		}
		nanos += current.nanoseconds - old.nanoseconds
	}
	owned := 100 * float64(nanos) / float64(now.at.Sub(previous.at).Nanoseconds()) / float64(now.cores)
	if owned < 0 || owned > 100 || owned > obs.Value.TotalPercent+2 {
		return obs
	}
	obs.Value.OwnedPercent = owned
	obs.Value.OtherPercent = obs.Value.TotalPercent - owned
	if obs.Value.OtherPercent < 0 {
		obs.Value.OtherPercent = 0
	}
	obs.Value.AttributionReliable = true
	return obs
}

func (c *Cache) Read(ctx context.Context) (domain.ResourceSnapshot, error) {
	if err := ctx.Err(); err != nil {
		return domain.ResourceSnapshot{}, err
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.snapshot, nil
}
func (c *Cache) Collect(ctx context.Context, sampler ports.ResourceSampler, interval time.Duration) {
	timer := time.NewTicker(interval)
	defer timer.Stop()
	for {
		s, err := sampler.Sample(ctx)
		if err != nil {
			s.MemoryPressure.Quality = domain.Unavailable
		}
		c.mu.Lock()
		c.snapshot = s
		c.mu.Unlock()
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
	}
}
