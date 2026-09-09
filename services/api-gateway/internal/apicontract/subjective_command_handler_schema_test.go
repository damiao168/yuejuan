package apicontract

import (
	"edugrade-enterprise/services/api-gateway/internal/auth"
	"edugrade-enterprise/services/api-gateway/internal/paper"
	"edugrade-enterprise/services/api-gateway/internal/subjective"
	"edugrade-enterprise/services/api-gateway/internal/workerruntime"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSubjectiveCommandHandlerResponsesValidateAgainstOpenAPI(t *testing.T) {
	doc := readOpenAPIDocument(t)
	store := subjective.NewMemoryStore()
	store.AddContext("tenant-1", "segment-1", subjective.Context{AnswerVersion: "v1", AnswerText: "answer", Question: paper.Question{ID: "q1", QuestionType: "short_answer", Score: 5}, Rubric: paper.Rubric{ID: "r1", Version: "v1"}})
	handler := subjective.NewHandler(store, subjective.NewMockLLMAdapter(), auth.NewMemoryStore()).WithWorkerRuntimeStore(workerruntime.NewMemoryStore())
	req := scopedContractRequest(httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"idempotency_key":"subjective-contract","segment_ids":["segment-1"]}`)))
	rec := httptest.NewRecorder()
	handler.CreateBatch(rec, req)
	if rec.Code != 201 {
		t.Fatalf("create: %d %s", rec.Code, rec.Body)
	}
	validateResponseBody(t, doc, "/api/v1/subjective-grading-batches", "post", "201", rec.Body.Bytes())
	var created struct {
		Batch subjective.GradingBatch `json:"batch"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	for _, commandID := range []string{"subjective-contract", "missing"} {
		req := scopedContractRequest(httptest.NewRequest(http.MethodGet, "/", nil))
		req.SetPathValue("commandId", commandID)
		rec := httptest.NewRecorder()
		handler.RecoverBatchCommand(rec, req)
		if rec.Code != 200 {
			t.Fatalf("recover: %d %s", rec.Code, rec.Body)
		}
		validateResponseBody(t, doc, "/api/v1/subjective-grading-batch-commands/{commandId}", "get", "200", rec.Body.Bytes())
	}
	req = scopedContractRequest(httptest.NewRequest(http.MethodPost, "/", nil))
	req.SetPathValue("batchId", created.Batch.ID)
	rec = httptest.NewRecorder()
	handler.EnqueueBatch(rec, req)
	if rec.Code != 200 {
		t.Fatalf("enqueue: %d %s", rec.Code, rec.Body)
	}
	validateResponseBody(t, doc, "/api/v1/subjective-grading-batches/{batchId}/enqueue", "post", "200", rec.Body.Bytes())
	rec = httptest.NewRecorder()
	handler.RecoverEnqueueCommand(rec, req)
	if rec.Code != 200 {
		t.Fatalf("enqueue recovery: %d %s", rec.Code, rec.Body)
	}
	validateResponseBody(t, doc, "/api/v1/subjective-grading-batches/{batchId}/enqueue-command", "get", "200", rec.Body.Bytes())
}
