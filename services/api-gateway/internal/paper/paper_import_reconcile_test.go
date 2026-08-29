package paper

import (
	"context"
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
	for _, input := range questions {
		input.ExamPaperID = p.ID
		if _, err := store.CreateQuestion(ctx, "tenant", "exam", "user", input); err != nil {
			t.Fatal(err)
		}
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

func TestPaperImportReconcilesExistingQuestion(t *testing.T) {
	store, job := newReconcileFixture(t, []CreateQuestionInput{{QuestionNo: "Q1", QuestionType: "short_answer", Score: 10, SortOrder: 1}})
	preview, err := store.CompletePaperImport(context.Background(), "tenant", job.ID, []PaperImportDraftQuestion{importedDraft("Q1", "short_answer", 10)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if preview.Questions[0].MatchStatus != "matched" || preview.Questions[0].MatchedQuestionID == "" {
		t.Fatalf("unexpected reconcile preview: %#v", preview)
	}
	if _, err = store.ApplyPaperImport(context.Background(), "tenant", job.ID, "user"); err != nil {
		t.Fatal(err)
	}
	questions, _ := store.ListQuestions(context.Background(), "tenant", "exam")
	if len(questions) != 1 || questions[0].Stem != "正式题干 Q1" || questions[0].AnswerKey == nil || questions[0].Rubric == nil {
		t.Fatalf("import did not enrich existing question: %#v", questions)
	}
}

func TestPaperImportReportsScoreAndTypeMismatchWithoutOverwritingBlueprint(t *testing.T) {
	for _, tc := range []struct {
		name, kind string
		score      float64
		issue      string
	}{
		{"score", "short_answer", 8, "分值不一致"}, {"type", "essay", 10, "题型不一致"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store, job := newReconcileFixture(t, []CreateQuestionInput{{QuestionNo: "Q1", QuestionType: "short_answer", Score: 10, SortOrder: 1}})
			preview, err := store.CompletePaperImport(context.Background(), "tenant", job.ID, []PaperImportDraftQuestion{importedDraft("Q1", tc.kind, tc.score)}, nil)
			if err != nil {
				t.Fatal(err)
			}
			if preview.Questions[0].MatchStatus != "mismatch" || !strings.Contains(strings.Join(preview.Questions[0].Issues, " "), tc.issue) {
				t.Fatalf("mismatch was not explicit: %#v", preview)
			}
			if _, err = store.ApplyPaperImport(context.Background(), "tenant", job.ID, "user"); err != nil {
				t.Fatal(err)
			}
			questions, _ := store.ListQuestions(context.Background(), "tenant", "exam")
			if questions[0].Score != 10 || questions[0].QuestionType != "short_answer" {
				t.Fatalf("blueprint core was overwritten: %#v", questions[0])
			}
		})
	}
}

func TestPaperImportReportsMissingAndExtraQuestions(t *testing.T) {
	store, job := newReconcileFixture(t, []CreateQuestionInput{
		{QuestionNo: "Q1", QuestionType: "short_answer", Score: 10, SortOrder: 1},
		{QuestionNo: "Q2", QuestionType: "short_answer", Score: 10, SortOrder: 2},
	})
	preview, err := store.CompletePaperImport(context.Background(), "tenant", job.ID, []PaperImportDraftQuestion{importedDraft("Q1", "short_answer", 10)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(preview.Issues, " "), "漏识别蓝图题目 Q2") {
		t.Fatalf("missing question issue absent: %#v", preview.Issues)
	}

	store, job = newReconcileFixture(t, []CreateQuestionInput{{QuestionNo: "Q1", QuestionType: "short_answer", Score: 10, SortOrder: 1}})
	preview, err = store.CompletePaperImport(context.Background(), "tenant", job.ID, []PaperImportDraftQuestion{importedDraft("Q1", "short_answer", 10), importedDraft("Q2", "short_answer", 10)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(preview.Issues, " "), "AI 多识别题目 Q2") || preview.Questions[1].MatchStatus != "extra" {
		t.Fatalf("extra question issue absent: %#v", preview)
	}
}
