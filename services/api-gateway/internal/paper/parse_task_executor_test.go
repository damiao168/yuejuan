package paper

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"edugrade-enterprise/services/api-gateway/internal/workerruntime"
)

func TestParseFinalizationContextSurvivesExecutionCancellation(t *testing.T) {
	executionCtx, cancelExecution := context.WithCancel(context.Background())
	cancelExecution()
	finalizeCtx, cancelFinalize := finalizationContext(executionCtx)
	defer cancelFinalize()
	if err := finalizeCtx.Err(); err != nil {
		t.Fatalf("finalization inherited canceled execution context: %v", err)
	}
	deadline, ok := finalizeCtx.Deadline()
	if !ok || time.Until(deadline) <= 0 || time.Until(deadline) > parseFinalizationTimeout {
		t.Fatalf("finalization context is not independently bounded: deadline=%v ok=%v", deadline, ok)
	}
}

func TestParseTaskExecutorLeaseAlwaysExceedsConfiguredTimeout(t *testing.T) {
	service := NewDocumentImportService(NewPostgresStore(nil), nil, nil, "", "", 0)
	runtime := workerruntime.NewMemoryStore()
	for _, timeout := range []time.Duration{0, 30 * time.Second, 58 * time.Minute} {
		executor, err := NewParseTaskExecutor(service, runtime, timeout)
		if err != nil {
			t.Fatalf("timeout %s rejected: %v", timeout, err)
		}
		if executor.executionTimeout < service.client.Timeout || executor.executionTimeout < timeout {
			t.Fatalf("executor timeout %s does not cover client timeout %s and requested timeout %s", executor.executionTimeout, service.client.Timeout, timeout)
		}
		if time.Duration(executor.leaseSeconds)*time.Second < executor.executionTimeout+time.Minute {
			t.Fatalf("timeout %s has unsafe lease %s", timeout, time.Duration(executor.leaseSeconds)*time.Second)
		}
	}
}

func TestParseExecutionErrorCarriesCorrelationWithoutLeaseToken(t *testing.T) {
	task := workerruntime.Task{ID: "task-1", AttemptCount: 3, LeaseToken: "must-not-leak"}
	binding := PaperImportRunBinding{ImportID: "import-1", RunID: "run-2", Generation: 2}
	err := executionError(workerruntime.ErrLeaseMismatch, task, binding, "paper_import_lease_mismatch", "rejected")
	var detail *ParseExecutionError
	if !errors.As(err, &detail) || !errors.Is(err, workerruntime.ErrLeaseMismatch) {
		t.Fatalf("correlated error must preserve its cause: %v", err)
	}
	if detail.ImportID != "import-1" || detail.RunID != "run-2" || detail.Generation != 2 || detail.TaskID != "task-1" || detail.Attempt != 3 {
		t.Fatalf("missing correlation fields: %#v", detail)
	}
	if strings.Contains(err.Error(), task.LeaseToken) || detail.LeaseValidation != "rejected" {
		t.Fatalf("lease diagnostic must report only the validation conclusion: %#v", detail)
	}
}

func TestParseTaskExecutorRejectsTimeoutWithoutLeaseSafetyMargin(t *testing.T) {
	service := NewDocumentImportService(NewPostgresStore(nil), nil, nil, "", "", 0)
	_, err := NewParseTaskExecutor(service, workerruntime.NewMemoryStore(), 60*time.Minute)
	if err == nil || !strings.Contains(err.Error(), "safety margin") {
		t.Fatalf("unsafe timeout accepted: %v", err)
	}
}
