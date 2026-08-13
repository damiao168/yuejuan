package regrade

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"edugrade-enterprise/services/api-gateway/internal/auth"
)

func TestListMineReturnsReviewerSafeWorkItems(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	store.SeedPublishedRelease("tenant-a", "exam-a", "release-a", 1, true, map[string][]SourceItem{
		"question-a": {{SubmissionID: "submission-a", OldFinalGradeID: "final-a", OldScore: 1, MaxScore: 5}},
	})
	service := NewService(store)
	severity := 1.0
	summary, err := service.Create(ctx, "tenant-a", "exam-a", "question-a", "manager-a", CreateInput{
		SourceReleaseID: "release-a", ReasonCode: ReasonRubricError, ReasonText: "criterion clarification",
		Strategy: StrategyHumanRecheck, SeverityDelta: &severity, AssigneeID: "reviewer-a", IdempotencyKey: "safe-work-item-list",
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

	req := httptest.NewRequest(http.MethodGet, "/api/v1/regrade-items/mine", nil)
	req = req.WithContext(auth.WithUser(req.Context(), auth.User{ID: "reviewer-a", TenantID: "tenant-a"}))
	response := httptest.NewRecorder()
	NewHandler(service, nil).ListMine(response, req)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", response.Code, response.Body.String())
	}
	var payload struct {
		Items []map[string]any `json:"regrade_items"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Items) != 1 {
		t.Fatalf("items = %#v", payload.Items)
	}
	for _, forbidden := range []string{"old_final_grade_id", "old_score", "candidate_score", "reviewed_score"} {
		if _, found := payload.Items[0][forbidden]; found {
			t.Fatalf("reviewer queue leaked %s: %#v", forbidden, payload.Items[0])
		}
	}
	if _, found := payload.Items[0]["submission_id"]; found {
		t.Fatalf("reviewer queue leaked submission identifier: %#v", payload.Items[0])
	}
	if payload.Items[0]["max_score"] != float64(5) {
		t.Fatalf("safe work item lost score bound: %#v", payload.Items[0])
	}
}
