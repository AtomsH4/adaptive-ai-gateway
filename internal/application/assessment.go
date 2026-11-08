// Package application coordinates use cases through consumer-owned interfaces.
package application

import (
	"context"
	"errors"

	"adaptive-ai-gateway/internal/application/ports"
	"adaptive-ai-gateway/internal/domain"
)

// AssessmentService is a read-only architecture reference use case.
// It does not own execution slots and must not be used as an admission endpoint.
type AssessmentService struct {
	reader    ports.ResourceReader
	evaluator ports.SignalEvaluator
	clock     ports.Clock
}

func NewAssessmentService(reader ports.ResourceReader, evaluator ports.SignalEvaluator, clock ports.Clock) (*AssessmentService, error) {
	if reader == nil || evaluator == nil || clock == nil {
		return nil, errors.New("assessment requires reader, evaluator and clock")
	}
	return &AssessmentService{reader: reader, evaluator: evaluator, clock: clock}, nil
}

func (s *AssessmentService) Assess(ctx context.Context) (domain.ResourceAssessment, error) {
	if err := ctx.Err(); err != nil {
		return domain.ResourceAssessment{}, err
	}
	snapshot, readErr := s.reader.Read(ctx)
	if err := ctx.Err(); err != nil {
		return domain.ResourceAssessment{}, err
	}
	if readErr != nil {
		// A driver returning old normal data alongside an error cannot reopen work.
		snapshot.MemoryPressure = domain.Observation[domain.MemoryPressure]{Source: "snapshot_reader_failure"}
	}
	now := s.clock.Now()
	return domain.ResourceAssessment{
		AssessedAt: now, Snapshot: snapshot,
		Signal: s.evaluator.Evaluate(snapshot, now), ReadFailed: readErr != nil,
	}, nil
}
