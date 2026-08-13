package backmark

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"edugrade-enterprise/services/api-gateway/internal/auth"
	"edugrade-enterprise/services/api-gateway/internal/paper"
	"edugrade-enterprise/services/api-gateway/internal/review"
)

func TestGetContextReturnsBlindBackmarkSurface(t *testing.T) {
	store := NewMemoryStore()
	store.SeedSource(SourceTask{ReviewTaskID: "source-task-secret", OriginalGradeID: "old-grade-secret", OriginalReviewer: "grader-a", OriginalScore: 4, MaxScore: 5, GradedAt: time.Now().UTC()})
	service := NewService(store).WithContextSource(backmarkContextSource{context: review.TaskContext{
		Question:       paper.Question{ID: "question-1", QuestionNo: "Q1", QuestionType: "extended_response", Score: 5},
		FrozenRubric:   paper.Rubric{ID: "rubric-1", MaxScore: 5},
		AnswerArtifact: review.AnswerArtifact{AnswerSegmentID: "segment-secret", RawAnswer: "answer", Status: "completed"},
	}})
	summary, err := service.Create(context.Background(), "tenant-1", "exam-1", "question-1", "manager-1", CreateInput{
		SourceIncidentID: "incident-1", ReassignedTo: "grader-b", Policy: Policy{Disposition: DispositionConfirm},
	})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/backmark-items/"+summary.Items[0].ID+"/context", nil)
	request.SetPathValue("itemId", summary.Items[0].ID)
	request = request.WithContext(auth.WithUser(request.Context(), auth.User{ID: "grader-b", TenantID: "tenant-1", Roles: []string{"grader"}}))
	response := httptest.NewRecorder()

	NewHandler(service, nil).GetContext(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	body := response.Body.String()
	for _, forbidden := range []string{"source-task-secret", "old-grade-secret", "original_score", "original_reviewer", "segment-secret"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("blind context leaked %q: %s", forbidden, body)
		}
	}
	var decoded struct {
		Context GraderContext `json:"backmark_context"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Context.Question.QuestionNo != "Q1" || decoded.Context.Answer.SegmentImageURL == "" {
		t.Fatalf("missing grading context: %#v", decoded.Context)
	}
}
