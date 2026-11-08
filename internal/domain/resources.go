// Package domain contains transport- and platform-independent values.
package domain

import "time"

type Quality uint8

const (
	Unavailable Quality = iota
	Available
)

// Observation does not infer freshness from a value, including a zero value.
// Raw platform constants must be normalized by a resource adapter.
type Observation[T any] struct {
	Value      T
	Quality    Quality
	ObservedAt time.Time
	Source     string
}

type MemoryPressure uint8

const (
	PressureUnknown MemoryPressure = iota
	PressureNormal
	PressureWarning
	PressureCritical
)

type CPUUsage struct {
	TotalPercent        float64
	OwnedPercent        float64
	OtherPercent        float64
	AttributionReliable bool
}

type ResourceSnapshot struct {
	MemoryPressure       Observation[MemoryPressure]
	AvailableMemoryBytes Observation[uint64]
	CPU                  Observation[CPUUsage]
	GPUPercent           Observation[float64]
}

type SignalKind string

const (
	SignalNormal           SignalKind = "normal"
	SignalWarning          SignalKind = "warning"
	SignalCritical         SignalKind = "critical"
	SignalObservationGuard SignalKind = "observation_guard"
)

type ReasonCode string

const (
	ReasonNone                 ReasonCode = ""
	ReasonTelemetryUnavailable ReasonCode = "telemetry_unavailable"
	ReasonTelemetryStale       ReasonCode = "telemetry_stale"
	ReasonMemoryPressure       ReasonCode = "memory_pressure"
)

// ResourceSignal is an input to the future single-owner coordinator.
// It is not an admission grant or an instruction to terminate an engine.
type ResourceSignal struct {
	Kind   SignalKind
	Reason ReasonCode
}

type ResourceAssessment struct {
	AssessedAt time.Time
	Snapshot   ResourceSnapshot
	Signal     ResourceSignal
	ReadFailed bool
}

type Decision struct {
	At   time.Time
	Code string
}
