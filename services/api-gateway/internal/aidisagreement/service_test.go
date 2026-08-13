package aidisagreement

import (
	"context"
	"errors"
	"testing"
)

func TestCaptureUsesActualSourceAndIsIdempotent(t *testing.T) {
	store := NewMemoryStore()
	service := NewService(store)
	store.AddActualComparison("tenant-1", comparison())
	input := CaptureInput{AICandidateID: "ai-1", HumanGradeID: "human-1"}
	item, created, err := service.Capture(context.Background(), "tenant-1", input)
	if err != nil || !created {
		t.Fatalf("capture = (%#v,%v,%v), want created", item, created, err)
	}
	if item.Severity != SeveritySevere || item.Status != StatusNeedsReview || item.DifferenceType != DifferenceScoreEvidence {
		t.Fatalf("unexpected disagreement %#v", item)
	}
	if item.Delta != 4 || item.AbsoluteDelta != 4 || item.RiskTier != "R3" {
		t.Fatalf("must preserve actual aggregate facts, got %#v", item)
	}
	if !contains(item.TriggerRules, "r3_priority") || !contains(item.TriggerRules, "missing_ai_evidence") {
		t.Fatalf("missing deterministic triggers %#v", item.TriggerRules)
	}
	again, created, err := service.Capture(context.Background(), "tenant-1", input)
	if err != nil || created || again.ID != item.ID {
		t.Fatalf("retry must return stored event, got (%#v,%v,%v)", again, created, err)
	}
}

func TestCaptureSkipsExactComparableFacts(t *testing.T) {
	store := NewMemoryStore()
	service := NewService(store)
	source := comparison()
	source.AISuggestedScore, source.HumanScore, source.AIEvidenceCount = 3, 3, 2
	source.AIMatchedCriterionCount, source.HumanCriterionCount = 2, 2
	store.AddActualComparison("tenant-1", source)
	item, created, err := service.Capture(context.Background(), "tenant-1", CaptureInput{AICandidateID: "ai-1", HumanGradeID: "human-1"})
	if err != nil || created || item.ID != "" {
		t.Fatalf("exact comparable pair must not enter queue: (%#v,%v,%v)", item, created, err)
	}
}

func TestClassificationRouteAndDatasetDoNotChangeScoreFacts(t *testing.T) {
	store := NewMemoryStore()
	service := NewService(store)
	store.AddActualComparison("tenant-1", comparison())
	item, _, err := service.Capture(context.Background(), "tenant-1", CaptureInput{AICandidateID: "ai-1", HumanGradeID: "human-1"})
	if err != nil {
		t.Fatal(err)
	}
	if dataset, err := service.Dataset(context.Background(), "tenant-1", Filter{}); err != nil || len(dataset) != 0 {
		t.Fatalf("unclassified issue must not be exported: %#v %v", dataset, err)
	}
	classified, err := service.Classify(context.Background(), "tenant-1", item.ID, "reviewer-1", ClassifyInput{
		Taxonomy: TaxonomyOCRError, Notes: "image quality needs correction", ExpectedRevision: item.Revision,
	})
	if err != nil || classified.Status != StatusClassified || classified.AICandidateScore != item.AICandidateScore || classified.HumanScore != item.HumanScore {
		t.Fatalf("classification changed source facts or failed: %#v %v", classified, err)
	}
	routed, err := service.Route(context.Background(), "tenant-1", item.ID, "manager-1", RouteInput{ReviewTaskID: "review-task-1", ExpectedRevision: classified.Revision})
	if err != nil || routed.Status != StatusRouted || routed.RoutedReviewTaskID != "review-task-1" {
		t.Fatalf("route = %#v %v", routed, err)
	}
	if _, err = service.Classify(context.Background(), "tenant-1", item.ID, "reviewer-1", ClassifyInput{Taxonomy: TaxonomyAIScoringError, ExpectedRevision: routed.Revision}); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("routed item must be immutable, got %v", err)
	}
	dataset, err := service.Dataset(context.Background(), "tenant-1", Filter{})
	if err != nil || len(dataset) != 1 || dataset[0].LineageRef == item.ID || dataset[0].HumanScore != item.HumanScore {
		t.Fatalf("dataset must be deidentified with aligned facts: %#v %v", dataset, err)
	}
	if !contains(RecommendedFollowUps(classified.Taxonomy), "ocr_correction_candidate") {
		t.Fatalf("taxonomy should propose, not execute, an OCR correction")
	}
}

func comparison() ActualComparison {
	return ActualComparison{
		ExamID: "exam-1", QuestionID: "question-1", SubmissionID: "submission-1", AnswerSegmentID: "segment-1",
		AICandidateID: "ai-1", HumanGradeID: "human-1", AISuggestedScore: 5, HumanScore: 1, MaxScore: 5,
		AIConfidence: .9, AIEvidenceCount: 0, AIMatchedCriterionCount: 1, HumanCriterionCount: 1, RiskTier: "R3",
	}
}

func contains(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}
