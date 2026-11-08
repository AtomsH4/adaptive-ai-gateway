// Package fake provides bounded, in-memory adapters for architecture checks.
// It does not run models or represent real hardware measurements.
package fake

import (
	"context"
	"sync"
	"time"

	"github.com/AtomsH4/adaptive-ai-gateway/internal/domain"
)

type StaticSampler struct {
	mu       sync.RWMutex
	snapshot domain.ResourceSnapshot
	err      error
}

func NewStaticSampler(snapshot domain.ResourceSnapshot) *StaticSampler {
	return &StaticSampler{snapshot: snapshot}
}

func (s *StaticSampler) Set(snapshot domain.ResourceSnapshot, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshot, s.err = snapshot, err
}

func (s *StaticSampler) Sample(ctx context.Context) (domain.ResourceSnapshot, error) {
	if err := ctx.Err(); err != nil {
		return domain.ResourceSnapshot{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.snapshot, s.err
}

func (s *StaticSampler) Read(ctx context.Context) (domain.ResourceSnapshot, error) {
	return s.Sample(ctx)
}

type FixedClock struct{ At time.Time }

func (c FixedClock) Now() time.Time { return c.At }

type SyntheticSampler struct{}

func (SyntheticSampler) Sample(ctx context.Context) (domain.ResourceSnapshot, error) {
	now := time.Now()
	return domain.ResourceSnapshot{MemoryPressure: domain.Observation[domain.MemoryPressure]{Value: domain.PressureNormal, Quality: domain.Available, ObservedAt: now, Source: "synthetic.development_only"}, AvailableMemoryBytes: domain.Observation[uint64]{Value: 16 << 30, Quality: domain.Available, ObservedAt: now, Source: "synthetic.development_only"}}, ctx.Err()
}
