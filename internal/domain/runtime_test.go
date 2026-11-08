package domain_test

import (
	"testing"
	"time"

	"github.com/AtomsH4/adaptive-ai-gateway/internal/domain"
)

func TestResourceReleaseRequiresEvidenceForTheCurrentIncarnation(t *testing.T) {
	ref := domain.ExecutionRef{Instance: domain.InstanceRef{ID: "local-engine", Epoch: 8}, ID: "run-2"}
	evidence := domain.ReleaseEvidence{Instance: ref.Instance, Execution: ref.ID, Kind: domain.EvidenceCancellationConfirmed, ObservedAt: time.Now()}
	if !evidence.ConfirmsExecution(ref) {
		t.Fatal("confirmed matching cancellation rejected")
	}
	old := evidence
	old.Instance.Epoch--
	if old.ConfirmsExecution(ref) {
		t.Fatal("old incarnation could release current capacity")
	}
	wrong := evidence
	wrong.Execution = "run-1"
	if wrong.ConfirmsExecution(ref) {
		t.Fatal("old execution could release new execution's slot")
	}
	requested := evidence
	requested.Kind = "cancel_requested"
	if requested.ConfirmsExecution(ref) {
		t.Fatal("cancellation request treated as completion")
	}
	unknown := domain.ReleaseEvidence{}
	if unknown.ConfirmsExecution(ref) {
		t.Fatal("missing evidence treated as completion")
	}
	exited := evidence
	exited.Kind, exited.Execution = domain.EvidenceInstanceExited, ""
	if !exited.ConfirmsExecution(ref) {
		t.Fatal("confirmed current instance exit did not cover its execution")
	}
	if evidence.ConfirmsExecution(domain.ExecutionRef{}) {
		t.Fatal("empty request reference accepted")
	}
}
