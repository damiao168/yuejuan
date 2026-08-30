package paper

import (
	"context"
	"errors"
	"testing"
)

func candidateImport(t *testing.T) (*MemoryStore, PaperImportJob) {
	t.Helper()
	store := NewMemoryStore()
	job, err := store.CreatePaperImport(context.Background(), "tenant", "exam", "user", CreatePaperImportInput{Subject: "mathematics", Sources: []CreatePaperImportSourceInput{{FileAssetID: "asset-1", DocumentIndex: 0, RoleHint: "auto"}}})
	if err != nil {
		t.Fatal(err)
	}
	return store, job
}

func issueCodes(job PaperImportJob) map[string]bool {
	out := map[string]bool{}
	for _, issue := range job.StructuredIssues {
		out[issue.Code] = true
	}
	return out
}

func TestAnswerOnlyImportRemainsReviewableWithoutInventingQuestions(t *testing.T) {
	store, job := candidateImport(t)
	out, err := store.CompletePaperImportCandidates(context.Background(), "tenant", job.ID, nil, nil, []AnswerCandidate{{CandidateID: "a1", QuestionNoHint: "1", StandardAnswer: "A", Confidence: .95}}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if out.Status != "review_required" || len(out.Questions) != 0 || len(out.AnswerCandidates) != 1 {
		t.Fatalf("unexpected answer-only result: %#v", out)
	}
	if !issueCodes(out)["UNMATCHED_ANSWER"] || !paperImportHasBlockingIssues(out) {
		t.Fatalf("answer-only result must remain reviewable but not applicable: %#v", out.StructuredIssues)
	}
	if !issueCodes(out)["UNMATCHED_ANSWER"] || !issueCodes(out)["POSSIBLE_MISSING_QUESTION"] {
		t.Fatalf("missing answer-only issues: %#v", out.StructuredIssues)
	}
}

func TestQuestionOnlyImportReportsMissingAnswerAndMissingScore(t *testing.T) {
	store, job := candidateImport(t)
	out, err := store.CompletePaperImportCandidates(context.Background(), "tenant", job.ID, nil, []QuestionCandidate{{CandidateID: "q1", QuestionNoRaw: "第1题", QuestionType: "short_answer", Stem: "题干", Confidence: .9}}, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if out.Status != "review_required" || len(out.QuestionCandidates) != 1 {
		t.Fatalf("unexpected question-only result: %#v", out)
	}
	codes := issueCodes(out)
	if !codes["MISSING_ANSWER"] || !codes["MISSING_SCORE"] {
		t.Fatalf("missing completeness issues: %#v", out.StructuredIssues)
	}
}

func TestCandidateReconciliationDetectsGapAndConflictingAnswers(t *testing.T) {
	store, job := candidateImport(t)
	score := 3.0
	questions := []QuestionCandidate{{CandidateID: "q1", QuestionNoRaw: "01", QuestionType: "single_choice", Score: &score, Confidence: .9}, {CandidateID: "q3", QuestionNoRaw: "第3题", QuestionType: "single_choice", Score: &score, Confidence: .9}}
	answers := []AnswerCandidate{{CandidateID: "a1", QuestionNoHint: "1", StandardAnswer: "A", Confidence: .9}, {CandidateID: "a2", QuestionNoHint: "1.", StandardAnswer: "C", Confidence: .9}}
	out, err := store.CompletePaperImportCandidates(context.Background(), "tenant", job.ID, nil, questions, answers, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	codes := issueCodes(out)
	if !codes["CONFLICTING_ANSWERS"] || !codes["POSSIBLE_MISSING_QUESTION"] {
		t.Fatalf("expected conflict and gap: %#v", out.StructuredIssues)
	}
}

func TestCandidateReconciliationKeepsQuestionAnswerAndSolutionIndependent(t *testing.T) {
	score := 5.0
	question := QuestionCandidate{CandidateID: "q1", QuestionNoRaw: "1", QuestionType: "single_choice", Score: &score, Stem: "题干", Confidence: .9}
	answer := AnswerCandidate{CandidateID: "a1", QuestionNoHint: "1", StandardAnswer: "B", Confidence: .9}
	solution := SolutionCandidate{CandidateID: "s1", QuestionNoHint: "1", RawText: "排除 A、C、D", Steps: []SolutionStep{{StepNo: 1, Content: "比较选项"}}, Confidence: .9}
	tests := []struct {
		name                  string
		questions             []QuestionCandidate
		answers               []AnswerCandidate
		solutions             []SolutionCandidate
		wantDrafts            int
		wantAnswer, wantSoln  bool
		wantUnmatchedAnswer   bool
		wantUnmatchedSolution bool
	}{
		{"question and answer", []QuestionCandidate{question}, []AnswerCandidate{answer}, nil, 1, true, false, false, false},
		{"question and solution", []QuestionCandidate{question}, nil, []SolutionCandidate{solution}, 1, false, true, false, false},
		{"answer and solution", nil, []AnswerCandidate{answer}, []SolutionCandidate{solution}, 0, false, false, true, true},
		{"fully mixed", []QuestionCandidate{question}, []AnswerCandidate{answer}, []SolutionCandidate{solution}, 1, true, true, false, false},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			drafts, issues := reconcilePaperImportCandidates(testCase.questions, testCase.answers, testCase.solutions, nil)
			if len(drafts) != testCase.wantDrafts {
				t.Fatalf("draft count = %d, want %d", len(drafts), testCase.wantDrafts)
			}
			if len(drafts) > 0 && ((drafts[0].AnswerKey != nil) != testCase.wantAnswer || (drafts[0].Solution != nil) != testCase.wantSoln) {
				t.Fatalf("independent fields were conflated: %#v", drafts[0])
			}
			codes := map[string]bool{}
			for _, issue := range issues {
				codes[issue.Code] = true
			}
			if codes["UNMATCHED_ANSWER"] != testCase.wantUnmatchedAnswer || codes["UNMATCHED_SOLUTION"] != testCase.wantUnmatchedSolution {
				t.Fatalf("unmatched issues = %#v", issues)
			}
		})
	}
}

func TestQuestionNumberNormalizerPreservesParentChildStructure(t *testing.T) {
	if got := normalizePaperImportQuestionNumber("18（1）"); got != "18(1)" {
		t.Fatalf("got %q", got)
	}
	if got := normalizePaperImportQuestionNumber("01."); got != "1" {
		t.Fatalf("got %q", got)
	}
	if got := normalizePaperImportQuestionNumber("1）"); got != "1" {
		t.Fatalf("got %q", got)
	}
	if got := normalizePaperImportQuestionNumber("二十三"); got != "23" {
		t.Fatalf("got %q", got)
	}
	if got := normalizePaperImportQuestionNumber("1))"); got == "1" {
		t.Fatalf("malformed number was silently normalized: %q", got)
	}
}

func TestSolutionOnlyImportRemainsReviewableWithoutInventingQuestion(t *testing.T) {
	store, job := candidateImport(t)
	out, err := store.CompletePaperImportCandidates(context.Background(), "tenant", job.ID, nil, nil, nil, []SolutionCandidate{{CandidateID: "s1", QuestionNoHint: "18(1)", RawText: "因为，所以", Confidence: .9}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if out.Status != "review_required" || len(out.Questions) != 0 || len(out.SolutionCandidates) != 1 || !issueCodes(out)["UNMATCHED_SOLUTION"] {
		t.Fatalf("unexpected solution-only result: %#v", out)
	}
	if !paperImportHasBlockingIssues(out) {
		t.Fatal("solution-only result was incorrectly applicable")
	}
}

func TestObjectiveCandidateGetsDeterministicRubricButEssayDoesNotInventAnswer(t *testing.T) {
	score := 3.0
	drafts, issues := reconcilePaperImportCandidates(
		[]QuestionCandidate{{CandidateID: "q1", QuestionNoRaw: "1", QuestionType: "single_choice", Score: &score, Stem: "选择", Confidence: .9}},
		[]AnswerCandidate{{CandidateID: "a1", QuestionNoHint: "1", StandardAnswer: "A", Confidence: .9}}, nil, nil,
	)
	if len(drafts) != 1 || drafts[0].Rubric == nil || !scoreEqual(drafts[0].Rubric.MaxScore, score) {
		t.Fatalf("objective rubric was not generated: %#v", drafts)
	}
	if issueSet := func() map[string]bool {
		out := map[string]bool{}
		for _, issue := range issues {
			out[issue.Code] = true
		}
		return out
	}(); issueSet["MISSING_ANSWER"] {
		t.Fatalf("unexpected missing answer: %#v", issues)
	}

	essayScore := 20.0
	_, essayIssues := reconcilePaperImportCandidates([]QuestionCandidate{{CandidateID: "qe", QuestionNoRaw: "2", QuestionType: "essay", Score: &essayScore, Stem: "作文", Confidence: .9}}, nil, nil, nil)
	codes := map[string]bool{}
	for _, issue := range essayIssues {
		codes[issue.Code] = true
	}
	if codes["MISSING_ANSWER"] || !codes["MISSING_RUBRIC"] {
		t.Fatalf("essay requirements were wrong: %#v", essayIssues)
	}
}

func TestReviewedDraftRequirementsFollowAssessmentArchetype(t *testing.T) {
	draft := PaperImportDraftQuestion{
		QuestionNo: "1", QuestionType: "short_answer", AssessmentArchetype: "extended_response", Score: 10, Stem: "开放写作",
		Rubric: &RubricInput{Status: "draft", MaxScore: 10, Points: []RubricPoint{{ID: "p1", Description: "内容", Score: 10}}},
	}
	issues := appendReviewedDraftIssues(nil, []PaperImportDraftQuestion{draft})
	for _, issue := range issues {
		if issue.Code == "MISSING_ANSWER" {
			t.Fatalf("extended_response archetype incorrectly required a unique answer: %#v", issues)
		}
	}
}

func TestCandidateImportRequiresExplicitHumanReviewBeforeApply(t *testing.T) {
	store, job := candidateImport(t)
	score := 3.0
	processed, err := store.CompletePaperImportCandidates(context.Background(), "tenant", job.ID, nil,
		[]QuestionCandidate{{CandidateID: "q1", QuestionNoRaw: "1", QuestionType: "single_choice", Score: &score, Stem: "选择正确答案", Confidence: .95}},
		[]AnswerCandidate{{CandidateID: "a1", QuestionNoHint: "1", StandardAnswer: "A", Confidence: .95}}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !issueCodes(processed)["HUMAN_REVIEW_REQUIRED"] {
		t.Fatalf("candidate result did not expose review gate: %#v", processed.StructuredIssues)
	}
	if _, err = store.ApplyPaperImport(context.Background(), "tenant", job.ID, "user"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("unreviewed candidate apply error = %v, want ErrInvalidInput", err)
	}
	reviewed, err := store.SavePaperImportReview(context.Background(), "tenant", job.ID, "user", ReviewPaperImportInput{Questions: processed.Questions})
	if err != nil {
		t.Fatal(err)
	}
	if issueCodes(reviewed)["HUMAN_REVIEW_REQUIRED"] {
		t.Fatalf("explicit review did not clear gate: %#v", reviewed.StructuredIssues)
	}
	if reviewed.Questions[0].CompletenessStatus != "complete" {
		t.Fatalf("reviewed draft completeness = %q", reviewed.Questions[0].CompletenessStatus)
	}
	if _, err = store.ApplyPaperImport(context.Background(), "tenant", job.ID, "user"); err != nil {
		t.Fatalf("reviewed candidate did not apply: %v", err)
	}
	questions, err := store.ListQuestions(context.Background(), "tenant", "exam")
	if err != nil || len(questions) != 1 {
		t.Fatalf("list applied questions: count=%d err=%v", len(questions), err)
	}
	if questions[0].PaperImportID != job.ID || questions[0].PaperImportCandidateID != "q1" || questions[0].AnswerKey == nil || questions[0].AnswerKey.PaperImportCandidateID != "a1" {
		t.Fatalf("applied provenance was not retained: %#v", questions[0])
	}
}

func TestHumanReviewCannotClearARequiredFieldAndBypassApplyGate(t *testing.T) {
	store, job := candidateImport(t)
	score := 3.0
	processed, err := store.CompletePaperImportCandidates(context.Background(), "tenant", job.ID, nil,
		[]QuestionCandidate{{CandidateID: "q1", QuestionNoRaw: "1", QuestionType: "single_choice", Score: &score, Stem: "选择", Confidence: .9}},
		[]AnswerCandidate{{CandidateID: "a1", QuestionNoHint: "1", StandardAnswer: "A", Confidence: .9}}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	processed.Questions[0].AnswerKey.StandardAnswer = ""
	reviewed, err := store.SavePaperImportReview(context.Background(), "tenant", job.ID, "user", ReviewPaperImportInput{Questions: processed.Questions})
	if err != nil {
		t.Fatal(err)
	}
	if !issueCodes(reviewed)["MISSING_ANSWER"] {
		t.Fatalf("review did not recompute missing answer: %#v", reviewed.StructuredIssues)
	}
	if _, err = store.ApplyPaperImport(context.Background(), "tenant", job.ID, "user"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid reviewed draft apply error = %v, want ErrInvalidInput", err)
	}
}

func TestReplacePaperImportSourcesReordersRemovesAndChangesRole(t *testing.T) {
	store := NewMemoryStore()
	job, err := store.CreatePaperImport(context.Background(), "tenant", "exam", "user", CreatePaperImportInput{Subject: "math", Sources: []CreatePaperImportSourceInput{{FileAssetID: "a", DocumentIndex: 0, RoleHint: "auto"}, {FileAssetID: "b", DocumentIndex: 1, RoleHint: "auto"}, {FileAssetID: "c", DocumentIndex: 2, RoleHint: "auto"}}})
	if err != nil {
		t.Fatal(err)
	}
	job, err = store.CompletePaperImportCandidates(context.Background(), "tenant", job.ID, nil, nil, []AnswerCandidate{{CandidateID: "a1", QuestionNoHint: "1", StandardAnswer: "A"}}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	out, err := store.ReplacePaperImportSources(context.Background(), "tenant", job.ID, "user", ReplacePaperImportSourcesInput{Sources: []ReplacePaperImportSourceInput{{ID: job.Sources[2].ID, DocumentIndex: 0, RoleHint: "solution"}, {ID: job.Sources[0].ID, DocumentIndex: 1, RoleHint: "auto"}}})
	if err != nil {
		t.Fatal(err)
	}
	if out.Status != "processing" || len(out.Sources) != 2 || out.Sources[0].FileAssetID != "c" || out.Sources[0].RoleHint != "solution" || out.Sources[1].DocumentIndex != 1 {
		t.Fatalf("unexpected replacement: %#v", out.Sources)
	}
}

func TestLegacyTwoFileImportCreatesOrderedTypedSources(t *testing.T) {
	store := NewMemoryStore()
	job, err := store.CreatePaperImport(context.Background(), "tenant", "exam", "user", CreatePaperImportInput{Subject: "math", PaperFileAssetID: "paper", AnswerFileAssetID: "answer"})
	if err != nil {
		t.Fatal(err)
	}
	if len(job.Sources) != 2 || job.Sources[0].FileAssetID != "paper" || job.Sources[0].RoleHint != "question" || job.Sources[1].FileAssetID != "answer" || job.Sources[1].RoleHint != "answer" {
		t.Fatalf("legacy source compatibility failed: %#v", job.Sources)
	}
	if _, err := store.GetPaperImport(context.Background(), "other-tenant", job.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-tenant import read error = %v", err)
	}
}

func TestCandidateRerunIsIdempotent(t *testing.T) {
	store, job := candidateImport(t)
	score := 2.0
	questions := []QuestionCandidate{{CandidateID: "q1", QuestionNoRaw: "1", QuestionType: "single_choice", Score: &score, Stem: "题干", Confidence: .9}}
	answers := []AnswerCandidate{{CandidateID: "a1", QuestionNoHint: "1", StandardAnswer: "A", Confidence: .9}}
	first, err := store.CompletePaperImportCandidates(context.Background(), "tenant", job.ID, nil, questions, answers, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.CompletePaperImportCandidates(context.Background(), "tenant", job.ID, nil, questions, answers, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Questions) != 1 || len(second.StructuredIssues) != len(first.StructuredIssues) || second.Questions[0].CandidateID != "q1" {
		t.Fatalf("rerun changed deterministic result: first=%#v second=%#v", first, second)
	}
}

func TestPaperImportSourceConfigurationHashTracksOrderAndRole(t *testing.T) {
	sources := []PaperImportSource{{ID: "a", FileAssetID: "fa", DocumentIndex: 0, RoleHint: "auto"}, {ID: "b", FileAssetID: "fb", DocumentIndex: 1, RoleHint: "answer"}}
	baseline := paperImportSourceConfigurationHash(sources)
	if baseline != paperImportSourceConfigurationHash([]PaperImportSource{sources[1], sources[0]}) {
		t.Fatal("hash depended on database row order instead of document_index")
	}
	changedRole := append([]PaperImportSource{}, sources...)
	changedRole[0].RoleHint = "question"
	changedOrder := append([]PaperImportSource{}, sources...)
	changedOrder[0].DocumentIndex, changedOrder[1].DocumentIndex = 1, 0
	if baseline == paperImportSourceConfigurationHash(changedRole) || baseline == paperImportSourceConfigurationHash(changedOrder) {
		t.Fatal("hash did not track a semantic source configuration change")
	}
}

func TestIncrementalRerunPreservesHumanConfirmedAnswer(t *testing.T) {
	store, job := candidateImport(t)
	score := 3.0
	q := []QuestionCandidate{{CandidateID: "q1", QuestionNoRaw: "1", QuestionType: "single_choice", Score: &score, Stem: "题干", Confidence: .9}}
	a := []AnswerCandidate{{CandidateID: "a1", QuestionNoHint: "1", StandardAnswer: "A", Confidence: .9}}
	solutions := []SolutionCandidate{{CandidateID: "s1", QuestionNoHint: "1", RawText: "机器解析一", Confidence: .9}}
	first, err := store.CompletePaperImportCandidates(context.Background(), "tenant", job.ID, nil, q, a, solutions, nil)
	if err != nil {
		t.Fatal(err)
	}
	first.Questions[0].AnswerKey.StandardAnswer = "C"
	first.Questions[0].Solution.RawText = "人工确认解析"
	reviewed, err := store.SavePaperImportReview(context.Background(), "tenant", job.ID, "user", ReviewPaperImportInput{Questions: first.Questions})
	if err != nil {
		t.Fatal(err)
	}
	if reviewed.Questions[0].AnswerKey.StandardAnswer != "C" {
		t.Fatal("review was not saved")
	}
	q[0].CandidateID = "q1-rerun"
	solutions[0].RawText = "机器解析二"
	rerun, err := store.CompletePaperImportCandidates(context.Background(), "tenant", job.ID, nil, q, a, solutions, nil)
	if err != nil {
		t.Fatal(err)
	}
	if rerun.Questions[0].AnswerKey.StandardAnswer != "C" {
		t.Fatalf("human answer overwritten: %#v", rerun.Questions[0].AnswerKey)
	}
	if rerun.Questions[0].Solution == nil || rerun.Questions[0].Solution.RawText != "人工确认解析" {
		t.Fatalf("human solution overwritten: %#v", rerun.Questions[0].Solution)
	}
	if !issueCodes(rerun)["CONFLICTING_ANSWERS"] {
		t.Fatalf("new machine conflict was hidden: %#v", rerun.StructuredIssues)
	}
	if !issueCodes(rerun)["HUMAN_CONFIRMED_CONFLICT"] {
		t.Fatalf("new machine solution conflict was hidden: %#v", rerun.StructuredIssues)
	}
}
