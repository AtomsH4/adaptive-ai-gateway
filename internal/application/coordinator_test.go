package application_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/AtomsH4/adaptive-ai-gateway/internal/adapter/fake"
	"github.com/AtomsH4/adaptive-ai-gateway/internal/application"
	"github.com/AtomsH4/adaptive-ai-gateway/internal/application/api"
	"github.com/AtomsH4/adaptive-ai-gateway/internal/domain"
	"github.com/AtomsH4/adaptive-ai-gateway/internal/policy"
)

type manualClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *manualClock) Now() time.Time          { c.mu.Lock(); defer c.mu.Unlock(); return c.now }
func (c *manualClock) advance(d time.Duration) { c.mu.Lock(); c.now = c.now.Add(d); c.mu.Unlock() }

type harness struct {
	c      *application.Coordinator
	b      *fake.Backend
	reader *fake.StaticSampler
	clock  *manualClock
	seq    int
}

func newHarness(t *testing.T, configure func(*application.Options, *fake.Backend)) *harness {
	t.Helper()
	clock := &manualClock{now: time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC)}
	reader := fake.NewStaticSampler(domain.ResourceSnapshot{})
	b := fake.NewBackend()
	opts := application.Options{Model: domain.ModelRef{ID: "model"}, Profile: domain.RuntimeProfileRef{ID: "test", Revision: "1"}, OwnerID: "test-owner", ContextTokens: 2048, RequiredFreeBytes: 1 << 20, QueueLimit: 4, QueueTimeout: 30 * time.Second, LoadTimeout: 120 * time.Second, ExecutionTimeout: 120 * time.Second, RequestTimeout: 270 * time.Second, IdleTimeout: 300 * time.Second, CriticalGrace: 5 * time.Second, CancelGrace: 3 * time.Second, RecoveryWindow: 30 * time.Second, ShutdownTimeout: 4 * time.Second}
	if configure != nil {
		configure(&opts, b)
	}
	guard, _ := policy.NewResourceGuard(6 * time.Second)
	c, err := application.NewCoordinator(opts, b, reader, guard, clock)
	if err != nil {
		t.Fatal(err)
	}
	h := &harness{c: c, b: b, reader: reader, clock: clock}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		closed := make(chan error, 1)
		go func() { closed <- c.Close(ctx) }()
		for i := 0; i < 100; i++ {
			s, err := c.Status(ctx)
			if err != nil || !s.ServiceReady {
				break
			}
			time.Sleep(time.Millisecond)
		}
		h.clock.advance(5 * time.Minute)
		if err := <-closed; err != nil {
			t.Error(err)
		}
	})
	h.sample(domain.PressureNormal)
	h.wait(t, func(s api.SchedulerStatus) bool { return s.ProtectionState == "recovering" })
	h.clock.advance(time.Second)
	h.sample(domain.PressureNormal)
	h.wait(t, func(s api.SchedulerStatus) bool { return s.ProtectionState == "normal" })
	return h
}
func (h *harness) sample(pressure domain.MemoryPressure) {
	now := h.clock.Now()
	h.reader.Set(domain.ResourceSnapshot{MemoryPressure: domain.Observation[domain.MemoryPressure]{Value: pressure, Quality: domain.Available, ObservedAt: now, Source: "test"}, AvailableMemoryBytes: domain.Observation[uint64]{Value: 8 << 30, Quality: domain.Available, ObservedAt: now, Source: "test"}}, nil)
}
func (h *harness) wait(t *testing.T, predicate func(api.SchedulerStatus) bool) api.SchedulerStatus {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		s, err := h.c.Status(ctx)
		cancel()
		if err == nil && predicate(s) {
			return s
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("coordinator condition did not become true")
	return api.SchedulerStatus{}
}
func (h *harness) request(t *testing.T, ctx context.Context, id string) api.Output {
	t.Helper()
	out, err := h.c.Generate(ctx, api.GenerateRequest{ID: id, Model: "model", Messages: []domain.Message{{Role: "user", Content: "test"}}, MaxOutputTokens: 32})
	if err != nil {
		t.Fatal(err)
	}
	return out
}
func next(t *testing.T, out api.Output) (domain.GenerationEvent, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	return out.Next(ctx)
}
func assertFailure(t *testing.T, out api.Output, code string) {
	t.Helper()
	_, err := next(t, out)
	var f *domain.Failure
	if !errors.As(err, &f) || f.Code != code {
		t.Fatalf("got %v, want %s", err, code)
	}
}

func TestSharedColdStartDoesNotSpendOrdinaryQueueBudget(t *testing.T) {
	h := newHarness(t, func(_ *application.Options, b *fake.Backend) { b.LoadDelay = 200 * time.Millisecond })
	ctx, cancel := context.WithCancel(context.Background())
	first := h.request(t, ctx, "first")
	defer first.Close()
	h.wait(t, func(s api.SchedulerStatus) bool { return s.EngineState == "loading" })
	second := h.request(t, context.Background(), "second")
	defer second.Close()
	cancel()
	h.clock.advance(45 * time.Second)
	h.sample(domain.PressureNormal)
	h.wait(t, func(s api.SchedulerStatus) bool { return s.WaitingRequests == 1 && s.EngineState == "loading" })
	e, err := next(t, second)
	if err != nil || e.Kind != domain.GenerationDelta {
		t.Fatalf("cold start failed: %+v %v", e, err)
	}
	if h.b.LoadCount() != 1 {
		t.Fatalf("loads=%d", h.b.LoadCount())
	}
}
func TestUnknownObservationsPreserveActiveButBlockLoadedModel(t *testing.T) {
	h := newHarness(t, func(_ *application.Options, b *fake.Backend) { b.TokenDelay = time.Hour; b.IgnoreCancellation = true })
	out := h.request(t, context.Background(), "active")
	defer out.Close()
	h.wait(t, func(s api.SchedulerStatus) bool { return s.ActiveExecutions == 1 })
	h.reader.Set(domain.ResourceSnapshot{}, nil)
	h.wait(t, func(s api.SchedulerStatus) bool {
		return s.ProtectionState == "observation_guard" && s.ActiveExecutions == 1 && s.ServiceReady
	})
	waiting := h.request(t, context.Background(), "waiting")
	defer waiting.Close()
	h.clock.advance(31 * time.Second)
	assertFailure(t, waiting, "telemetry_unavailable")
	h.wait(t, func(s api.SchedulerStatus) bool { return s.ActiveExecutions == 1 })
}
func TestCancellationDoesNotReleaseUntilOwnedExitEvidence(t *testing.T) {
	h := newHarness(t, func(_ *application.Options, b *fake.Backend) { b.TokenDelay = time.Hour; b.IgnoreCancellation = true })
	out := h.request(t, context.Background(), "active")
	h.wait(t, func(s api.SchedulerStatus) bool { return s.ActiveExecutions == 1 })
	out.Close()
	h.wait(t, func(s api.SchedulerStatus) bool { return s.ActiveExecutions == 1 })
	h.clock.advance(4 * time.Second)
	h.sample(domain.PressureNormal)
	h.wait(t, func(s api.SchedulerStatus) bool { return s.ActiveExecutions == 0 && s.EngineState == "unloaded" })
}
func TestCriticalEscalatesDespiteMissingObservationThenRequiresRecovery(t *testing.T) {
	h := newHarness(t, func(_ *application.Options, b *fake.Backend) { b.TokenDelay = time.Hour; b.IgnoreCancellation = true })
	out := h.request(t, context.Background(), "active")
	defer out.Close()
	h.wait(t, func(s api.SchedulerStatus) bool { return s.ActiveExecutions == 1 })
	h.sample(domain.PressureCritical)
	h.wait(t, func(s api.SchedulerStatus) bool { return s.ProtectionState == "reclaiming" && s.ActiveExecutions == 1 })
	h.reader.Set(domain.ResourceSnapshot{}, nil)
	h.clock.advance(6 * time.Second)
	assertFailure(t, out, "memory_pressure_interrupted")
	h.wait(t, func(s api.SchedulerStatus) bool { return s.ActiveExecutions == 0 && s.EngineState == "unloaded" })
	h.sample(domain.PressureNormal)
	h.wait(t, func(s api.SchedulerStatus) bool { return s.ProtectionState == "recovering" })
	h.clock.advance(31 * time.Second)
	h.sample(domain.PressureNormal)
	h.wait(t, func(s api.SchedulerStatus) bool { return s.ProtectionState == "normal" })
}
func TestWaitingCapacityIncludesAllColdStartWaiters(t *testing.T) {
	h := newHarness(t, func(_ *application.Options, b *fake.Backend) { b.LoadDelay = time.Hour })
	for _, id := range []string{"1", "2", "3", "4"} {
		out := h.request(t, context.Background(), id)
		defer out.Close()
	}
	_, err := h.c.Generate(context.Background(), api.GenerateRequest{ID: "5", Model: "model", Messages: []domain.Message{{Role: "user", Content: "test"}}, MaxOutputTokens: 1})
	var f *domain.Failure
	if !errors.As(err, &f) || f.Code != "queue_full" {
		t.Fatalf("got %v", err)
	}
}

func TestLateWaiterDoesNotResetSharedLoadDeadline(t *testing.T) {
	h := newHarness(t, func(_ *application.Options, b *fake.Backend) { b.LoadDelay = time.Hour })
	first := h.request(t, context.Background(), "first")
	defer first.Close()
	h.wait(t, func(s api.SchedulerStatus) bool { return s.EngineState == "loading" })
	h.clock.advance(90 * time.Second)
	h.sample(domain.PressureNormal)
	second := h.request(t, context.Background(), "second")
	defer second.Close()
	h.wait(t, func(s api.SchedulerStatus) bool { return s.WaitingRequests == 2 })
	h.clock.advance(31 * time.Second)
	h.sample(domain.PressureNormal)
	assertFailure(t, first, "model_load_timeout")
	assertFailure(t, second, "model_load_timeout")
	if h.b.LoadCount() != 1 {
		t.Fatalf("shared load restarted: %d", h.b.LoadCount())
	}
}

func TestLoadedModelObservationWaitUsesOrdinaryBudget(t *testing.T) {
	h := newHarness(t, func(_ *application.Options, b *fake.Backend) { b.LoadDelay = 150 * time.Millisecond })
	out := h.request(t, context.Background(), "guarded")
	defer out.Close()
	h.wait(t, func(s api.SchedulerStatus) bool { return s.EngineState == "loading" })
	h.reader.Set(domain.ResourceSnapshot{}, nil)
	h.wait(t, func(s api.SchedulerStatus) bool { return s.EngineState == "ready" && s.ActiveExecutions == 0 })
	h.clock.advance(31 * time.Second)
	assertFailure(t, out, "telemetry_unavailable")
}

func TestRepeatedDiagnosticQueriesDoNotCountAsRecoverySamples(t *testing.T) {
	h := newHarness(t, nil)
	h.reader.Set(domain.ResourceSnapshot{}, nil)
	h.wait(t, func(s api.SchedulerStatus) bool { return s.ProtectionState == "observation_guard" })
	h.sample(domain.PressureNormal)
	h.wait(t, func(s api.SchedulerStatus) bool { return s.ProtectionState == "recovering" })
	for i := 0; i < 10; i++ {
		s, err := h.c.Status(context.Background())
		if err != nil || s.ProtectionState != "recovering" {
			t.Fatalf("query reopened admission: %+v %v", s, err)
		}
	}
	h.clock.advance(time.Second)
	h.sample(domain.PressureNormal)
	h.wait(t, func(s api.SchedulerStatus) bool { return s.ProtectionState == "normal" })
}

func TestParallelRequestsCannotExceedWaitingCapacityOrDuplicateLoad(t *testing.T) {
	h := newHarness(t, func(_ *application.Options, b *fake.Backend) { b.LoadDelay = time.Hour })
	var wg sync.WaitGroup
	accepted := make(chan api.Output, 12)
	failures := make(chan error, 12)
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			out, err := h.c.Generate(context.Background(), api.GenerateRequest{ID: "caller", Model: "model", Messages: []domain.Message{{Role: "user", Content: "test"}}, MaxOutputTokens: 1})
			if err != nil {
				failures <- err
			} else {
				accepted <- out
			}
		}()
	}
	wg.Wait()
	close(accepted)
	close(failures)
	if len(accepted) != 4 || len(failures) != 8 {
		t.Fatalf("accepted=%d failures=%d", len(accepted), len(failures))
	}
	h.wait(t, func(s api.SchedulerStatus) bool { return s.EngineState == "loading" })
	if h.b.LoadCount() != 1 {
		t.Fatalf("loads=%d", h.b.LoadCount())
	}
	for out := range accepted {
		out.Close()
	}
}
