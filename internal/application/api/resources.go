// Package api declares application use cases available to inbound adapters.
// It exposes no engine, sampling, or resource-reservation authority.
package api

import (
	"context"

	"github.com/AtomsH4/adaptive-ai-gateway/internal/domain"
)

// ResourceAssessmentQuery reads a cached snapshot and classifies its signals.
// It does not grant execution capacity or trigger physical resource sampling.
type ResourceAssessmentQuery interface {
	Assess(context.Context) (domain.ResourceAssessment, error)
}
