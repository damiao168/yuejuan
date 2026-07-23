package ocr

import (
	"context"
	"errors"
	"strings"
	"testing"

	"edugrade-enterprise/services/api-gateway/internal/workerruntime"
)

func TestMemoryRuntimeCoordinatorRollsBackCreateAndRetriesIdempotently(t *testing.T) {
	ctx := context.Background()
	source := NewMemoryStore()
	runtime := workerruntime.NewMemoryStore()
	coordinator := &memoryRuntimeCoordinator{source: source, runtime: runtime}
	injected := errors.New("injected create failure")
	coordinator.hook = func(point string) error {
		if point == "create.after_runtime" {
			return injected
		}
		return nil
	}
	input := CreateTaskInput{Engine: "paddleocr", EngineVersion: "pp-ocrv5", MinConfidence: 0.8, IdempotencyKey: "create-atomic-1"}

	if _, err := coordinator.CreateTask(ctx, "tenant-1", "submission-1", "actor-1", input); !errors.Is(err, injected) {
		t.Fatalf("create should return injected failure, got %v", err)
	}
	if tasks, _ := source.ListBySubmission(ctx, "tenant-1", "submission-1"); len(tasks) != 0 {
		t.Fatalf("source task leaked after rollback: %#v", tasks)
	}
	if _, err := runtime.GetBySource(ctx, "tenant-1", "ocr_task", "ocr-task-1"); !errors.Is(err, workerruntime.ErrNotFound) {
		t.Fatalf("runtime task leaked after rollback: %v", err)
	}

	coordinator.hook = nil
	first, err := coordinator.CreateTask(ctx, "tenant-1", "submission-1", "actor-1", input)
	if err != nil {
		t.Fatalf("create after rollback: %v", err)
	}
	second, err := coordinator.CreateTask(ctx, "tenant-1", "submission-1", "actor-1", input)
	if err != nil || second.ID != first.ID {
		t.Fatalf("idempotent retry returned %#v, %v; first=%#v", second, err, first)
	}
	if tasks, _ := source.ListBySubmission(ctx, "tenant-1", "submission-1"); len(tasks) != 1 {
		t.Fatalf("idempotent retry created %d source tasks", len(tasks))
	}
	runtimeTask, err := runtime.GetBySource(ctx, "tenant-1", "ocr_task", first.ID)
	if err != nil || runtimeTask.Status != workerruntime.StatusQueued {
		t.Fatalf("runtime task missing after committed create: %v %#v", err, runtimeTask)
	}

	conflict := input
	conflict.EngineVersion = "different-version"
	if _, err = coordinator.CreateTask(ctx, "tenant-1", "submission-1", "actor-1", conflict); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("reused key with different source input should conflict, got %v", err)
	}
}

func TestMemoryRuntimeCoordinatorRollsBackTerminalMutations(t *testing.T) {
	ctx := context.Background()
	source := NewMemoryStore()
	runtime := workerruntime.NewMemoryStore()
	coordinator := &memoryRuntimeCoordinator{source: source, runtime: runtime}
	task, err := coordinator.CreateTask(ctx, "tenant-1", "submission-1", "actor-1", CreateTaskInput{
		Engine: "paddleocr", EngineVersion: "pp-ocrv5", MinConfidence: 0.8, IdempotencyKey: "terminal-atomic-1",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	claimed, err := runtime.Claim(ctx, "tenant-1", workerruntime.ClaimInput{
		QueueName: "ocr", WorkerService: "ocr-worker", WorkerInstanceID: "worker-1", Limit: 1, LeaseSeconds: 300,
	})
	if err != nil || len(claimed) != 1 {
		t.Fatalf("claim: %v %#v", err, claimed)
	}
	if _, err = source.StartTask(ctx, "tenant-1", task.ID); err != nil {
		t.Fatalf("start source: %v", err)
	}
	complete := CompleteTaskInput{
		Results:      []ResultInput{{SubmissionPageID: "page-1", Text: "x & y < z > 0", BBox: []float64{1, 2, 3, 4}, Confidence: 0.95}},
		ModelVersion: "model&<v>", ConfigHash: "cfg", InputHash: "input", DurationMS: 12,
		WorkerID: "worker-1", PreprocessProfile: "default",
		RuntimeTaskID: claimed[0].ID, RuntimeLeaseToken: claimed[0].LeaseToken,
	}
	injected := errors.New("injected terminal failure")
	coordinator.hook = func(point string) error {
		if point == "complete.after_runtime" {
			return injected
		}
		return nil
	}
	if _, err = coordinator.CompleteTask(ctx, "tenant-1", task.ID, complete); !errors.Is(err, injected) {
		t.Fatalf("completion should return injected failure, got %v", err)
	}
	sourceAfterRollback, _ := source.GetTask(ctx, "tenant-1", task.ID)
	runtimeAfterRollback, _ := runtime.Get(ctx, "tenant-1", claimed[0].ID)
	if sourceAfterRollback.Status != "processing" || len(sourceAfterRollback.Results) != 0 {
		t.Fatalf("source completion leaked after rollback: %#v", sourceAfterRollback)
	}
	if runtimeAfterRollback.Status != workerruntime.StatusLeased || len(runtimeAfterRollback.Result) != 0 {
		t.Fatalf("runtime completion leaked after rollback: %#v", runtimeAfterRollback)
	}

	coordinator.hook = nil
	completed, err := coordinator.CompleteTask(ctx, "tenant-1", task.ID, complete)
	if err != nil || completed.Status != "completed" {
		t.Fatalf("completion after rollback: %v %#v", err, completed)
	}
	runtimeCompleted, _ := runtime.Get(ctx, "tenant-1", claimed[0].ID)
	if runtimeCompleted.Status != workerruntime.StatusSucceeded {
		t.Fatalf("runtime not completed atomically: %#v", runtimeCompleted)
	}
	if _, err = coordinator.CompleteTask(ctx, "tenant-1", task.ID, complete); err != nil {
		t.Fatalf("same completion retry should be idempotent: %v", err)
	}
}

func TestMemoryRuntimeCoordinatorRollsBackFailureAndRetries(t *testing.T) {
	ctx := context.Background()
	source := NewMemoryStore()
	runtime := workerruntime.NewMemoryStore()
	coordinator := &memoryRuntimeCoordinator{source: source, runtime: runtime}
	task, err := coordinator.CreateTask(ctx, "tenant-1", "submission-1", "actor-1", CreateTaskInput{
		Engine: "paddleocr", EngineVersion: "pp-ocrv5", MinConfidence: 0.8, IdempotencyKey: "fail-atomic-1",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	claimed, err := runtime.Claim(ctx, "tenant-1", workerruntime.ClaimInput{
		QueueName: "ocr", WorkerService: "ocr-worker", WorkerInstanceID: "worker-1", Limit: 1, LeaseSeconds: 300,
	})
	if err != nil || len(claimed) != 1 {
		t.Fatalf("claim: %v %#v", err, claimed)
	}
	if _, err = source.StartTask(ctx, "tenant-1", task.ID); err != nil {
		t.Fatalf("start source: %v", err)
	}
	fail := FailTaskInput{
		ErrorMessage: "engine_unavailable", RuntimeTaskID: claimed[0].ID,
		RuntimeLeaseToken: claimed[0].LeaseToken, Retryable: false,
	}
	injected := errors.New("injected failure rollback")
	coordinator.hook = func(point string) error {
		if point == "fail.after_runtime" {
			return injected
		}
		return nil
	}
	if _, err = coordinator.FailTask(ctx, "tenant-1", task.ID, fail); !errors.Is(err, injected) {
		t.Fatalf("failure should return injected error, got %v", err)
	}
	sourceAfterRollback, _ := source.GetTask(ctx, "tenant-1", task.ID)
	runtimeAfterRollback, _ := runtime.Get(ctx, "tenant-1", claimed[0].ID)
	if sourceAfterRollback.Status != "processing" || sourceAfterRollback.ErrorMessage != "" {
		t.Fatalf("source failure leaked after rollback: %#v", sourceAfterRollback)
	}
	if runtimeAfterRollback.Status != workerruntime.StatusLeased || runtimeAfterRollback.ErrorCode != "" {
		t.Fatalf("runtime failure leaked after rollback: %#v", runtimeAfterRollback)
	}

	coordinator.hook = nil
	failed, err := coordinator.FailTask(ctx, "tenant-1", task.ID, fail)
	if err != nil || failed.Status != "failed" {
		t.Fatalf("failure after rollback: %v %#v", err, failed)
	}
	runtimeFailed, _ := runtime.Get(ctx, "tenant-1", claimed[0].ID)
	if runtimeFailed.Status != workerruntime.StatusFailed || len(runtimeFailed.Attempts) != 1 || runtimeFailed.Attempts[0].Status != workerruntime.StatusFailed {
		t.Fatalf("runtime attempt not failed atomically: %#v", runtimeFailed)
	}
}

func TestCompletedRuntimeReconciliationStatusPolicy(t *testing.T) {
	for _, status := range []string{
		workerruntime.StatusQueued,
		workerruntime.StatusLeased,
		workerruntime.StatusRunning,
		workerruntime.StatusFailed,
		workerruntime.StatusDeadLetter,
	} {
		if !canReconcileRuntimeStatus(status) {
			t.Errorf("status %q should be recoverable", status)
		}
	}
	for _, status := range []string{workerruntime.StatusSucceeded, workerruntime.StatusCancelled, "unknown"} {
		if canReconcileRuntimeStatus(status) {
			t.Errorf("status %q should require separate handling", status)
		}
	}
}

func TestRuntimeResultCanonicalJSONEscapesSpecialCharacters(t *testing.T) {
	task := Task{
		ID: "task-1", ResultCount: 2, ModelVersion: "m&<v>\u2028line\u2029end",
		ConfigHash: "cfg<&>", InputHash: "input&hash",
	}
	raw, err := workerruntime.CanonicalResultJSON(RuntimeResultSchema, RuntimeResult(task))
	if err != nil {
		t.Fatalf("canonical runtime result: %v", err)
	}
	for _, escaped := range []string{`m\u0026\u003cv\u003e\u2028line\u2029end`, `cfg\u003c\u0026\u003e`, `input\u0026hash`} {
		if !strings.Contains(string(raw), escaped) {
			t.Fatalf("canonical JSON %s missing escape %s", raw, escaped)
		}
	}
}
