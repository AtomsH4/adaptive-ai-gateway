//go:build darwin && !cgo

package platform

import "github.com/AtomsH4/adaptive-ai-gateway/internal/domain"

func (*HostSampler) readCPU() domain.Observation[domain.CPUUsage] {
	return domain.Observation[domain.CPUUsage]{Source: "darwin.cpu.requires_cgo"}
}
