package regrade

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"edugrade-enterprise/services/api-gateway/internal/paper"
)

func TestRegradeGraderContextIsIndependentOfSourceReleaseFacts(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	store.SeedPublishedRelease("tenant-a", "exam-a", "release-a", 1, true, map[string][]SourceItem{
		"question-a": {{SubmissionID: "submission-a", OldFinalGradeID: "old-grade-secret", OldScore: 1, MaxScore: 5}},
	})
	store.SeedGraderEvidence("submission-a", MemoryGraderEvidence{
		Question:     GraderQuestion{ID: "question-a", QuestionNo: "Q1", QuestionType: "extended_response", Score: 5, Stem: "Explain the result", KnowledgePoints: []string{"reasoning"}},
		FrozenRubric: paper.Rubric{ID: "rubric-v2", Version: "2", MaxScore: 5, Points: []paper.RubricPoint{{ID: "reason", Description: "valid reasoning", Score: 5}}},
		RawAnswer:    "student answer", SegmentStatus: "completed", AnswerSegmentID: "segment-a",
	})
	service := NewService(store).WithContextSource(store)
	summary, err := service.Create(ctx, "tenant-a", "exam-a", "question-a", "manager-a", CreateInput{
		SourceReleaseID: "release-a", ReasonCode: ReasonRubricError, ReasonText: "new rubric", Strategy: StrategyHumanRecheck,
		AssigneeID: "reviewer-a", IdempotencyKey: "regrade-grader-context",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Approve(ctx, "tenant-a", summary.Job.ID, "manager-a"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Start(ctx, "tenant-a", summary.Job.ID, "manager-a"); err != nil {
		t.Fatal(err)
	}
	claimed, err := service.Claim(ctx, "tenant-a", summary.Items[0].ID, "reviewer-a")
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := service.GetGraderContext(ctx, "tenant-a", claimed.ID, "reviewer-a")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Question.QuestionNo != "Q1" || loaded.FrozenRubric.Version != "2" || loaded.Answer.RawAnswer != "student answer" || loaded.Answer.SegmentImageURL == "" {
		t.Fatalf("context = %#v", loaded)
	}
	raw, err := json.Marshal(loaded)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"submission-a", "old-grade-secret", "old_score", "original_reviewer"} {
		if strings.Contains(string(raw), forbidden) {
			t.Fatalf("grader context leaked %q: %s", forbidden, raw)
		}
	}
	if _, err := service.GetGraderContext(ctx, "tenant-a", claimed.ID, "other-reviewer"); !errors.Is(err, ErrAssignmentForbidden) {
		t.Fatalf("other reviewer context err=%v", err)
	}
	segmentID, err := service.GetSegmentID(ctx, "tenant-a", claimed.ID, "reviewer-a")
	if err != nil || segmentID != "segment-a" {
		t.Fatalf("segment bridge = %q, %v", segmentID, err)
	}
	if _, err := service.RecordCandidate(ctx, "tenant-a", claimed.ID, "reviewer-a", CandidateInput{Score: 4, ExpectedRevision: claimed.Revision}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.GetGraderContext(ctx, "tenant-a", claimed.ID, "reviewer-a"); !errors.Is(err, ErrAssignmentForbidden) {
		t.Fatalf("completed context should not remain readable, err=%v", err)
	}
}
