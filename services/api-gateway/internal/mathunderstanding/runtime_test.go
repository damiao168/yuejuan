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
	input := runtimeSpatialInput(false)
	artifact := completeRuntimeArtifact(t, input)
	if len(artifact.Blocks) != 2 {
		t.Fatalf("runtime merged OCR blocks: %#v", artifact.Blocks)
	}
	if len(artifact.Relations) == 0 {
		t.Fatal("runtime did not derive spatial relations")
	}
	if artifact.SolutionGraph.BuilderVersion != runtimeSolutionBuilderVersion {
		t.Fatalf("builder version=%q", artifact.SolutionGraph.BuilderVersion)
	}
	if len(artifact.SolutionGraph.Steps) != 2 || artifact.SolutionGraph.Steps[0].ID == "step-1" {
		t.Fatalf("runtime retained placeholder graph: %#v", artifact.SolutionGraph)
	}
	if len(artifact.SolutionGraph.Edges) == 0 {
		t.Fatalf("runtime graph has no spatial edge: %#v", artifact.SolutionGraph)
	}
}

func TestRuntimeCompletionPreservesWorkerHumanReviewSignal(t *testing.T) {
	artifact := completeRuntimeArtifact(t, runtimeSpatialInput(true))
	if !artifact.SolutionGraph.RequiresHumanReview {
		t.Fatal("runtime discarded worker human-review signal")
	}
}

func TestPrepareRuntimeArtifactPreservesWorkerRelations(t *testing.T) {
	input := runtimeSpatialInput(false)
	input.Relations = []SpatialRelation{{
		ID: "worker-relation", FromID: "block-1", ToID: "block-2", Kind: "continues", Confidence: .9,
	}}
	prepared, err := prepareRuntimeArtifact(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(prepared.Relations) != 1 || prepared.Relations[0].ID != "worker-relation" {
		t.Fatalf("runtime replaced worker relations: %#v", prepared.Relations)
	}
}

func runtimeSpatialInput(requiresHumanReview bool) CreateArtifactInput {
	input := validInput()
	input.InputHash = "hash-1"
	input.Blocks = []MathAnswerBlock{block("block-1", .1, .1), block("block-2", .1, .3)}
	input.Formulas = nil
	input.Relations = nil
	input.SolutionGraph = SolutionGraph{
		ID:                  "worker-placeholder",
		AnswerSegmentID:     input.AnswerSegmentID,
		BuilderVersion:      "math-runtime-v1",
		FormulaModelVersion: "text-only",
		OverallConfidence:   .9,
		RequiresHumanReview: requiresHumanReview,
		Steps:               []SolutionStep{{ID: "step-1", OrderHint: 1, BlockIDs: []string{"block-1", "block-2"}, Confidence: .9}},
	}
	input.Verifications = nil
	input.RubricEvidence = nil
	return input
}

func completeRuntimeArtifact(t *testing.T, input CreateArtifactInput) Artifact {
	t.Helper()
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
	return artifact
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
