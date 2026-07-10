package review

import (
	"context"
	"errors"
	"testing"

	"edugrade-enterprise/services/api-gateway/internal/paper"
)

const tenantID = "tenant-1"

func TestReviewTaskWorkflowCreatesAssignsSubmitsAndReturns(t *testing.T) {
	store := NewMemoryStore()
	store.AddContext(tenantID, "segment-1", reviewContext())

	task, err := store.CreateTask(context.Background(), tenantID, "manager-1", CreateTaskInput{
		AnswerSegmentID: "segment-1",
		Source:          "evidence_verification_failed",
		Priority:        5,
	})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	if task.Status != "pending" || task.AnonymousCode != "ANON-001" {
		t.Fatalf("created task should be pending with anonymous code, got %#v", task)
	}

	task, err = store.AssignTask(context.Background(), tenantID, task.ID, "manager-1", AssignTaskInput{AssignedTo: "reviewer-1"})
	if err != nil {
		t.Fatalf("assign task: %v", err)
	}
	if task.Status != "assigned" || task.AssignedTo != "reviewer-1" {
		t.Fatalf("assigned task mismatch: %#v", task)
	}

	submission, err := store.SubmitGrade(context.Background(), tenantID, task.ID, "reviewer-1", SubmitGradeInput{
		Score:            4,
		RubricSelections: []RubricSelection{{PointID: "p1", Score: 4}},
		Comments:         "clear answer",
		StudentFeedback:  "Good work.",
		Reason:           "manual review completed",
	})
	if err != nil {
		t.Fatalf("submit grade: %v", err)
	}
	if submission.Task.Status != "submitted" {
		t.Fatalf("submitted task should be submitted, got %#v", submission.Task)
	}
	if submission.Grade.Score != 4 || submission.Grade.MaxScore != 5 || submission.Grade.ReviewerID != "reviewer-1" {
		t.Fatalf("human grade mismatch: %#v", submission.Grade)
	}

	returned, err := store.ReturnTask(context.Background(), tenantID, task.ID, "manager-1", ReturnTaskInput{Reason: "needs second look"})
	if err != nil {
		t.Fatalf("return task: %v", err)
	}
	if returned.Status != "returned" || returned.ReturnReason != "needs second look" {
		t.Fatalf("returned task mismatch: %#v", returned)
	}
}

func TestReviewerCanOnlySubmitAssignedTask(t *testing.T) {
	store := NewMemoryStore()
	store.AddContext(tenantID, "segment-1", reviewContext())
	task, err := store.CreateTask(context.Background(), tenantID, "manager-1", CreateTaskInput{AnswerSegmentID: "segment-1", Source: "ai_low_confidence", AssignedTo: "reviewer-1"})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}

	_, err = store.SubmitGrade(context.Background(), tenantID, task.ID, "reviewer-2", SubmitGradeInput{Score: 3, RubricSelections: []RubricSelection{{PointID: "p1", Score: 3}}})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected ErrForbidden for unassigned reviewer, got %v", err)
	}
}

func TestBatchAssignTasks(t *testing.T) {
	store := NewMemoryStore()
	store.AddContext(tenantID, "segment-1", reviewContext())
	store.AddContext(tenantID, "segment-2", reviewContext())
	first, err := store.CreateTask(context.Background(), tenantID, "manager-1", CreateTaskInput{AnswerSegmentID: "segment-1", Source: "manual_sample"})
	if err != nil {
		t.Fatalf("create first task: %v", err)
	}
	second, err := store.CreateTask(context.Background(), tenantID, "manager-1", CreateTaskInput{AnswerSegmentID: "segment-2", Source: "score_anomaly"})
	if err != nil {
		t.Fatalf("create second task: %v", err)
	}

	tasks, err := store.BatchAssignTasks(context.Background(), tenantID, "manager-1", BatchAssignInput{
		TaskIDs:    []string{first.ID, second.ID},
		AssignedTo: "reviewer-1",
	})
	if err != nil {
		t.Fatalf("batch assign: %v", err)
	}
	if len(tasks) != 2 {
		t.Fatalf("expected two assigned tasks, got %#v", tasks)
	}
	for _, task := range tasks {
		if task.Status != "assigned" || task.AssignedTo != "reviewer-1" {
			t.Fatalf("batch assigned task mismatch: %#v", task)
		}
	}
}

func TestSubmitGradeValidatesScoreAndRubricSelections(t *testing.T) {
	store := NewMemoryStore()
	store.AddContext(tenantID, "segment-1", reviewContext())
	task, err := store.CreateTask(context.Background(), tenantID, "manager-1", CreateTaskInput{AnswerSegmentID: "segment-1", Source: "manual_sample", AssignedTo: "reviewer-1"})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}

	_, err = store.SubmitGrade(context.Background(), tenantID, task.ID, "reviewer-1", SubmitGradeInput{Score: 6, RubricSelections: []RubricSelection{{PointID: "p1", Score: 5}}})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected invalid score, got %v", err)
	}

	_, err = store.SubmitGrade(context.Background(), tenantID, task.ID, "reviewer-1", SubmitGradeInput{Score: 4, RubricSelections: []RubricSelection{{PointID: "missing", Score: 4}}})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected invalid rubric selection, got %v", err)
	}
}

func TestCreateTaskRejectsInvalidSource(t *testing.T) {
	store := NewMemoryStore()
	store.AddContext(tenantID, "segment-1", reviewContext())

	_, err := store.CreateTask(context.Background(), tenantID, "manager-1", CreateTaskInput{AnswerSegmentID: "segment-1", Source: "made_up_source"})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected invalid source, got %v", err)
	}
}

func TestDoubleMarkAutoFinalizesByResolutionStrategy(t *testing.T) {
	tests := []struct {
		strategy string
		want     float64
	}{
		{strategy: "average", want: 4.5},
		{strategy: "first", want: 4},
		{strategy: "second", want: 5},
		{strategy: "higher", want: 5},
		{strategy: "lower", want: 4},
	}
	for _, tt := range tests {
		t.Run(tt.strategy, func(t *testing.T) {
			store := NewMemoryStore()
			store.AddContext(tenantID, "segment-1", reviewContext())
			if _, err := store.SetExamDoubleMarkPolicy(context.Background(), tenantID, "exam-1", "manager-1", SetDoubleMarkPolicyInput{
				Enabled:            true,
				Threshold:          2,
				ResolutionStrategy: tt.strategy,
			}); err != nil {
				t.Fatalf("set policy: %v", err)
			}
			session, err := store.CreateDoubleMarkSession(context.Background(), tenantID, "manager-1", CreateDoubleMarkSessionInput{
				AnswerSegmentID:  "segment-1",
				FirstReviewerID:  "reviewer-1",
				SecondReviewerID: "reviewer-2",
			})
			if err != nil {
				t.Fatalf("create double mark session: %v", err)
			}
			if session.FirstReviewTaskID == session.SecondReviewTaskID {
				t.Fatalf("double mark tasks must be independent: %#v", session)
			}

			first, err := store.SubmitGrade(context.Background(), tenantID, session.FirstReviewTaskID, "reviewer-1", SubmitGradeInput{
				Score:            4,
				RubricSelections: []RubricSelection{{PointID: "p1", Score: 4}},
			})
			if err != nil {
				t.Fatalf("submit first mark: %v", err)
			}
			if first.FinalGrade != nil || first.ArbitrationTask != nil || first.DoubleMarkSession == nil || first.DoubleMarkSession.Status != "first_submitted" {
				t.Fatalf("first mark should only update session state, got %#v", first)
			}

			second, err := store.SubmitGrade(context.Background(), tenantID, session.SecondReviewTaskID, "reviewer-2", SubmitGradeInput{
				Score:            5,
				RubricSelections: []RubricSelection{{PointID: "p1", Score: 5}},
			})
			if err != nil {
				t.Fatalf("submit second mark: %v", err)
			}
			if second.FinalGrade == nil || second.FinalGrade.Score != tt.want {
				t.Fatalf("expected final score %.2f, got %#v", tt.want, second.FinalGrade)
			}
			if second.DoubleMarkSession == nil || second.DoubleMarkSession.Status != "auto_finalized" {
				t.Fatalf("session should auto finalize, got %#v", second.DoubleMarkSession)
			}
			firstTask, err := store.GetTask(context.Background(), tenantID, session.FirstReviewTaskID)
			if err != nil {
				t.Fatalf("get first task: %v", err)
			}
			if firstTask.Status != "completed" || firstTask.GradeRound != "first_mark" {
				t.Fatalf("first task should be completed first_mark, got %#v", firstTask)
			}
		})
	}
}

func TestDoubleMarkCreatesArbitrationAndBlocksSameArbitratorByDefault(t *testing.T) {
	store := NewMemoryStore()
	store.AddContext(tenantID, "segment-1", reviewContext())
	if _, err := store.SetExamDoubleMarkPolicy(context.Background(), tenantID, "exam-1", "manager-1", SetDoubleMarkPolicyInput{
		Enabled:            true,
		Threshold:          1,
		ResolutionStrategy: "average",
	}); err != nil {
		t.Fatalf("set policy: %v", err)
	}
	session, err := store.CreateDoubleMarkSession(context.Background(), tenantID, "manager-1", CreateDoubleMarkSessionInput{
		AnswerSegmentID:  "segment-1",
		FirstReviewerID:  "reviewer-1",
		SecondReviewerID: "reviewer-2",
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if _, err := store.SubmitGrade(context.Background(), tenantID, session.FirstReviewTaskID, "reviewer-1", SubmitGradeInput{
		Score:            5,
		RubricSelections: []RubricSelection{{PointID: "p1", Score: 5}},
	}); err != nil {
		t.Fatalf("submit first: %v", err)
	}
	result, err := store.SubmitGrade(context.Background(), tenantID, session.SecondReviewTaskID, "reviewer-2", SubmitGradeInput{
		Score:            2,
		RubricSelections: []RubricSelection{{PointID: "p1", Score: 2}},
	})
	if err != nil {
		t.Fatalf("submit second: %v", err)
	}
	if result.ArbitrationTask == nil || result.FinalGrade != nil {
		t.Fatalf("expected arbitration task without final grade, got %#v", result)
	}
	if result.ArbitrationTask.ScoreDifference != 3 || result.DoubleMarkSession.Status != "needs_arbitration" {
		t.Fatalf("arbitration mismatch: %#v", result.ArbitrationTask)
	}

	_, err = store.AssignArbitrationTask(context.Background(), tenantID, result.ArbitrationTask.ID, "manager-1", AssignArbitrationTaskInput{AssignedTo: "reviewer-1"})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected first reviewer to be blocked as arbitrator, got %v", err)
	}
	assigned, err := store.AssignArbitrationTask(context.Background(), tenantID, result.ArbitrationTask.ID, "manager-1", AssignArbitrationTaskInput{AssignedTo: "arbitrator-1"})
	if err != nil {
		t.Fatalf("assign arbitrator: %v", err)
	}
	if assigned.Status != "assigned" || assigned.AssignedTo != "arbitrator-1" {
		t.Fatalf("assigned arbitration mismatch: %#v", assigned)
	}
	submitted, finalGrade, err := store.SubmitArbitration(context.Background(), tenantID, result.ArbitrationTask.ID, "arbitrator-1", SubmitArbitrationInput{
		FinalScore:      4,
		Reason:          "accepted stronger rubric evidence",
		StudentFeedback: "Final score set after arbitration.",
	})
	if err != nil {
		t.Fatalf("submit arbitration: %v", err)
	}
	if submitted.Status != "submitted" || submitted.FinalScore == nil || *submitted.FinalScore != 4 {
		t.Fatalf("submitted arbitration mismatch: %#v", submitted)
	}
	if finalGrade.Score != 4 || finalGrade.Source != "arbitration" {
		t.Fatalf("final grade mismatch: %#v", finalGrade)
	}
}

func TestQuestionDoubleMarkPolicyOverridesExamPolicy(t *testing.T) {
	store := NewMemoryStore()
	store.AddContext(tenantID, "segment-1", reviewContext())
	if _, err := store.SetExamDoubleMarkPolicy(context.Background(), tenantID, "exam-1", "manager-1", SetDoubleMarkPolicyInput{
		Enabled:            true,
		Threshold:          10,
		ResolutionStrategy: "average",
	}); err != nil {
		t.Fatalf("set exam policy: %v", err)
	}
	if _, err := store.SetQuestionDoubleMarkPolicy(context.Background(), tenantID, "question-1", "manager-1", SetDoubleMarkPolicyInput{
		Enabled:            true,
		Threshold:          1,
		ResolutionStrategy: "higher",
	}); err != nil {
		t.Fatalf("set question policy: %v", err)
	}
	session, err := store.CreateDoubleMarkSession(context.Background(), tenantID, "manager-1", CreateDoubleMarkSessionInput{
		AnswerSegmentID:  "segment-1",
		FirstReviewerID:  "reviewer-1",
		SecondReviewerID: "reviewer-2",
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if session.Threshold != 1 || session.ResolutionStrategy != "higher" {
		t.Fatalf("question policy should override exam policy, got %#v", session)
	}

	if _, err := store.SetQuestionDoubleMarkPolicy(context.Background(), tenantID, "question-1", "manager-1", SetDoubleMarkPolicyInput{
		Enabled:            false,
		Threshold:          1,
		ResolutionStrategy: "higher",
	}); err != nil {
		t.Fatalf("disable question policy: %v", err)
	}
	_, err = store.CreateDoubleMarkSession(context.Background(), tenantID, "manager-1", CreateDoubleMarkSessionInput{
		AnswerSegmentID:  "segment-1",
		FirstReviewerID:  "reviewer-1",
		SecondReviewerID: "reviewer-2",
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("disabled question policy should override enabled exam policy, got %v", err)
	}
}

func TestAllowSameArbitratorPolicyAllowsReviewerAsArbitrator(t *testing.T) {
	store := NewMemoryStore()
	store.AddContext(tenantID, "segment-1", reviewContext())
	if _, err := store.SetExamDoubleMarkPolicy(context.Background(), tenantID, "exam-1", "manager-1", SetDoubleMarkPolicyInput{
		Enabled:             true,
		Threshold:           1,
		ResolutionStrategy:  "average",
		AllowSameArbitrator: true,
	}); err != nil {
		t.Fatalf("set policy: %v", err)
	}
	session, err := store.CreateDoubleMarkSession(context.Background(), tenantID, "manager-1", CreateDoubleMarkSessionInput{
		AnswerSegmentID:  "segment-1",
		FirstReviewerID:  "reviewer-1",
		SecondReviewerID: "reviewer-2",
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if _, err := store.SubmitGrade(context.Background(), tenantID, session.FirstReviewTaskID, "reviewer-1", SubmitGradeInput{Score: 5, RubricSelections: []RubricSelection{{PointID: "p1", Score: 5}}}); err != nil {
		t.Fatalf("submit first: %v", err)
	}
	result, err := store.SubmitGrade(context.Background(), tenantID, session.SecondReviewTaskID, "reviewer-2", SubmitGradeInput{Score: 2, RubricSelections: []RubricSelection{{PointID: "p1", Score: 2}}})
	if err != nil {
		t.Fatalf("submit second: %v", err)
	}
	assigned, err := store.AssignArbitrationTask(context.Background(), tenantID, result.ArbitrationTask.ID, "manager-1", AssignArbitrationTaskInput{AssignedTo: "reviewer-1"})
	if err != nil {
		t.Fatalf("same arbitrator should be allowed by policy: %v", err)
	}
	if !assigned.AllowSameArbitrator || assigned.AssignedTo != "reviewer-1" {
		t.Fatalf("expected same-reviewer arbitrator assignment, got %#v", assigned)
	}
}

func reviewContext() Context {
	return Context{
		ExamID:          "exam-1",
		SubmissionID:    "submission-1",
		AnswerSegmentID: "segment-1",
		AnonymousCode:   "ANON-001",
		Question: paper.Question{
			ID:           "question-1",
			TenantID:     tenantID,
			ExamID:       "exam-1",
			QuestionNo:   "Q1",
			QuestionType: "short_answer",
			Score:        5,
		},
		Rubric: paper.Rubric{
			ID:         "rubric-1",
			QuestionID: "question-1",
			Version:    "v1",
			Status:     "approved",
			MaxScore:   5,
			Points: []paper.RubricPoint{
				{ID: "p1", Description: "main idea", Score: 5, Required: true},
			},
		},
		RawAnswer: "student wrote the main idea",
		OCRText:   "student wrote the main idea",
		AISuggestion: map[string]any{
			"suggested_score": 4,
			"mock":            true,
		},
	}
}
