package paper

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func newReconcileFixture(t *testing.T, questions []CreateQuestionInput) (*MemoryStore, PaperImportJob) {
	t.Helper()
	store := NewMemoryStore()
	ctx := context.Background()
	p, err := store.CreatePaper(ctx, "tenant", "exam", "user", CreatePaperInput{FileAssetID: "paper-file"})
	if err != nil {
		t.Fatal(err)
	}
	var total float64
	for _, input := range questions {
		input.ExamPaperID = p.ID
		total += input.Score
		if _, err := store.CreateQuestion(ctx, "tenant", "exam", "user", input); err != nil {
			t.Fatal(err)
		}
	}
	if len(questions) > 0 {
		store.SetExamTotal("exam", total)
	}
	job, err := store.CreatePaperImport(ctx, "tenant", "exam", "user", CreatePaperImportInput{ExamPaperID: p.ID, PaperFileAssetID: p.FileAssetID, AnswerFileAssetID: "answer-file", Subject: "mathematics"})
	if err != nil {
		t.Fatal(err)
	}
	return store, job
}

func importedDraft(number, kind string, score float64) PaperImportDraftQuestion {
	return PaperImportDraftQuestion{
		QuestionNo: number, QuestionType: kind, Score: score, Stem: "正式题干 " + number,
		KnowledgePoints: []string{"知识点"}, Confidence: .91, Issues: []string{},
		AnswerKey: &AnswerKeyInput{StandardAnswer: "42", EquivalentAnswers: []any{"42"}, Tolerance: map[string]any{}},
		Rubric:    &RubricInput{Status: "draft", MaxScore: score, Points: []RubricPoint{{ID: "p1", Description: "正确", Score: score, Required: true}}},
	}
}

func joinedImportIssues(job PaperImportJob) string {
	parts := append([]string{}, job.Issues...)
	for _, question := range job.Questions {
		parts = append(parts, question.Issues...)
	}
	return strings.Join(parts, " ")
}

func TestNormalizePaperImportQuestionNumber(t *testing.T) {
	for input, expected := range map[string]string{
		"1": "1", "1.": "1", "1、": "1", "（1）": "1", "1(1)": "1(1)", "一": "1", "一、1": "一、1",
	} {
		if actual := normalizePaperImportQuestionNumber(input); actual != expected {
			t.Errorf("normalize %q: got %q want %q", input, actual, expected)
		}
	}
}

func TestPaperImportReconciliationDetectsQuestionSetConflicts(t *testing.T) {
	blueprint := []CreateQuestionInput{
		{QuestionNo: "1", QuestionType: "short_answer", Score: 10, SortOrder: 1},
		{QuestionNo: "2", QuestionType: "short_answer", Score: 10, SortOrder: 2},
	}
	tests := []struct {
		name   string
		drafts []PaperImportDraftQuestion
		code   string
	}{
		{"duplicate normalized number", []PaperImportDraftQuestion{importedDraft("1.", "short_answer", 10), importedDraft("1、", "short_answer", 10)}, "paper_import.duplicate_question"},
		{"missing question", []PaperImportDraftQuestion{importedDraft("1", "short_answer", 10)}, "paper_import.missing_question"},
		{"unexpected question", []PaperImportDraftQuestion{importedDraft("1", "short_answer", 10), importedDraft("2", "short_answer", 10), importedDraft("3", "short_answer", 10)}, "paper_import.unexpected_question"},
		{"answer for nonexistent question", []PaperImportDraftQuestion{importedDraft("1", "short_answer", 10), importedDraft("3", "short_answer", 10)}, "paper_import.unexpected_question"},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			store, job := newReconcileFixture(t, blueprint)
			preview, err := store.CompletePaperImport(context.Background(), "tenant", job.ID, testCase.drafts, nil)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(joinedImportIssues(preview), testCase.code) {
				t.Fatalf("missing issue %s: %#v", testCase.code, preview)
			}
		})
	}
}

func TestPaperImportReconciliationDetectsQuestionAndRubricConflicts(t *testing.T) {
	blueprint := []CreateQuestionInput{{QuestionNo: "1", QuestionType: "short_answer", Score: 10, SortOrder: 1}}
	tests := []struct {
		name   string
		mutate func(*PaperImportDraftQuestion)
		code   string
	}{
		{"question type", func(draft *PaperImportDraftQuestion) { draft.QuestionType = "essay" }, "paper_import.question_type_mismatch"},
		{"question score", func(draft *PaperImportDraftQuestion) {
			draft.Score, draft.Rubric.MaxScore, draft.Rubric.Points[0].Score = 8, 8, 8
		}, "paper_import.question_score_mismatch"},
		{"rubric max score", func(draft *PaperImportDraftQuestion) { draft.Rubric.MaxScore = 8 }, "paper_import.rubric_max_score_mismatch"},
		{"rubric point total", func(draft *PaperImportDraftQuestion) { draft.Rubric.Points[0].Score = 8 }, "paper_import.rubric_points_score_mismatch"},
		{"missing answer", func(draft *PaperImportDraftQuestion) { draft.AnswerKey = nil }, "paper_import.missing_answer"},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			store, job := newReconcileFixture(t, blueprint)
			draft := importedDraft("1", "short_answer", 10)
			testCase.mutate(&draft)
			preview, err := store.CompletePaperImport(context.Background(), "tenant", job.ID, []PaperImportDraftQuestion{draft}, nil)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(joinedImportIssues(preview), testCase.code) {
				t.Fatalf("missing issue %s: %#v", testCase.code, preview)
			}
		})
	}
}

func TestPaperImportReconciliationChecksExamTotalWithoutExistingQuestions(t *testing.T) {
	store, job := newReconcileFixture(t, nil)
	store.SetExamTotal("exam", 20)
	preview, err := store.CompletePaperImport(context.Background(), "tenant", job.ID, []PaperImportDraftQuestion{importedDraft("1", "short_answer", 10)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(joinedImportIssues(preview), "paper_import.total_score_mismatch") {
		t.Fatalf("exam total mismatch was not reported: %#v", preview)
	}
}

func TestPaperImportValidCompletePaperApplies(t *testing.T) {
	store, job := newReconcileFixture(t, []CreateQuestionInput{
		{QuestionNo: "1", QuestionType: "short_answer", Score: 10, SortOrder: 1},
		{QuestionNo: "2", QuestionType: "short_answer", Score: 10, SortOrder: 2},
	})
	preview, err := store.CompletePaperImport(context.Background(), "tenant", job.ID, []PaperImportDraftQuestion{
		importedDraft("1.", "short_answer", 10), importedDraft("2、", "short_answer", 10),
	}, nil)
	if err != nil || len(preview.Issues) != 0 {
		t.Fatalf("valid paper rejected: %#v err=%v", preview, err)
	}
	if _, err = store.ApplyPaperImport(context.Background(), "tenant", job.ID, "user"); err != nil {
		t.Fatal(err)
	}
	questions, _ := store.ListQuestions(context.Background(), "tenant", "exam")
	if len(questions) != 2 || questions[0].AnswerKey == nil || questions[1].AnswerKey == nil {
		t.Fatalf("valid import was not applied: %#v", questions)
	}
}

func TestPaperImportReconciliationIssuesBlockApply(t *testing.T) {
	store, job := newReconcileFixture(t, []CreateQuestionInput{{QuestionNo: "1", QuestionType: "short_answer", Score: 10, SortOrder: 1}})
	invalid := importedDraft("1", "essay", 10)
	if _, err := store.CompletePaperImport(context.Background(), "tenant", job.ID, []PaperImportDraftQuestion{invalid}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ApplyPaperImport(context.Background(), "tenant", job.ID, "user"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("reconciliation issue bypassed apply gate: %v", err)
	}
	questions, _ := store.ListQuestions(context.Background(), "tenant", "exam")
	if questions[0].Stem != "" || questions[0].AnswerKey != nil {
		t.Fatalf("blocked import mutated canonical question: %#v", questions[0])
	}
}

func TestPaperImportCanApplyAfterCandidateCorrection(t *testing.T) {
	store, job := newReconcileFixture(t, []CreateQuestionInput{{QuestionNo: "1", QuestionType: "short_answer", Score: 10, SortOrder: 1}})
	if _, err := store.CompletePaperImport(context.Background(), "tenant", job.ID, []PaperImportDraftQuestion{importedDraft("2", "short_answer", 10)}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ApplyPaperImport(context.Background(), "tenant", job.ID, "user"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid candidate unexpectedly applied: %v", err)
	}
	corrected, err := store.CompletePaperImport(context.Background(), "tenant", job.ID, []PaperImportDraftQuestion{importedDraft("1", "short_answer", 10)}, nil)
	if err != nil || len(corrected.Issues) != 0 {
		t.Fatalf("corrected candidate did not reconcile: %#v err=%v", corrected, err)
	}
	if _, err := store.ApplyPaperImport(context.Background(), "tenant", job.ID, "user"); err != nil {
		t.Fatalf("corrected candidate could not apply: %v", err)
	}
}
