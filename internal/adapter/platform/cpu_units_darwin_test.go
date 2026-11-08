//go:build darwin && cgo

package platform

import (
	"os"
	"syscall"
	"testing"
	"time"
)

func rusageNanos(t *testing.T) int64 {
	t.Helper()
	var usage syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &usage); err != nil {
		t.Fatal(err)
	}
	return (usage.Utime.Sec+usage.Stime.Sec)*int64(time.Second) + (int64(usage.Utime.Usec)+int64(usage.Stime.Usec))*int64(time.Microsecond)
}

func TestMachProcessTimeAgreesWithSystemRusageNanoseconds(t *testing.T) {
	sampler := &HostSampler{}
	sampler.readCPU()
	before := sampler.previous.processes[os.Getpid()].nanoseconds
	referenceBefore := rusageNanos(t)
	until := time.Now().Add(150 * time.Millisecond)
	for time.Now().Before(until) {
	}
	reference := rusageNanos(t) - referenceBefore
	sampler.readCPU()
	measured := sampler.previous.processes[os.Getpid()].nanoseconds - before
	if reference < 5*int64(time.Millisecond) {
		t.Skip("not enough CPU time for a stable comparison")
	}
	ratio := float64(measured) / float64(reference)
	if ratio < 0.75 || ratio > 1.3 {
		t.Fatalf("process time is in the wrong units: measured=%d reference=%d ratio=%f", measured, reference, ratio)
	}
}
