package answergroup

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"edugrade-enterprise/services/api-gateway/internal/auth"
)

func TestConfirmHandlerReturnsPolicyConflict(t *testing.T) {
	store := NewMemoryStore(nil, DefaultPolicy())
	store.SetSourceAnswers("tenant", "exam", "question", []SourceAnswer{
		{SubmissionID: "s1", SegmentID: "a1", SnapshotID: "snapshot", ArchetypeCode: ArchetypeExactText, AnswerText: "A", Source: "manual_entry"},
		{SubmissionID: "s2", SegmentID: "a2", SnapshotID: "snapshot", ArchetypeCode: ArchetypeExactText, AnswerText: "A", Source: "manual_entry"},
	})
	groups, err := store.Build(context.Background(), "tenant", "exam", "question", "teacher", BuildInput{})
	if err != nil {
		t.Fatal(err)
	}
	group, err := store.PutDecision(context.Background(), "tenant", groups[0].ID, "teacher", DecisionInput{
		ScoreCandidate: map[string]any{"score": 1}, RubricSelection: map[string]any{"P1": "matched"},
	})
	if err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(store, nil)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/answer-groups/"+group.ID+"/confirm", bytes.NewBufferString(`{"expected_revision":1}`))
	request.SetPathValue("groupId", group.ID)
	request = request.WithContext(auth.WithUser(request.Context(), auth.User{ID: "teacher", TenantID: "tenant"}))
	recorder := httptest.NewRecorder()
	handler.Confirm(recorder, request)
	if recorder.Code != http.StatusConflict || !strings.Contains(recorder.Body.String(), "answer_group_sampling_incomplete") {
		t.Fatalf("expected explicit policy conflict, got %d %s", recorder.Code, recorder.Body.String())
	}
}
