package apicontract

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"edugrade-enterprise/services/api-gateway/internal/auth"
	"edugrade-enterprise/services/api-gateway/internal/grading"
)

type scoringContractStore struct {
	grading.Store
	grading.ScoringRunStore
	run grading.ScoringRun
}

func (s *scoringContractStore) StartScoringRun(context.Context, string, string, string, grading.StartScoringRunInput) (grading.ScoringRun, error) {
	return s.run, nil
}
func (s *scoringContractStore) ProcessRuleCandidates(context.Context, string, string, string, *grading.Engine) error {
	return nil
}
func (s *scoringContractStore) GetScoringSummary(context.Context, string, string) (grading.ScoringSummary, error) {
	return grading.ScoringSummary{Run: &s.run}, nil
}
func (s *scoringContractStore) RecoverScoringCommand(_ context.Context, _, _, _, id string) (grading.ScoringCommandRecovery, error) {
	result := grading.ScoringCommandRecovery{CommandID: id, Status: "not_accepted"}
	if id == s.run.IdempotencyKey {
		result.Status = "succeeded"
		result.Run = &s.run
	}
	return result, nil
}
func TestScoringCommandHandlerResponsesValidateAgainstOpenAPI(t *testing.T) {
	document := readOpenAPIDocument(t)
	store := &scoringContractStore{run: grading.ScoringRun{ID: "run-1", TenantID: "tenant-1", ExamID: "exam-1", StartedBy: "actor-1", IdempotencyKey: "command-1", Status: "processing"}}
	handler := grading.NewHandler(store, grading.NewEngine(), auth.NewMemoryStore())
	request := scopedContractRequest(httptest.NewRequest(http.MethodPost, "/api/v1/exams/exam-1/scoring-runs", strings.NewReader(`{"idempotency_key":"command-1"}`)))
	request.SetPathValue("examId", "exam-1")
	response := httptest.NewRecorder()
	handler.StartScoringRun(response, request)
	if response.Code != 201 {
		t.Fatalf("start: %d %s", response.Code, response.Body)
	}
	validateResponseBody(t, document, "/api/v1/exams/{examId}/scoring-runs", "post", "201", response.Body.Bytes())
	for _, id := range []string{"command-1", "missing"} {
		request := scopedContractRequest(httptest.NewRequest(http.MethodGet, "/", nil))
		request.SetPathValue("examId", "exam-1")
		request.SetPathValue("commandId", id)
		response := httptest.NewRecorder()
		handler.RecoverScoringCommand(response, request)
		if response.Code != 200 {
			t.Fatalf("recover: %d %s", response.Code, response.Body)
		}
		validateResponseBody(t, document, "/api/v1/exams/{examId}/scoring-runs/commands/{commandId}", "get", "200", response.Body.Bytes())
	}
}
