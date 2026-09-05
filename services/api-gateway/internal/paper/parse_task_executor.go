package paper

import (
	"context"
	"errors"
	"fmt"
	"time"

	"edugrade-enterprise/services/api-gateway/internal/workerruntime"
)

type paperImportParseInputStore interface {
	LoadPaperImportParseInput(context.Context, string, string) (string, PaperImportParseRequest, error)
}

type ParseTaskExecutor struct {
	service      *DocumentImportService
	inputs       paperImportParseInputStore
	runtime      workerruntime.Store
	instanceID   string
	leaseSeconds int
	pollInterval time.Duration
}

func NewParseTaskExecutor(service *DocumentImportService, runtime workerruntime.Store, timeout time.Duration) (*ParseTaskExecutor, error) {
	if service == nil || runtime == nil {
		return nil, errors.New("paper parse executor dependencies are required")
	}
	inputs, ok := service.store.(paperImportParseInputStore)
	if !ok {
		return nil, errors.New("paper parse input store is unavailable")
	}
	leaseSeconds := int((timeout + time.Minute).Seconds())
	if leaseSeconds < 300 {
		leaseSeconds = 300
	}
	if leaseSeconds > 3600 {
		leaseSeconds = 3600
	}
	return &ParseTaskExecutor{
		service: service, inputs: inputs, runtime: runtime,
		instanceID:   fmt.Sprintf("api-gateway-%d", time.Now().UTC().UnixNano()),
		leaseSeconds: leaseSeconds, pollInterval: time.Second,
	}, nil
}

func (e *ParseTaskExecutor) RunOnce(ctx context.Context) (bool, error) {
	tasks, err := e.runtime.ClaimAcrossTenants(ctx, "", workerruntime.ClaimInput{
		QueueName: "paper-parse", WorkerService: "api-gateway-paper-parser", WorkerInstanceID: e.instanceID,
		Limit: 1, LeaseSeconds: e.leaseSeconds,
	})
	if err != nil || len(tasks) == 0 {
		return false, err
	}
	task := tasks[0]
	started := time.Now()
	if task.TaskType != "paper_parse" || task.SourceType != "paper_import_parse" {
		_, failErr := e.runtime.Fail(ctx, task.TenantID, task.ID, workerruntime.FailInput{
			LeaseToken: task.LeaseToken, Retryable: false, ErrorCode: "invalid_paper_parse_task",
		})
		return true, failErr
	}
	if _, err = e.runtime.Heartbeat(ctx, task.TenantID, task.ID, workerruntime.HeartbeatInput{
		LeaseToken: task.LeaseToken, WorkerService: "api-gateway-paper-parser", WorkerInstanceID: e.instanceID,
		State: workerruntime.StatusRunning, LeaseSeconds: e.leaseSeconds,
	}); err != nil {
		return true, err
	}
	importID, input, err := e.inputs.LoadPaperImportParseInput(ctx, task.TenantID, task.SourceID)
	if err == nil {
		_, err = e.service.ExecuteParse(ctx, task.TenantID, importID, input)
	}
	duration := int(time.Since(started).Milliseconds())
	if err != nil {
		retryable := task.AttemptCount < task.MaxAttempts && !errors.Is(err, ErrConflict) && !errors.Is(err, ErrInvalidInput)
		_, failErr := e.runtime.Fail(ctx, task.TenantID, task.ID, workerruntime.FailInput{
			LeaseToken: task.LeaseToken, Retryable: retryable, ErrorCode: "paper_parse_failed",
			ErrorDetail: map[string]any{"message": err.Error()}, DurationMS: duration,
		})
		if failErr != nil {
			return true, failErr
		}
		if !retryable && !errors.Is(err, ErrConflict) {
			_, _ = e.service.store.FailPaperImport(ctx, task.TenantID, importID, "ai_parse_failed", []string{"AI 解析服务暂不可用，可稍后重试"})
		}
		return true, err
	}
	_, err = e.runtime.Complete(ctx, task.TenantID, task.ID, workerruntime.CompleteInput{
		LeaseToken: task.LeaseToken, ResultSchemaVersion: "paper-import-parse-result-v1",
		Result: map[string]any{"paper_import_id": importID, "status": "review_required"}, DurationMS: duration,
	})
	return true, err
}

func (e *ParseTaskExecutor) Run(ctx context.Context, onError func(error)) {
	for {
		worked, err := e.RunOnce(ctx)
		if err != nil && onError != nil && !errors.Is(err, context.Canceled) {
			onError(err)
		}
		if ctx.Err() != nil {
			return
		}
		if worked {
			continue
		}
		timer := time.NewTimer(e.pollInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}
