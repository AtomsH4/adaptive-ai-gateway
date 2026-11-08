package api

import (
	"context"

	"github.com/AtomsH4/adaptive-ai-gateway/internal/domain"
)

type GenerateRequest struct {
	ID              string
	Model           string
	Messages        []domain.Message
	MaxOutputTokens int
	Temperature     float64
}

// Output belongs to one caller. Close cancels that caller, never an engine directly.
// Next yields delta/finished events; failures are returned as errors, never success.
type Output interface {
	Next(context.Context) (domain.GenerationEvent, error)
	Close() error
}

type SchedulerStatus struct {
	ServiceReady     bool
	EngineState      string
	ProtectionState  string
	CanExecuteNow    bool
	CanQueue         bool
	ActiveExecutions int
	WaitingRequests  int
	ReasonCodes      []string
	RecentDecisions  []domain.Decision
}

type Gateway interface {
	Generate(context.Context, GenerateRequest) (Output, error)
	Status(context.Context) (SchedulerStatus, error)
}
