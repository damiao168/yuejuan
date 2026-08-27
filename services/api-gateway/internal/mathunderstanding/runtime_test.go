package mathunderstanding

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"edugrade-enterprise/services/api-gateway/internal/auth"
	"edugrade-enterprise/services/api-gateway/internal/workerruntime"
)

func TestRuntimeCompletionPersistsBoundArtifactAndCompletesLease(t *testing.T) {
	ctx := context.Background()
	artifacts := NewMemoryStore()
	runtime := workerruntime.NewMemoryStore()
	task, err := runtime.CreateTask(ctx, "tenant-a", "worker-user", workerruntime.CreateTaskInput{
		TaskType: "evidence_verify", QueueName: "math-understanding", SourceType: "answer_segment", SourceID: "segment-1",
		PayloadSchemaVersion: "math-understanding-task-v1", IdempotencyKey: "math:segment-1:hash-1",
		Payload: map[string]any{"answer_segment_id": "segment-1", "exam_question_snapshot_id": "snapshot-1", "subject_code": "mathematics", "region_kind": "formula", "input_hash": "hash-1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	claimed, err := runtime.Claim(ctx, "tenant-a", workerruntime.ClaimInput{QueueName: "math-understanding", WorkerService: "ocr-worker", WorkerInstanceID: "worker-1", Limit: 1, LeaseSeconds: 300})
	if err != nil || len(claimed) != 1 {
		t.Fatalf("claim: items=%d err=%v", len(claimed), err)
	}
	input := validInput()
	input.InputHash = "hash-1"
	body, _ := json.Marshal(completeRuntimeRequest{LeaseToken: claimed[0].LeaseToken, DurationMS: 12, Artifact: input})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/internal/math-understanding/tasks/"+task.ID+"/complete", bytes.NewReader(body))
	req.SetPathValue("taskId", task.ID)
	req = req.WithContext(auth.WithUser(req.Context(), auth.User{ID: "worker-user", TenantID: "tenant-a"}))
	rec := httptest.NewRecorder()
	NewHandler(artifacts, NewMemoryCorrectionStore(artifacts), NewMemoryPilotGateStore(), nil, nil).WithRuntime(runtime).CompleteRuntimeTask(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("complete code=%d body=%s", rec.Code, rec.Body.String())
	}
	completed, err := runtime.Get(ctx, "tenant-a", task.ID)
	if err != nil || completed.Status != workerruntime.StatusSucceeded {
		t.Fatalf("runtime status=%q err=%v", completed.Status, err)
	}
	artifact, err := artifacts.GetLatestArtifact(ctx, "tenant-a", "segment-1")
	if err != nil || artifact.InputHash != "hash-1" {
		t.Fatalf("artifact=%#v err=%v", artifact, err)
	}
}

func TestArtifactCreationIsIdempotentForSameCropHash(t *testing.T) {
	store := NewMemoryStore()
	input := validInput()
	first, err := store.CreateArtifact(context.Background(), "tenant-a", input)
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.CreateArtifact(context.Background(), "tenant-a", input)
	if err != nil {
		t.Fatal(err)
	}
	if second.ID != first.ID || second.Version != first.Version {
		t.Fatalf("same crop must reuse artifact: first=%#v second=%#v", first, second)
	}
}
