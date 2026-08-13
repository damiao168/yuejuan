package scorerelease

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

const (
	tenant = "tenant-1"
	exam   = "exam-1"
	actor  = "manager-1"
)

func TestPublishedReleaseIsImmutableAndNewReleaseCarriesCorrection(t *testing.T) {
	store := NewMemoryStore()
	service := NewService(store)
	store.SeedFacts(exam, []SubmissionFact{fixtureFact(4)})
	first, err := service.Create(context.Background(), tenant, exam, actor, CreateInput{Source: SourceInitial, Reason: "initial verified release", IdempotencyKey: "release-key-0001", VisibilityPolicy: VisibilityPolicy{ShowQuestionScores: true, ShowFeedback: true, ShowRubricSummary: true}})
	if err != nil {
		t.Fatalf("create first release: %v", err)
	}
	if _, err := service.Publish(context.Background(), tenant, first.ID, actor); err != nil {
		t.Fatalf("publish first release: %v", err)
	}

	store.SeedFacts(exam, []SubmissionFact{fixtureFact(5)})
	second, err := service.Create(context.Background(), tenant, exam, actor, CreateInput{Source: SourceAppeal, Reason: "appeal correction reviewed", IdempotencyKey: "release-key-0002", VisibilityPolicy: VisibilityPolicy{ShowQuestionScores: true}})
	if err != nil {
		t.Fatalf("create corrected release: %v", err)
	}
	if second.Version != 2 {
		t.Fatalf("expected version 2, got %#v", second)
	}
	diff, err := service.Diff(context.Background(), tenant, second.ID, first.ID)
	if err != nil || diff.AffectedCount != 1 || len(diff.Questions) != 1 || diff.Questions[0].OldScore != 4 || diff.Questions[0].NewScore != 5 {
		t.Fatalf("expected a question-level correction diff, got %#v err=%v", diff, err)
	}
	if _, err := service.Publish(context.Background(), tenant, second.ID, actor); err != nil {
		t.Fatalf("publish corrected release: %v", err)
	}

	old, err := service.Get(context.Background(), tenant, first.ID)
	if err != nil || old.Items[0].TotalScore != 4 || old.Questions[0].Score != 4 || old.Release.Status != StatusPublished {
		t.Fatalf("published first release was changed: %#v err=%v", old, err)
	}
	current, err := service.StudentResult(context.Background(), tenant, exam, "student-1")
	if err != nil || current.TotalScore != 5 || current.ReleaseVersion != 2 {
		t.Fatalf("student should see new release only: %#v err=%v", current, err)
	}
}

func TestPublishRecomputesGateAndRollbackIsAnotherVersion(t *testing.T) {
	store := NewMemoryStore()
	service := NewService(store)
	store.SeedFacts(exam, []SubmissionFact{fixtureFact(4)})
	first, err := service.Create(context.Background(), tenant, exam, actor, CreateInput{Reason: "initial", IdempotencyKey: "release-key-0011", VisibilityPolicy: VisibilityPolicy{ShowQuestionScores: true}})
	if err != nil {
		t.Fatal(err)
	}
	store.SetGateIssues(exam, []GateIssue{{Code: "open_quality_incident", Message: "unreviewed critical incident", Blocking: true, Count: 1}})
	if _, err := service.Publish(context.Background(), tenant, first.ID, actor); !errors.Is(err, ErrGateBlocked) {
		t.Fatalf("publish must recheck a newly blocked gate, got %v", err)
	}
	store.SetGateIssues(exam, nil)
	if _, err := service.Publish(context.Background(), tenant, first.ID, actor); err != nil {
		t.Fatal(err)
	}

	store.SeedFacts(exam, []SubmissionFact{fixtureFact(2)})
	second, err := service.Create(context.Background(), tenant, exam, actor, CreateInput{Source: SourceRegrade, Reason: "regrade", IdempotencyKey: "release-key-0012", VisibilityPolicy: VisibilityPolicy{ShowQuestionScores: true}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Publish(context.Background(), tenant, second.ID, actor); err != nil {
		t.Fatal(err)
	}

	rollback, err := service.CreateRollback(context.Background(), tenant, exam, actor, RollbackInput{SourceReleaseID: first.ID, Reason: "restore verified release", IdempotencyKey: "release-key-0013"})
	if err != nil {
		t.Fatal(err)
	}
	if rollback.Version != 3 || rollback.Source != SourceRollback || rollback.SourceReleaseID != first.ID {
		t.Fatalf("rollback must create a traced new draft: %#v", rollback)
	}
	if _, err := service.Publish(context.Background(), tenant, rollback.ID, actor); err != nil {
		t.Fatal(err)
	}
	current, err := service.StudentResult(context.Background(), tenant, exam, "student-1")
	if err != nil || current.TotalScore != 4 || current.ReleaseVersion != 3 {
		t.Fatalf("rollback should publish a new immutable version: %#v err=%v", current, err)
	}
}

func TestStudentResultCannotLeakInternalReleaseFieldsAndRespectsAppealWindow(t *testing.T) {
	store := NewMemoryStore()
	fixed := time.Date(2026, 8, 11, 12, 0, 0, 0, time.UTC)
	store.SetNow(func() time.Time { return fixed })
	service := NewService(store)
	store.SeedFacts(exam, []SubmissionFact{fixtureFact(4)})
	closes := fixed.Add(time.Hour)
	release, err := service.Create(context.Background(), tenant, exam, actor, CreateInput{Reason: "student safe", IdempotencyKey: "release-key-0021", VisibilityPolicy: VisibilityPolicy{ShowQuestionScores: true, ShowFeedback: true, ShowRubricSummary: true}, AppealWindow: AppealWindow{Enabled: true, ClosesAt: &closes, AllowedReasonCodes: []string{"recognition_error", "rubric_disagreement"}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Publish(context.Background(), tenant, release.ID, actor); err != nil {
		t.Fatal(err)
	}
	result, err := service.StudentResult(context.Background(), tenant, exam, "student-1")
	if err != nil {
		t.Fatal(err)
	}
	bytes, _ := json.Marshal(result)
	for _, forbidden := range []string{"final_grade_id", "source_id", "private_note", "reviewer", "prompt", "quality"} {
		if strings.Contains(string(bytes), forbidden) {
			t.Fatalf("student result leaked %q: %s", forbidden, bytes)
		}
	}
	if !result.AppealWindow.Open || len(result.AppealWindow.AllowedReasonCodes) != 2 || result.Questions[0].Feedback != "Teacher feedback" {
		t.Fatalf("unexpected student-safe result: %#v", result)
	}
}

func TestStudentQuestionImageRequiresCurrentPublishedQuestionAndStudentScope(t *testing.T) {
	store := NewMemoryStore()
	service := NewService(store)
	store.SeedFacts(exam, []SubmissionFact{fixtureFact(4)})
	release, err := service.Create(context.Background(), tenant, exam, actor, CreateInput{
		Reason: "student answer image", IdempotencyKey: "release-key-0022", VisibilityPolicy: VisibilityPolicy{ShowQuestionScores: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Publish(context.Background(), tenant, release.ID, actor); err != nil {
		t.Fatal(err)
	}
	store.SeedStudentQuestionImage(tenant, exam, "student-1", "question-1", "segment-1")
	source, err := service.StudentQuestionImage(context.Background(), tenant, exam, "student-1", "question-1")
	if err != nil || source.AnswerSegmentID != "segment-1" {
		t.Fatalf("student image source = %#v, %v", source, err)
	}
	if _, err := service.StudentQuestionImage(context.Background(), tenant, exam, "other-student", "question-1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("other student image error = %v, want ErrNotFound", err)
	}
}

func fixtureFact(score float64) SubmissionFact {
	return SubmissionFact{StudentID: "student-1", SubmissionID: "submission-1", TotalScore: score, MaxScore: 5, Status: "confirmed", Questions: []QuestionFact{{QuestionID: "question-1", QuestionNo: "Q1", FinalGradeID: "final-1", Score: score, MaxScore: 5, SourceType: "single_review", SourceID: "human-grade-1", Explanation: StudentExplanation{Feedback: "Teacher feedback", RubricSummary: []string{"Shows the required method"}}}}}
}
