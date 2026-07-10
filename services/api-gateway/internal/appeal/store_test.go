package appeal

import (
	"context"
	"errors"
	"testing"
)

const tenantID = "tenant-1"

func TestCreateAppealRequiresPublishedGradeAndSupportsTargets(t *testing.T) {
	store := seededAppealStore()

	examAppeal, err := store.CreateAppeal(context.Background(), tenantID, "student-user-1", CreateAppealInput{
		ExamID:     "exam-1",
		StudentID:  "student-1",
		TargetType: "exam",
		Reason:     "total score looks wrong",
	})
	if err != nil {
		t.Fatalf("create exam appeal: %v", err)
	}
	if examAppeal.Status != "submitted" || examAppeal.SubmissionGradeID == "" {
		t.Fatalf("exam appeal mismatch: %#v", examAppeal)
	}

	questionAppeal, err := store.CreateAppeal(context.Background(), tenantID, "student-user-1", CreateAppealInput{
		ExamID:       "exam-1",
		StudentID:    "student-1",
		TargetType:   "question",
		FinalGradeID: "final-1",
		Reason:       "Q1 should receive full credit",
	})
	if err != nil {
		t.Fatalf("create question appeal: %v", err)
	}
	if questionAppeal.QuestionNo != "Q1" || questionAppeal.Evidence == nil || questionAppeal.Evidence.RawAnswer == "" {
		t.Fatalf("question appeal should include evidence, got %#v", questionAppeal)
	}

	deductionAppeal, err := store.CreateAppeal(context.Background(), tenantID, "student-user-1", CreateAppealInput{
		ExamID:           "exam-1",
		StudentID:        "student-1",
		TargetType:       "deduction_point",
		FinalGradeID:     "final-1",
		DeductionPointID: "deduct-unit",
		Reason:           "unit deduction is incorrect",
	})
	if err != nil {
		t.Fatalf("create deduction appeal: %v", err)
	}
	if deductionAppeal.DeductionPointID != "deduct-unit" {
		t.Fatalf("deduction appeal mismatch: %#v", deductionAppeal)
	}

	_, err = store.CreateAppeal(context.Background(), tenantID, "student-user-2", CreateAppealInput{
		ExamID:       "exam-1",
		StudentID:    "student-2",
		TargetType:   "question",
		FinalGradeID: "final-1",
		Reason:       "not published",
	})
	if !errors.Is(err, ErrUnpublishedGrade) {
		t.Fatalf("unpublished grade should not be appealable, got %v", err)
	}
}

func TestReviewAppealAdjustsScoreAndStatistics(t *testing.T) {
	store := seededAppealStore()
	item, err := store.CreateAppeal(context.Background(), tenantID, "student-user-1", CreateAppealInput{
		ExamID:       "exam-1",
		StudentID:    "student-1",
		TargetType:   "question",
		FinalGradeID: "final-1",
		Reason:       "Q1 should receive full credit",
	})
	if err != nil {
		t.Fatalf("create appeal: %v", err)
	}
	newScore := 5.0
	reviewed, adjustment, err := store.ReviewAppeal(context.Background(), tenantID, item.ID, "teacher-1", ReviewAppealInput{
		Status:        "score_adjusted",
		Reason:        "rubric evidence supports full credit",
		AdjustedScore: &newScore,
	})
	if err != nil {
		t.Fatalf("review appeal: %v", err)
	}
	if reviewed.Status != "score_adjusted" || adjustment == nil || adjustment.PreviousScore != 4 || adjustment.AdjustedScore != 5 {
		t.Fatalf("score adjustment mismatch: appeal=%#v adjustment=%#v", reviewed, adjustment)
	}
	if len(reviewed.Adjustments) != 1 || reviewed.Evidence.FinalGrade["score"].(float64) != 5 {
		t.Fatalf("reviewed appeal should include updated history and final grade, got %#v", reviewed)
	}
	closed, err := store.CloseAppeal(context.Background(), tenantID, item.ID, "teacher-1", CloseAppealInput{Reason: "resolved"})
	if err != nil {
		t.Fatalf("close appeal: %v", err)
	}
	if closed.Status != "closed" || closed.ClosedBy != "teacher-1" {
		t.Fatalf("closed appeal mismatch: %#v", closed)
	}
	stats, err := store.Statistics(context.Background(), tenantID, StatisticsFilter{ExamID: "exam-1"})
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if stats.Total != 1 || stats.ByStatus["closed"] != 1 || stats.ScoreAdjustedCount != 1 {
		t.Fatalf("statistics mismatch: %#v", stats)
	}
}

func TestReviewAppealDoesNotReopenTerminalStatus(t *testing.T) {
	store := seededAppealStore()
	item, err := store.CreateAppeal(context.Background(), tenantID, "student-user-1", CreateAppealInput{
		ExamID:       "exam-1",
		StudentID:    "student-1",
		TargetType:   "question",
		FinalGradeID: "final-1",
		Reason:       "Q1 should receive full credit",
	})
	if err != nil {
		t.Fatalf("create appeal: %v", err)
	}
	newScore := 5.0
	if _, _, err := store.ReviewAppeal(context.Background(), tenantID, item.ID, "teacher-1", ReviewAppealInput{
		Status:        "score_adjusted",
		Reason:        "rubric evidence supports full credit",
		AdjustedScore: &newScore,
	}); err != nil {
		t.Fatalf("score adjust appeal: %v", err)
	}
	if _, _, err := store.ReviewAppeal(context.Background(), tenantID, item.ID, "teacher-1", ReviewAppealInput{
		Status: "rejected",
		Reason: "second decision should not overwrite terminal result",
	}); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("terminal appeal should not be reopened, got %v", err)
	}
}

func seededAppealStore() *MemoryStore {
	store := NewMemoryStore()
	store.AddSubmissionGrade(SubmissionGradeSeed{
		ID:            "submission-grade-1",
		ExamID:        "exam-1",
		SubmissionID:  "submission-1",
		StudentID:     "student-1",
		AnonymousCode: "ANON-001",
		TotalScore:    8,
		MaxScore:      10,
		Status:        "published",
		Locked:        true,
	})
	store.AddSubmissionGrade(SubmissionGradeSeed{
		ID:            "submission-grade-2",
		ExamID:        "exam-1",
		SubmissionID:  "submission-2",
		StudentID:     "student-2",
		AnonymousCode: "ANON-002",
		TotalScore:    7,
		MaxScore:      10,
		Status:        "confirmed",
		Locked:        false,
	})
	store.AddFinalGrade(FinalGradeSeed{
		ID:              "final-1",
		ExamID:          "exam-1",
		SubmissionID:    "submission-1",
		AnswerSegmentID: "segment-1",
		QuestionID:      "question-1",
		QuestionNo:      "Q1",
		Score:           4,
		MaxScore:        5,
		Status:          "locked",
		Locked:          true,
		RawAnswer:       "student wrote a correct explanation",
		OCRText:         "student wrote a correct explanation",
		AIGrades:        []map[string]any{{"suggested_score": 4.0, "mock": false}},
		HumanGrades:     []map[string]any{{"score": 4.0, "reviewer_id": "reviewer-1"}},
		Rubric:          map[string]any{"points": []any{"main idea"}},
	})
	return store
}
