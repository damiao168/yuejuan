package paper

import (
	"context"
	"errors"
	"fmt"
	"time"

	"edugrade-enterprise/services/api-gateway/internal/workerruntime"
	"github.com/google/uuid"
)

type paperImportParseInputStore interface {
	LoadPaperImportParseRunInput(context.Context, string, string) (PaperImportRunBinding, error)
}

type paperImportParseCommitter interface {
	CompletePaperImportParseTask(context.Context, string, string, string, PaperImportRunBinding, PaperImportParseResult, int) (PaperImportJob, error)
	FailPaperImportRuntime(context.Context, string, string, PaperImportRuntimeFailure) error
}

type paperImportDispatchStore interface {
	ReconcileExpiredPaperImportTask(context.Context) (bool, error)
	ClaimPendingPaperImportDispatch(context.Context, string, time.Duration) (PaperImportJob, bool, error)
	FailPendingPaperImportDispatch(context.Context, PaperImportJob, string, string) error
}

const parseFinalizationTimeout = 10 * time.Second

type ParseTaskExecutor struct {
	service          *DocumentImportService
	inputs           paperImportParseInputStore
	committer        paperImportParseCommitter
	runtime          workerruntime.Store
	instanceID       string
	leaseSeconds     int
	executionTimeout time.Duration
	pollInterval     time.Duration
	dispatches       paperImportDispatchStore
}

// ParseExecutionError keeps operational correlation data without exposing the
// lease token. Unwrap preserves the domain error used by retry decisions/tests.
type ParseExecutionError struct {
	Err             error
	ImportID        string
	RunID           string
	Generation      int64
	TaskID          string
	Attempt         int
	ErrorCode       string
	LeaseValidation string
}

func (e *ParseExecutionError) Error() string { return e.Err.Error() }
func (e *ParseExecutionError) Unwrap() error { return e.Err }

func executionError(err error, task workerruntime.Task, binding PaperImportRunBinding, errorCode string, leaseValidation string) error {
	if err == nil {
		return nil
	}
	return &ParseExecutionError{
		Err: err, ImportID: binding.ImportID, RunID: binding.RunID, Generation: binding.Generation,
		TaskID: task.ID, Attempt: task.AttemptCount, ErrorCode: errorCode, LeaseValidation: leaseValidation,
	}
}

func parseCommitErrorCode(err error) string {
	switch {
	case errors.Is(err, workerruntime.ErrLeaseExpired):
		return "paper_import_lease_expired"
	case errors.Is(err, workerruntime.ErrLeaseMismatch):
		return "paper_import_lease_mismatch"
	case errors.Is(err, ErrConflict):
		return "paper_import_version_or_result_conflict"
	default:
		return "paper_import_result_commit_failed"
	}
}

func NewParseTaskExecutor(service *DocumentImportService, runtime workerruntime.Store, timeout time.Duration) (*ParseTaskExecutor, error) {
	if service == nil || runtime == nil {
		return nil, errors.New("paper parse executor dependencies are required")
	}
	if service.client != nil && timeout < service.client.Timeout {
		timeout = service.client.Timeout
	}
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	if timeout > 59*time.Minute {
		return nil, errors.New("paper parse timeout must leave at least one minute of lease safety margin")
	}
	inputs, ok := service.store.(paperImportParseInputStore)
	if !ok {
		return nil, errors.New("paper parse input store is unavailable")
	}
	committer, ok := service.store.(paperImportParseCommitter)
	if !ok {
		return nil, errors.New("paper parse transactional committer is unavailable")
	}
	dispatches, ok := service.store.(paperImportDispatchStore)
	if !ok {
		return nil, errors.New("paper import durable dispatch reconciler is unavailable")
	}
	leaseSeconds := int((timeout + time.Minute).Seconds())
	if leaseSeconds < 300 {
		leaseSeconds = 300
	}
	if leaseSeconds > 3600 {
		leaseSeconds = 3600
	}
	executor := &ParseTaskExecutor{
		service: service, inputs: inputs, committer: committer, runtime: runtime,
		instanceID:   fmt.Sprintf("api-gateway-%d", time.Now().UTC().UnixNano()),
		leaseSeconds: leaseSeconds, pollInterval: time.Second,
		executionTimeout: timeout,
	}
	executor.dispatches = dispatches
	return executor, nil
}

func (e *ParseTaskExecutor) RunOnce(ctx context.Context) (bool, error) {
	if e.dispatches != nil {
		if repaired, err := e.dispatches.ReconcileExpiredPaperImportTask(ctx); repaired || err != nil {
			return repaired, err
		}
		owner := e.instanceID + ":" + uuid.NewString()
		job, ok, claimErr := e.dispatches.ClaimPendingPaperImportDispatch(ctx, owner, time.Duration(e.leaseSeconds)*time.Second)
		if claimErr != nil {
			return false, claimErr
		}
		if ok {
			executionCtx, cancelExecution := e.executionContext(ctx)
			_, dispatchErr := e.service.processSources(executionCtx, job.TenantID, job.CreatedBy, job)
			cancelExecution()
			if dispatchErr != nil {
				finalizeCtx, cancelFinalize := finalizationContext(ctx)
				persistenceErr := e.dispatches.FailPendingPaperImportDispatch(finalizeCtx, job, owner, dispatchErr.Error())
				cancelFinalize()
				if persistenceErr != nil {
					dispatchErr = errors.Join(dispatchErr, fmt.Errorf("persist dispatch failure: %w", persistenceErr))
				}
			}
			if dispatchErr != nil {
				return true, &ParseExecutionError{Err: dispatchErr, ImportID: job.ID, RunID: job.RunID, Generation: job.Generation, ErrorCode: "paper_import_dispatch_failed"}
			}
			return true, nil
		}
	}
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
		return true, executionError(err, task, PaperImportRunBinding{}, parseCommitErrorCode(err), "rejected")
	}
	binding, err := e.inputs.LoadPaperImportParseRunInput(ctx, task.TenantID, task.SourceID)
	if binding.ImportID == "" {
		if value, ok := task.Payload["paper_import_id"].(string); ok {
			binding.ImportID = value
		}
	}
	var parsed documentParseResponse
	if err == nil {
		executionCtx, cancelExecution := e.executionContext(ctx)
		parsed, err = e.service.computeParseRun(executionCtx, task.TenantID, binding)
		cancelExecution()
	}
	duration := int(time.Since(started).Milliseconds())
	if err != nil {
		retryable := task.AttemptCount < task.MaxAttempts && !errors.Is(err, ErrConflict) && !errors.Is(err, ErrInvalidInput)
		finalizeCtx, cancelFinalize := finalizationContext(ctx)
		failErr := e.committer.FailPaperImportRuntime(finalizeCtx, task.TenantID, binding.ImportID, PaperImportRuntimeFailure{TaskID: task.ID, LeaseToken: task.LeaseToken, Retryable: retryable, ErrorCode: "paper_parse_failed", ErrorDetail: map[string]any{"message": err.Error()}, DurationMS: duration})
		cancelFinalize()
		if failErr != nil {
			return true, executionError(failErr, task, binding, "paper_import_failure_commit_failed", "accepted")
		}
		return true, executionError(err, task, binding, "paper_parse_failed", "accepted")
	}
	finalizeCtx, cancelFinalize := finalizationContext(ctx)
	_, err = e.committer.CompletePaperImportParseTask(finalizeCtx, task.TenantID, task.ID, task.LeaseToken, binding, parsed, duration)
	cancelFinalize()
	leaseValidation := "accepted"
	if errors.Is(err, workerruntime.ErrLeaseExpired) || errors.Is(err, workerruntime.ErrLeaseMismatch) {
		leaseValidation = "rejected"
	}
	return true, executionError(err, task, binding, parseCommitErrorCode(err), leaseValidation)
}

func (e *ParseTaskExecutor) executionContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if e.executionTimeout > 0 {
		return context.WithTimeout(ctx, e.executionTimeout)
	}
	return context.WithCancel(ctx)
}

func finalizationContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), parseFinalizationTimeout)
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
