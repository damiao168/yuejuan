package calibration

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"edugrade-enterprise/services/api-gateway/internal/goldpaper"
)

func TestCalibrationProducesSampleAwareMetricsAndQualification(t *testing.T) {
	service, _, _ := testService(t, 2)
	session, err := service.CreateSession(context.Background(), "tenant-a", "exam-1", "question-1", "grader-1")
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(session)
	if strings.Contains(string(encoded), "reference_score") || strings.Contains(string(encoded), "expected_criteria") {
		t.Fatalf("unsubmitted reference answer leaked: %s", encoded)
	}
	if len(session.Samples) != 2 {
		t.Fatalf("expected two Gold samples, got %+v", session.Samples)
	}

	_, progress, qualification, err := service.SubmitAttempt(context.Background(), "tenant-a", session.ID, SubmitAttemptInput{
		GoldPaperID: session.Samples[0].GoldPaperID, SubmittedScore: 2,
		RubricSelections: map[string]any{"concept": true, "evidence": "partial"},
	})
	if err != nil || qualification != nil || progress.SubmittedCount != 1 {
		t.Fatalf("unexpected progress: %+v %+v %v", progress, qualification, err)
	}
	_, completed, qualification, err := service.SubmitAttempt(context.Background(), "tenant-a", session.ID, SubmitAttemptInput{
		GoldPaperID: session.Samples[1].GoldPaperID, SubmittedScore: 3,
		RubricSelections: map[string]any{"concept": true, "evidence": "full"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if completed.Status != SessionPassed || qualification == nil || qualification.Status != QualificationQualified {
		t.Fatalf("expected passed qualification: %+v %+v", completed, qualification)
	}
	if qualification.Metrics.SampleCount != 2 || qualification.Metrics.CriterionSampleCount != 4 ||
		qualification.Metrics.CriterionAgreement == nil || *qualification.Metrics.CriterionAgreement != 1 {
		t.Fatalf("metrics must retain denominators: %+v", qualification.Metrics)
	}
	if err := service.RequireQualification(context.Background(), "tenant-a", "exam-1", "question-1", "grader-1"); err != nil {
		t.Fatalf("claim gate rejected current qualification: %v", err)
	}
}

func TestCalibrationReportsRubricPointBiasAndFailsConfiguredPolicy(t *testing.T) {
	service, _, _ := testService(t, 1)
	criterion := 1.0
	_, err := service.PutPolicy(context.Background(), "tenant-a", "exam-1", "question-1", PutPolicyInput{
		ArchetypeCode: "short_constructed", MaxScore: 5, MinimumSamples: 1, MaximumMAE: 5,
		MinimumExactAgreement: 0, MinimumWithinOneAgreement: 0, MinimumCriterionAgreement: &criterion,
		MaximumSevereRate: 1, SevereErrorThreshold: 4, QualificationValidityDays: 30, ExpectedRevision: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	session, err := service.CreateSession(context.Background(), "tenant-a", "exam-1", "question-1", "grader-2")
	if err != nil {
		t.Fatal(err)
	}
	attempt, completed, qualification, err := service.SubmitAttempt(context.Background(), "tenant-a", session.ID, SubmitAttemptInput{
		GoldPaperID: session.Samples[0].GoldPaperID, SubmittedScore: 2,
		RubricSelections: map[string]any{"concept": false, "evidence": "partial"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if completed.Status != SessionFailed || qualification.Status != QualificationRevoked || len(attempt.CriterionDifferences) != 1 {
		t.Fatalf("expected criterion-level failure: attempt=%+v session=%+v qualification=%+v", attempt, completed, qualification)
	}
	if attempt.CriterionDifferences[0].Criterion != "concept" {
		t.Fatalf("wrong rubric point difference: %+v", attempt.CriterionDifferences)
	}
	if err := service.RequireQualification(context.Background(), "tenant-a", "exam-1", "question-1", "grader-2"); err != ErrQualificationRequired {
		t.Fatalf("failed grader passed claim gate: %v", err)
	}
}

func TestGoldVersionChangeRevokesQualification(t *testing.T) {
	service, gold, _ := testService(t, 1)
	session, err := service.CreateSession(context.Background(), "tenant-a", "exam-1", "question-1", "grader-1")
	if err != nil {
		t.Fatal(err)
	}
	_, _, qualification, err := service.SubmitAttempt(context.Background(), "tenant-a", session.ID, SubmitAttemptInput{
		GoldPaperID: session.Samples[0].GoldPaperID, SubmittedScore: 2,
		RubricSelections: map[string]any{"concept": true, "evidence": "partial"},
	})
	if err != nil || qualification.Status != QualificationQualified {
		t.Fatalf("qualification setup failed: %+v %v", qualification, err)
	}
	item, err := gold.CreateVersion(context.Background(), "tenant-a", session.Samples[0].GoldPaperID, "chief", goldpaper.CreateVersionInput{
		ReferenceScore: 3, Explanation: "rubric revision", TraitScores: map[string]any{"concept": true, "evidence": "full"}, SourceGradeIDs: []string{"grade-0"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = gold.Approve(context.Background(), "tenant-a", item.ID, "approver", 2); err != nil {
		t.Fatal(err)
	}

	currentQualification, err := service.GetQualification(context.Background(), "tenant-a", "exam-1", "question-1", "grader-1")
	if err != nil {
		t.Fatal(err)
	}
	if currentQualification.Status != QualificationRevoked {
		t.Fatalf("Gold change did not invalidate qualification: %+v", currentQualification)
	}
	if err := service.RequireQualification(context.Background(), "tenant-a", "exam-1", "question-1", "grader-1"); err != ErrQualificationRequired {
		t.Fatalf("stale qualification passed claim gate: %v", err)
	}
}

func TestPolicyIsQuestionSpecificAndOptimisticallyVersioned(t *testing.T) {
	store := NewMemoryStore()
	service := NewService(store, goldpaper.NewMemoryStore())
	input := PutPolicyInput{ArchetypeCode: "extended_response", MaxScore: 60, MinimumSamples: 4,
		MaximumMAE: 2.5, MinimumExactAgreement: .25, MinimumWithinOneAgreement: .6,
		MaximumSevereRate: .1, SevereErrorThreshold: 8, QualificationValidityDays: 14}
	policy, err := service.PutPolicy(context.Background(), "tenant", "exam", "essay", input)
	if err != nil || policy.Revision != 1 || policy.MaxScore != 60 {
		t.Fatalf("unexpected essay policy: %+v %v", policy, err)
	}
	input.ExpectedRevision = 0
	if _, err = service.PutPolicy(context.Background(), "tenant", "exam", "essay", input); err != ErrConflict {
		t.Fatalf("stale policy write accepted: %v", err)
	}
}

func testService(t *testing.T, goldCount int) (*Service, *goldpaper.MemoryStore, *MemoryStore) {
	t.Helper()
	ctx := context.Background()
	gold := goldpaper.NewMemoryStore()
	for index := 0; index < goldCount; index++ {
		submission, grade := "submission-"+string(rune('a'+index)), "grade-"+string(rune('0'+index))
		gold.SetSource("tenant-a", goldpaper.Source{ExamID: "exam-1", QuestionID: "question-1", SubmissionID: submission,
			SnapshotID: "snapshot-1", SubjectCode: "biology", ArchetypeCode: "short_constructed", RiskTier: "R3", MaxScore: 5,
			RubricSnapshot: map[string]any{"criteria": []any{"concept", "evidence"}}, AvailableGradeIDs: []string{grade}})
		score, evidence := float64(2+index), "partial"
		if index > 0 {
			evidence = "full"
		}
		item, err := gold.Nominate(ctx, "tenant-a", "exam-1", "question-1", "chief", goldpaper.NominateInput{
			SubmissionID: submission, ReferenceScore: score, Explanation: "approved calibration sample",
			TraitScores: map[string]any{"concept": true, "evidence": evidence}, SourceGradeIDs: []string{grade},
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err = gold.Approve(ctx, "tenant-a", item.ID, "approver", 1); err != nil {
			t.Fatal(err)
		}
	}
	store := NewMemoryStore()
	service := NewService(store, gold)
	criterion := .75
	_, err := service.PutPolicy(ctx, "tenant-a", "exam-1", "question-1", PutPolicyInput{
		ArchetypeCode: "short_constructed", MaxScore: 5, MinimumSamples: goldCount, MaximumMAE: .5,
		MinimumExactAgreement: .5, MinimumWithinOneAgreement: 1, MinimumCriterionAgreement: &criterion,
		MaximumSevereRate: 0, SevereErrorThreshold: 2, QualificationValidityDays: 30,
	})
	if err != nil {
		t.Fatal(err)
	}
	service.now = func() time.Time { return time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC) }
	return service, gold, store
}
