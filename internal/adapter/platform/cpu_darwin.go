//go:build darwin && cgo

package platform

/*
#include <mach/mach.h>
#include <mach/host_info.h>
#include <mach/mach_time.h>
#include <libproc.h>
#include <pthread.h>
#include <sys/resource.h>
#include <stdint.h>

static pthread_once_t gateway_time_once = PTHREAD_ONCE_INIT;
static mach_timebase_info_data_t gateway_timebase;
static void gateway_init_timebase(void) { mach_timebase_info(&gateway_timebase); }

static int gateway_cpu(uint64_t *total, uint64_t *busy) {
    host_cpu_load_info_data_t info;
    mach_msg_type_number_t count = HOST_CPU_LOAD_INFO_COUNT;
    mach_port_t host = mach_host_self();
    kern_return_t result = host_statistics(host, HOST_CPU_LOAD_INFO, (host_info_t)&info, &count);
    mach_port_deallocate(mach_task_self(), host);
    if (result != KERN_SUCCESS) return -1;
    *busy = (uint64_t)info.cpu_ticks[CPU_STATE_USER] + info.cpu_ticks[CPU_STATE_SYSTEM] + info.cpu_ticks[CPU_STATE_NICE];
    *total = *busy + info.cpu_ticks[CPU_STATE_IDLE];
    return 0;
}
static int gateway_process(int pid, uint64_t *nanos, uint64_t *start) {
    struct rusage_info_v2 info;
    if (proc_pid_rusage(pid, RUSAGE_INFO_V2, (rusage_info_t *)&info) != 0) return -1;
    pthread_once(&gateway_time_once, gateway_init_timebase);
    if (gateway_timebase.denom == 0) return -1;
    // Rusage times are Mach ticks, including on Apple Silicon's non-1:1 clock.
    __uint128_t ticks = (__uint128_t)info.ri_user_time + info.ri_system_time;
    __uint128_t converted = ticks * gateway_timebase.numer / gateway_timebase.denom;
    if (converted > UINT64_MAX) return -1;
    *nanos = (uint64_t)converted;
    *start = info.ri_proc_start_abstime;
    return 0;
}
*/
import "C"

import (
	"os"
	"runtime"
	"time"

	"github.com/AtomsH4/adaptive-ai-gateway/internal/domain"
)

func (s *HostSampler) readCPU() domain.Observation[domain.CPUUsage] {
	var total, busy C.uint64_t
	now := cpuCounters{at: time.Now(), cores: runtime.NumCPU(), processes: map[int]processCounter{}}
	if C.gateway_cpu(&total, &busy) != 0 {
		s.previous = cpuCounters{}
		return domain.Observation[domain.CPUUsage]{Source: "darwin.mach.unavailable"}
	}
	now.total = uint64(total)
	now.busy = uint64(busy)
	pids := []int{os.Getpid()}
	if s.Owned != nil {
		pids = append(pids, s.Owned.OwnedPIDs()...)
	}
	for _, pid := range pids {
		var nanos, start C.uint64_t
		if C.gateway_process(C.int(pid), &nanos, &start) != 0 {
			now.processes = nil
			break
		}
		now.processes[pid] = processCounter{start: uint64(start), nanoseconds: uint64(nanos)}
	}
	return s.cpu(now)
}
