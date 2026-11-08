package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/AtomsH4/adaptive-ai-gateway/internal/adapter/fake"
	"github.com/AtomsH4/adaptive-ai-gateway/internal/application"
	"github.com/AtomsH4/adaptive-ai-gateway/internal/application/api"
	"github.com/AtomsH4/adaptive-ai-gateway/internal/application/ports"
	"github.com/AtomsH4/adaptive-ai-gateway/internal/domain"
	"github.com/AtomsH4/adaptive-ai-gateway/internal/policy"
)

type independentReader struct{ snapshot domain.ResourceSnapshot }

func (r independentReader) Read(context.Context) (domain.ResourceSnapshot, error) {
	return r.snapshot, nil
}

func TestReaderReplacementRequiresNoApplicationChanges(t *testing.T) {
	now := time.Now()
	guard, _ := policy.NewResourceGuard(6 * time.Second)
	snapshot := domain.ResourceSnapshot{MemoryPressure: domain.Observation[domain.MemoryPressure]{Value: domain.PressureNormal, Quality: domain.Available, Source: "fake", ObservedAt: now}}
	readers := []ports.ResourceReader{fake.NewStaticSampler(snapshot), independentReader{snapshot: snapshot}}
	for _, reader := range readers {
		service, err := application.NewAssessmentService(reader, guard, fake.FixedClock{At: now})
		if err != nil {
			t.Fatal(err)
		}
		var query api.ResourceAssessmentQuery = service
		got, err := query.Assess(context.Background())
		if err != nil || got.Signal.Kind != domain.SignalNormal {
			t.Fatalf("replacement failed: %+v %v", got, err)
		}
	}
}

func TestReaderFailureCannotReusePreviouslyNormalEvidence(t *testing.T) {
	now := time.Now()
	guard, _ := policy.NewResourceGuard(6 * time.Second)
	snapshot := domain.ResourceSnapshot{MemoryPressure: domain.Observation[domain.MemoryPressure]{Value: domain.PressureNormal, Quality: domain.Available, Source: "fake", ObservedAt: now}}
	reader := fake.NewStaticSampler(snapshot)
	reader.Set(snapshot, errors.New("driver failed while returning cached data"))
	service, _ := application.NewAssessmentService(reader, guard, fake.FixedClock{At: now})
	got, err := service.Assess(context.Background())
	if err != nil || !got.ReadFailed || got.Signal.Kind != domain.SignalObservationGuard {
		t.Fatalf("unsafe failure projection: %+v %v", got, err)
	}
}

func TestCancelledCallerIsNotConvertedIntoSuccessfulAssessment(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	guard, _ := policy.NewResourceGuard(6 * time.Second)
	service, _ := application.NewAssessmentService(fake.NewStaticSampler(domain.ResourceSnapshot{}), guard, fake.FixedClock{At: time.Now()})
	if _, err := service.Assess(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v", err)
	}
}
