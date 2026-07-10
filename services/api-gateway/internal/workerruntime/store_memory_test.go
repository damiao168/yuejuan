package workerruntime_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"edugrade-enterprise/services/api-gateway/internal/workerruntime"
)

const runtimeTenantID = "tenant-1"

func TestCreateTaskIsIdempotent(t *testing.T) {
	store := workerruntime.NewMemoryStore()
	input := imageQualityTaskInput()

	first, err := store.CreateTask(context.Background(), runtimeTenantID, "actor-1", input)
	if err != nil {
		t.Fatalf("create first task: %v", err)
	}
	second, err := store.CreateTask(context.Background(), runtimeTenantID, "actor-1", input)
	if err != nil {
		t.Fatalf("create duplicate task: %v", err)
	}
	if first.ID != second.ID {
		t.Fatalf("idempotent create returned different tasks: %s != %s", first.ID, second.ID)
	}
}

func TestClaimHeartbeatAndComplete(t *testing.T) {
	store := workerruntime.NewMemoryStore()
	task, _ := store.CreateTask(context.Background(), runtimeTenantID, "actor-1", imageQualityTaskInput())

	claimed, err := store.Claim(context.Background(), runtimeTenantID, workerruntime.ClaimInput{
		QueueName: "image-quality", WorkerService: "image-quality-worker", WorkerInstanceID: "worker-a", Limit: 1, LeaseSeconds: 300,
	})
	if err != nil || len(claimed) != 1 {
		t.Fatalf("claim task: %v %#v", err, claimed)
	}
	if claimed[0].ID != task.ID || claimed[0].Status != workerruntime.StatusLeased || claimed[0].LeaseToken == "" || claimed[0].AttemptCount != 1 {
		t.Fatalf("unexpected claimed task: %#v", claimed[0])
	}

	running, err := store.Heartbeat(context.Background(), runtimeTenantID, task.ID, workerruntime.HeartbeatInput{
		LeaseToken: claimed[0].LeaseToken, WorkerService: "image-quality-worker", WorkerInstanceID: "worker-a", State: workerruntime.StatusRunning, LeaseSeconds: 600,
	})
	if err != nil || running.Status != workerruntime.StatusRunning {
		t.Fatalf("heartbeat task: %v %#v", err, running)
	}
	if running.LeaseExpiresAt == nil || !running.LeaseExpiresAt.After(*claimed[0].LeaseExpiresAt) {
		t.Fatalf("heartbeat should extend lease: before=%v after=%v", claimed[0].LeaseExpiresAt, running.LeaseExpiresAt)
	}

	completed, err := store.Complete(context.Background(), runtimeTenantID, task.ID, workerruntime.CompleteInput{
		LeaseToken: claimed[0].LeaseToken, ResultSchemaVersion: "image-quality-result.v1", Result: map[string]any{"quality_status": "passed"}, DurationMS: 1200,
	})
	if err != nil || completed.Status != workerruntime.StatusSucceeded || completed.CompletedAt == nil {
		t.Fatalf("complete task: %v %#v", err, completed)
	}

	duplicate, err := store.Complete(context.Background(), runtimeTenantID, task.ID, workerruntime.CompleteInput{
		LeaseToken: claimed[0].LeaseToken, ResultSchemaVersion: "image-quality-result.v1", Result: map[string]any{"quality_status": "passed"}, DurationMS: 1200,
	})
	if err != nil || duplicate.Status != workerruntime.StatusSucceeded {
		t.Fatalf("duplicate complete should be idempotent: %v %#v", err, duplicate)
	}
	_, err = store.Complete(context.Background(), runtimeTenantID, task.ID, workerruntime.CompleteInput{
		LeaseToken: "stale-token", ResultSchemaVersion: "image-quality-result.v1", Result: map[string]any{"quality_status": "passed"}, DurationMS: 1200,
	})
	if !errors.Is(err, workerruntime.ErrLeaseMismatch) {
		t.Fatalf("duplicate complete with a stale token must be rejected, got %v", err)
	}
}

func TestRetryAndDeadLetter(t *testing.T) {
	store := workerruntime.NewMemoryStore()
	input := imageQualityTaskInput()
	input.MaxAttempts = 2
	task, _ := store.CreateTask(context.Background(), runtimeTenantID, "actor-1", input)

	first := mustClaim(t, store)
	retried, err := store.Fail(context.Background(), runtimeTenantID, task.ID, workerruntime.FailInput{
		LeaseToken: first.LeaseToken, Retryable: true, ErrorCode: "storage_timeout",
	})
	if err != nil || retried.Status != workerruntime.StatusQueued || retried.NotBefore == nil {
		t.Fatalf("retry task: %v %#v", err, retried)
	}
	store.MakeRunnableForTest(task.ID)

	second := mustClaim(t, store)
	dead, err := store.Fail(context.Background(), runtimeTenantID, task.ID, workerruntime.FailInput{
		LeaseToken: second.LeaseToken, Retryable: true, ErrorCode: "storage_timeout",
	})
	if err != nil || dead.Status != workerruntime.StatusDeadLetter {
		t.Fatalf("dead-letter task: %v %#v", err, dead)
	}

	requeued, err := store.Requeue(context.Background(), runtimeTenantID, task.ID)
	if err != nil || requeued.Status != workerruntime.StatusQueued {
		t.Fatalf("requeue dead letter: %v %#v", err, requeued)
	}
	if len(requeued.Attempts) != 2 {
		t.Fatalf("attempt history was lost: %#v", requeued.Attempts)
	}
	if requeued.MaxAttempts <= requeued.AttemptCount {
		t.Fatalf("manual requeue must grant another attempt: %#v", requeued)
	}
}

func TestExpiredLeaseCanBeReclaimedAndRejectsLateComplete(t *testing.T) {
	store := workerruntime.NewMemoryStore()
	task, _ := store.CreateTask(context.Background(), runtimeTenantID, "actor-1", imageQualityTaskInput())
	first := mustClaim(t, store)
	store.ForceExpireLeaseForTest(task.ID)

	second := mustClaim(t, store)
	if second.LeaseToken == first.LeaseToken || second.AttemptCount != 2 {
		t.Fatalf("expected a new lease and attempt: first=%#v second=%#v", first, second)
	}
	_, err := store.Complete(context.Background(), runtimeTenantID, task.ID, workerruntime.CompleteInput{
		LeaseToken: first.LeaseToken, ResultSchemaVersion: "v1", Result: map[string]any{"ok": true},
	})
	if !errors.Is(err, workerruntime.ErrLeaseMismatch) {
		t.Fatalf("late complete should fail with lease mismatch, got %v", err)
	}
}

func TestCancelAndMetrics(t *testing.T) {
	store := workerruntime.NewMemoryStore()
	first, _ := store.CreateTask(context.Background(), runtimeTenantID, "actor-1", imageQualityTaskInput())
	secondInput := imageQualityTaskInput()
	secondInput.SourceID = "quality-run-2"
	secondInput.IdempotencyKey = "image-quality-run:quality-run-2"
	_, _ = store.CreateTask(context.Background(), runtimeTenantID, "actor-1", secondInput)

	cancelled, err := store.Cancel(context.Background(), runtimeTenantID, first.ID)
	if err != nil || cancelled.Status != workerruntime.StatusCancelled {
		t.Fatalf("cancel task: %v %#v", err, cancelled)
	}
	metrics, err := store.Metrics(context.Background(), runtimeTenantID)
	if err != nil || len(metrics.Queues) != 1 || metrics.Queues[0].Queued != 1 || metrics.Queues[0].Cancelled != 1 {
		t.Fatalf("unexpected metrics: %v %#v", err, metrics)
	}
}

func TestConcurrentClaimsReturnTaskOnlyOnce(t *testing.T) {
	store := workerruntime.NewMemoryStore()
	_, _ = store.CreateTask(context.Background(), runtimeTenantID, "actor-1", imageQualityTaskInput())

	var claimedCount atomic.Int32
	var wg sync.WaitGroup
	for index := 0; index < 16; index++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			claimed, err := store.Claim(context.Background(), runtimeTenantID, workerruntime.ClaimInput{
				QueueName: "image-quality", WorkerService: "image-quality-worker", WorkerInstanceID: "worker-concurrent", Limit: 1, LeaseSeconds: 300,
			})
			if err != nil {
				t.Errorf("claim %d: %v", worker, err)
				return
			}
			claimedCount.Add(int32(len(claimed)))
		}(index)
	}
	wg.Wait()
	if claimedCount.Load() != 1 {
		t.Fatalf("one queued task must be claimed exactly once, got %d claims", claimedCount.Load())
	}
}

func TestConcurrentCompleteAndCancelProduceOneTerminalResult(t *testing.T) {
	store := workerruntime.NewMemoryStore()
	task, _ := store.CreateTask(context.Background(), runtimeTenantID, "actor-1", imageQualityTaskInput())
	claimed := mustClaim(t, store)

	start := make(chan struct{})
	results := make(chan error, 2)
	go func() {
		<-start
		_, err := store.Complete(context.Background(), runtimeTenantID, task.ID, workerruntime.CompleteInput{
			LeaseToken: claimed.LeaseToken, ResultSchemaVersion: "v1", Result: map[string]any{"ok": true},
		})
		results <- err
	}()
	go func() {
		<-start
		_, err := store.Cancel(context.Background(), runtimeTenantID, task.ID)
		results <- err
	}()
	close(start)

	first, second := <-results, <-results
	successes := 0
	for _, err := range []error{first, second} {
		if err == nil {
			successes++
		} else if !errors.Is(err, workerruntime.ErrInvalidTransition) {
			t.Fatalf("losing terminal transition should be rejected, got %v", err)
		}
	}
	if successes != 1 {
		t.Fatalf("exactly one terminal transition must succeed, got %d", successes)
	}
	final, err := store.Get(context.Background(), runtimeTenantID, task.ID)
	if err != nil || (final.Status != workerruntime.StatusSucceeded && final.Status != workerruntime.StatusCancelled) {
		t.Fatalf("unexpected terminal task: %v %#v", err, final)
	}
}

func mustClaim(t *testing.T, store *workerruntime.MemoryStore) workerruntime.Task {
	t.Helper()
	claimed, err := store.Claim(context.Background(), runtimeTenantID, workerruntime.ClaimInput{
		QueueName: "image-quality", WorkerService: "image-quality-worker", WorkerInstanceID: "worker-a", Limit: 1, LeaseSeconds: 300,
	})
	if err != nil || len(claimed) != 1 {
		t.Fatalf("claim: %v %#v", err, claimed)
	}
	return claimed[0]
}

func imageQualityTaskInput() workerruntime.CreateTaskInput {
	return workerruntime.CreateTaskInput{
		TaskType: "image_quality", QueueName: "image-quality", SourceType: "image_quality_run", SourceID: "quality-run-1",
		IdempotencyKey: "image-quality-run:quality-run-1", PayloadSchemaVersion: "image-quality.v1",
		Payload: map[string]any{"submission_page_id": "page-1"}, MaxAttempts: 3, RetryBackoffSeconds: 1,
	}
}

var _ = time.Second
