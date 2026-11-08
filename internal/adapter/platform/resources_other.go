//go:build !darwin

package platform

import (
	"context"
	"github.com/AtomsH4/adaptive-ai-gateway/internal/domain"
)

func (*HostSampler) Sample(ctx context.Context) (domain.ResourceSnapshot, error) {
	return domain.ResourceSnapshot{MemoryPressure: domain.Observation[domain.MemoryPressure]{Source: "unsupported.platform"}}, ctx.Err()
}
