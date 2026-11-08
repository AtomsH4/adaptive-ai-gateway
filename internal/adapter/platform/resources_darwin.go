//go:build darwin

package platform

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/AtomsH4/adaptive-ai-gateway/internal/domain"
)

func (sampler *HostSampler) Sample(ctx context.Context) (domain.ResourceSnapshot, error) {
	ctx, cancel := context.WithTimeout(ctx, 1500*time.Millisecond)
	defer cancel()
	s := domain.ResourceSnapshot{}
	pressure, err := exec.CommandContext(ctx, "/usr/sbin/sysctl", "-n", "kern.memorystatus_vm_pressure_level").Output()
	s.MemoryPressure = domain.Observation[domain.MemoryPressure]{Source: "darwin.sysctl.dispatch_pressure", ObservedAt: time.Now()}
	if err == nil {
		value, _ := strconv.Atoi(strings.TrimSpace(string(pressure)))
		switch value {
		case 1:
			s.MemoryPressure.Value = domain.PressureNormal
		case 2:
			s.MemoryPressure.Value = domain.PressureWarning
		case 4:
			s.MemoryPressure.Value = domain.PressureCritical
		}
		if s.MemoryPressure.Value != domain.PressureUnknown {
			s.MemoryPressure.Quality = domain.Available
		}
	}
	vm, vmErr := exec.CommandContext(ctx, "/usr/bin/vm_stat").Output()
	s.AvailableMemoryBytes = domain.Observation[uint64]{Source: "darwin.vm_stat.free_plus_speculative", ObservedAt: time.Now()}
	if vmErr == nil {
		if n, e := parseFreeMemory(string(vm)); e == nil {
			s.AvailableMemoryBytes.Value = n
			s.AvailableMemoryBytes.Quality = domain.Available
		}
	}
	s.CPU = sampler.readCPU()
	// GPU utilization remains optional and unavailable, never inferred as zero.
	return s, nil
}

func parseFreeMemory(text string) (uint64, error) {
	lines := strings.Split(text, "\n")
	if len(lines) == 0 {
		return 0, fmt.Errorf("missing vm_stat header")
	}
	var pageSize uint64
	if _, err := fmt.Sscanf(lines[0], "Mach Virtual Memory Statistics: (page size of %d bytes)", &pageSize); err != nil || pageSize == 0 {
		return 0, fmt.Errorf("invalid vm_stat page size")
	}
	var pages uint64
	found := false
	for _, line := range lines[1:] {
		key, value, ok := strings.Cut(line, ":")
		if !ok || (key != "Pages free" && key != "Pages speculative") {
			continue
		}
		n, err := strconv.ParseUint(strings.TrimSuffix(strings.TrimSpace(value), "."), 10, 64)
		if err != nil {
			return 0, err
		}
		pages += n
		if key == "Pages free" {
			found = true
		}
	}
	if !found || pages > (^uint64(0))/pageSize {
		return 0, fmt.Errorf("invalid vm_stat counters")
	}
	return pages * pageSize, nil
}
