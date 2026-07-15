package grading

import "testing"

func TestOMRAutoConfirmationRequiresServerEligibility(t *testing.T) {
	input := OMRResultInput{Decision: "selected", Selected: []string{"A"}, Confidence: 0.99, NeedsHumanReview: false}
	if omrResultCanAutoConfirm(input, false, 0.9) {
		t.Fatal("high worker confidence must not bypass the server eligibility snapshot")
	}
	if got := omrReviewReason(input, false, "manual_only_profile", 0.9); got != "omr_manual_only_profile" {
		t.Fatalf("manual-only result should have an actionable review reason, got %q", got)
	}
	if !omrResultCanAutoConfirm(input, true, 0.98) {
		t.Fatal("an eligible profile should retain the normal confidence gate")
	}
	if omrResultCanAutoConfirm(OMRResultInput{Decision: "selected", Selected: []string{"A", "B"}, Confidence: 1}, true, 0.98) {
		t.Fatal("multiple selected options must remain outside the calibrated single-choice auto-confirm branch")
	}
}

func TestOMRReviewReasonKeepsWorkerAmbiguity(t *testing.T) {
	input := OMRResultInput{Decision: "selected", Confidence: 0.99, NeedsHumanReview: true}
	if got := omrReviewReason(input, false, "manual_only_profile", 0.9); got != "omr_selected" {
		t.Fatalf("worker-requested review should keep the extraction reason, got %q", got)
	}
	input = OMRResultInput{Decision: "blank", Confidence: 0.1}
	if got := omrReviewReason(input, false, "manual_only_profile", 0.9); got != "omr_blank" {
		t.Fatalf("non-selected result should keep its extraction reason, got %q", got)
	}
}
