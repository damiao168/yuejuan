package review

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"edugrade-enterprise/services/api-gateway/internal/assessment"
	"edugrade-enterprise/services/api-gateway/internal/auth"
)

type rejectingQualificationGate struct{}

func (rejectingQualificationGate) RequireQualification(context.Context, string, string, string, string) error {
	return errors.New("qualification required")
}

func TestClaimNextR3TaskReleasesClaimWhenQualificationMissing(t *testing.T) {
	store := NewMemoryStore()
	ctx := reviewContext()
	ctx.AssessmentSnapshot = taskContextSnapshot(assessment.RiskR3, assessment.ScoringHumanPrimary)
	store.AddContext(tenantID, "segment-1", ctx)
	task, err := store.CreateTask(context.Background(), tenantID, "manager-1", CreateTaskInput{
		AnswerSegmentID: "segment-1", Source: "manual_sample", AssignedTo: "reviewer-1",
	})
	if err != nil {
		t.Fatalf("create assigned R3 task: %v", err)
	}

	handler := NewHandler(store, nil, nil, nil).WithQualificationGate(rejectingQualificationGate{})
	request := httptest.NewRequest("POST", "/api/v1/review-tasks/next", strings.NewReader(`{}`))
	request = request.WithContext(auth.WithUser(request.Context(), auth.User{
		ID: "reviewer-1", TenantID: tenantID, Roles: []string{"grader"}, Permissions: []string{"review:work"},
	}))
	response := httptest.NewRecorder()
	handler.ClaimNextTask(response, request)
	if response.Code != 403 || !strings.Contains(response.Body.String(), "grader_qualification_required") {
		t.Fatalf("missing qualification should be rejected, status=%d body=%s", response.Code, response.Body.String())
	}
	unchanged, err := store.GetTask(context.Background(), tenantID, task.ID)
	if err != nil {
		t.Fatalf("reload task: %v", err)
	}
	if unchanged.Status != "assigned" || unchanged.AssignedTo != "reviewer-1" {
		t.Fatalf("rejected claim must preserve assignment without a live claim: %#v", unchanged)
	}
}
