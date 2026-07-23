package ocr_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	ocrpkg "edugrade-enterprise/services/api-gateway/internal/ocr"
)

func TestMemoryStartTaskIsIdempotentWhileProcessing(t *testing.T) {
	store := ocrpkg.NewMemoryStore()
	task, err := store.CreateTask(context.Background(), tenantID, "submission-1", userID, ocrpkg.CreateTaskInput{Engine: "paddleocr", EngineVersion: "v1"})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	first, err := store.StartTask(context.Background(), tenantID, task.ID)
	if err != nil || first.StartedAt == nil {
		t.Fatalf("start task: %v %#v", err, first)
	}

	const callers = 16
	results := make(chan ocrpkg.Task, callers)
	errorsSeen := make(chan error, callers)
	var wg sync.WaitGroup
	for index := 0; index < callers; index++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			started, startErr := store.StartTask(context.Background(), tenantID, task.ID)
			results <- started
			errorsSeen <- startErr
		}()
	}
	wg.Wait()
	close(results)
	close(errorsSeen)
	for startErr := range errorsSeen {
		if startErr != nil {
			t.Fatalf("idempotent concurrent start: %v", startErr)
		}
	}
	for started := range results {
		if started.Status != "processing" || started.StartedAt == nil || !started.StartedAt.Equal(*first.StartedAt) {
			t.Fatalf("idempotent start rewrote source state: %#v", started)
		}
	}
}

func TestMemoryStartTaskRetriesFailedButRejectsCompleted(t *testing.T) {
	store := ocrpkg.NewMemoryStore()
	task, _ := store.CreateTask(context.Background(), tenantID, "submission-1", userID, ocrpkg.CreateTaskInput{Engine: "paddleocr", EngineVersion: "v1"})
	_, _ = store.StartTask(context.Background(), tenantID, task.ID)
	failed, err := store.FailTask(context.Background(), tenantID, task.ID, "lease expired")
	if err != nil || failed.CompletedAt == nil {
		t.Fatalf("fail task: %v %#v", err, failed)
	}
	retried, err := store.StartTask(context.Background(), tenantID, task.ID)
	if err != nil || retried.Status != "processing" || retried.CompletedAt != nil || retried.ErrorMessage != "" {
		t.Fatalf("retry failed source task: %v %#v", err, retried)
	}
	completed, err := store.CompleteTask(context.Background(), tenantID, task.ID, ocrpkg.CompleteTaskInput{Results: []ocrpkg.ResultInput{{
		SubmissionPageID: "page-1", Text: "answer", BBox: []float64{1, 2, 3, 4}, Confidence: 0.99,
	}}})
	if err != nil || completed.Status != "completed" {
		t.Fatalf("complete task: %v %#v", err, completed)
	}
	if _, err := store.StartTask(context.Background(), tenantID, task.ID); !errors.Is(err, ocrpkg.ErrInvalidTransition) {
		t.Fatalf("completed source task must not restart, got %v", err)
	}
}
