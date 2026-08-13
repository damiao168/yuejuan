package releasegate

import (
	"context"
	"errors"
	"testing"
	"time"

	"edugrade-enterprise/services/api-gateway/internal/scorerelease"
)

type fakeBaseGate struct{ gate scorerelease.Gate }

func (f fakeBaseGate) Gate(context.Context, string, string) (scorerelease.Gate, error) {
	return f.gate, nil
}

func TestWarningWaiverCanSatisfyVersionedPolicyWithoutBypassingBlockers(t *testing.T) {
	now := time.Date(2026, 8, 11, 9, 0, 0, 0, time.UTC)
	store := NewMemoryStore()
	store.SetNow(func() time.Time { return now })
	base := fakeBaseGate{gate: scorerelease.Gate{Version: "A18", Passed: true, Warnings: []scorerelease.GateIssue{{Code: "quality_warning", Blocking: false, Count: 1}}, Blocking: []scorerelease.GateIssue{}, Counts: map[string]int{}}}
	service := NewService(store, base)
	service.now = func() time.Time { return now }
	policy, err := service.CreatePolicy(context.Background(), "tenant", "exam", "manager", CreatePolicyInput{Version: "school-2026.1", RequireWarningAcknowledgement: true, WaivableWarningCodes: []string{"quality_warning"}})
	if err != nil {
		t.Fatalf("create policy: %v", err)
	}
	preview, err := service.Preview(context.Background(), "tenant", "exam", "release", "manager")
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if preview.Evaluation.Policy.ID != policy.ID || preview.Evaluation.Passed || len(preview.Evaluation.PendingWarningCodes) != 1 {
		t.Fatalf("unexpected preview: %#v", preview.Evaluation)
	}
	waiver, err := service.RequestWaiver(context.Background(), "tenant", "exam", "manager", RequestWaiverInput{EvidenceID: preview.ID, IssueCode: "quality_warning", Reason: "The issue is documented and non-blocking."})
	if err != nil {
		t.Fatalf("request waiver: %v", err)
	}
	if _, err := service.DecideWaiver(context.Background(), "tenant", waiver.ID, "principal", DecideWaiverInput{Approve: true, Reason: "Approved after review."}); err != nil {
		t.Fatalf("approve waiver: %v", err)
	}
	publish, err := service.RecheckForPublish(context.Background(), "tenant", "exam", "release", "principal")
	if err != nil {
		t.Fatalf("publish recheck: %v", err)
	}
	if !publish.Evaluation.Passed || len(publish.Evaluation.AppliedWaiverIDs) != 1 {
		t.Fatalf("approved warning not applied: %#v", publish.Evaluation)
	}
}

func TestBlockerCannotBeRequestedOrApprovedAsWaiver(t *testing.T) {
	store := NewMemoryStore()
	base := fakeBaseGate{gate: scorerelease.Gate{Version: "A18", Passed: false, Blocking: []scorerelease.GateIssue{{Code: "unidentified_submission", Blocking: true, Count: 1}}, Warnings: []scorerelease.GateIssue{{Code: "unidentified_submission", Blocking: false, Count: 1}}, Counts: map[string]int{}}}
	service := NewService(store, base)
	if _, err := service.CreatePolicy(context.Background(), "tenant", "exam", "manager", CreatePolicyInput{Version: "school-2026.1", RequireWarningAcknowledgement: true, WaivableWarningCodes: []string{"unidentified_submission"}}); err != nil {
		t.Fatal(err)
	}
	preview, err := service.Preview(context.Background(), "tenant", "exam", "release", "manager")
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.RequestWaiver(context.Background(), "tenant", "exam", "manager", RequestWaiverInput{EvidenceID: preview.ID, IssueCode: "unidentified_submission", Reason: "try bypass"})
	if !errors.Is(err, ErrForbiddenWaiver) {
		t.Fatalf("expected blocker waiver rejection, got %v", err)
	}
	_, err = service.RecheckForPublish(context.Background(), "tenant", "exam", "release", "manager")
	if !errors.Is(err, ErrGateBlocked) {
		t.Fatalf("expected publish gate to remain blocked, got %v", err)
	}
}

func TestDefaultPolicyStillPersistsVersionedEvidenceAndDoesNotWaiveWarnings(t *testing.T) {
	store := NewMemoryStore()
	base := fakeBaseGate{gate: scorerelease.Gate{Version: "A18", Passed: true, Blocking: []scorerelease.GateIssue{}, Warnings: []scorerelease.GateIssue{{Code: "routine", Blocking: false, Count: 1}}, Counts: map[string]int{}}}
	service := NewService(store, base)
	evidence, err := service.RecheckForPublish(context.Background(), "tenant", "exam", "release", "manager")
	if err != nil {
		t.Fatal(err)
	}
	if evidence.Evaluation.Policy.Version != DefaultPolicyVersion || !evidence.Evaluation.Passed {
		t.Fatalf("unexpected default evidence: %#v", evidence)
	}
	_, err = service.RequestWaiver(context.Background(), "tenant", "exam", "manager", RequestWaiverInput{EvidenceID: evidence.ID, IssueCode: "routine", Reason: "not possible"})
	if !errors.Is(err, ErrForbiddenWaiver) {
		t.Fatalf("default policy must not allow waiver: %v", err)
	}
}
