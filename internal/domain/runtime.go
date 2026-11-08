package domain

import "time"

type ModelRef struct {
	ID             string
	Revision       string
	ArtifactSHA256 string
}

// RuntimeProfileRef identifies immutable serving configuration, not model weights.
// Backend-specific knobs stay in that adapter's validated configuration.
type RuntimeProfileRef struct {
	ID       string
	Revision string
}

// InstanceRef identifies a logical engine incarnation, never merely an OS PID.
type InstanceRef struct {
	ID    string
	Epoch uint64
}

type ExecutionRef struct {
	Instance InstanceRef
	ID       string
}

type Ownership string

const (
	OwnershipManaged  Ownership = "managed"
	OwnershipAttached Ownership = "attached"
)

// False capability values mean unsupported or not verified, never optimistic.
type BackendCapabilities struct {
	CanStopOwnedInstance    bool
	CanConfirmCancellation  bool
	CanObserveExecutionEnd  bool
	CanReportTokenUsage     bool
	CanValidateContextLimit bool
}

type BackendDescription struct {
	Name            string
	ProtocolVersion string
	Ownership       Ownership
	Capabilities    BackendCapabilities
}

type LoadCommand struct {
	Instance       InstanceRef
	Model          ModelRef
	Profile        RuntimeProfileRef
	ContextTokens  int
	MaxConcurrency int
}

type Message struct {
	Role    string
	Content string
}

type ExecutionCommand struct {
	Execution       ExecutionRef
	Model           ModelRef
	Profile         RuntimeProfileRef
	Messages        []Message
	MaxOutputTokens int
	Temperature     float64
}

type EvidenceKind string

const (
	EvidenceExecutionCompleted    EvidenceKind = "execution_completed"
	EvidenceCancellationConfirmed EvidenceKind = "cancellation_confirmed"
	EvidenceInstanceExited        EvidenceKind = "instance_exited"
)

// ReleaseEvidence must come from a trusted backend adapter's observation.
// A cancellation request, a closed client socket, or expired lease is not evidence.
type ReleaseEvidence struct {
	Instance   InstanceRef
	Execution  string
	Kind       EvidenceKind
	ObservedAt time.Time
}

// ConfirmsExecution rejects missing evidence and events from old incarnations.
// It validates scope; it cannot independently prove that an adapter is truthful.
func (e ReleaseEvidence) ConfirmsExecution(ref ExecutionRef) bool {
	if ref.Instance.ID == "" || ref.Instance.Epoch == 0 || ref.ID == "" ||
		e.Instance != ref.Instance || e.ObservedAt.IsZero() {
		return false
	}
	switch e.Kind {
	case EvidenceInstanceExited:
		return true
	case EvidenceExecutionCompleted, EvidenceCancellationConfirmed:
		return e.Execution == ref.ID
	default:
		return false
	}
}

type InstanceState string

const (
	InstanceUnknown  InstanceState = "unknown"
	InstanceLoading  InstanceState = "loading"
	InstanceReady    InstanceState = "ready"
	InstanceStopping InstanceState = "stopping"
	InstanceExited   InstanceState = "exited"
)

type InstanceObservation struct {
	Instance InstanceRef
	State    InstanceState
	Evidence *ReleaseEvidence
}

type TokenUsage struct {
	Input  uint64
	Output uint64
}

type GenerationEventKind string

const (
	GenerationDelta    GenerationEventKind = "delta"
	GenerationFinished GenerationEventKind = "finished"
	GenerationFailed   GenerationEventKind = "failed"
)

// GenerationEvent contains semantic output, not HTTP/SSE bytes.
// Finished output still requires separate release evidence before slot reuse.
type GenerationEvent struct {
	Execution    ExecutionRef
	Kind         GenerationEventKind
	Text         string
	FinishReason string
	FailureCode  string
	Usage        *TokenUsage
}
