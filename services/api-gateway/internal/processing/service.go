package processing

import (
	"context"
	"strings"
	"time"

	"edugrade-enterprise/services/api-gateway/internal/assessment"
	"edugrade-enterprise/services/api-gateway/internal/workerruntime"
)

type Service struct {
	store   Store
	runtime workerruntime.Store
	now     func() time.Time
}

func NewService(store Store, runtime workerruntime.Store) *Service {
	return &Service{store: store, runtime: runtime, now: func() time.Time { return time.Now().UTC() }}
}

// Summary refreshes only the compact projection before aggregating it. The
// implementation is set-oriented in Postgres; it never starts processing in
// an HTTP request.
func (s *Service) Summary(ctx context.Context, tenantID, examID string) (Summary, error) {
	if s == nil || s.store == nil || strings.TrimSpace(tenantID) == "" || strings.TrimSpace(examID) == "" {
		return Summary{}, ErrInvalidInput
	}
	if err := s.store.RefreshExam(ctx, tenantID, examID); err != nil {
		return Summary{}, err
	}
	return s.store.Summary(ctx, tenantID, examID)
}

func (s *Service) ListExceptions(ctx context.Context, tenantID string, filter ExceptionFilter) (ListResult, error) {
	if s == nil || s.store == nil || strings.TrimSpace(tenantID) == "" {
		return ListResult{}, ErrInvalidInput
	}
	if filter.Limit <= 0 || filter.Limit > 100 {
		filter.Limit = 25
	}
	if filter.Severity != "" && !filter.Severity.Valid() || filter.Stage != "" && !filter.Stage.Valid() || filter.Status != "" && !filter.Status.Valid() {
		return ListResult{}, ErrInvalidInput
	}
	// An exam-specific exception list is also refreshed. Global operations
	// searches deliberately read the durable projection rather than issuing an
	// unbounded refresh across every school exam.
	if filter.ExamID != "" {
		if err := s.store.RefreshExam(ctx, tenantID, filter.ExamID); err != nil {
			return ListResult{}, err
		}
	}
	return s.store.ListExceptions(ctx, tenantID, filter)
}

func (s *Service) Assign(ctx context.Context, tenantID, exceptionID, actorID string, input AssignInput) (Exception, error) {
	if s == nil || s.store == nil || strings.TrimSpace(tenantID) == "" || strings.TrimSpace(exceptionID) == "" || strings.TrimSpace(actorID) == "" || strings.TrimSpace(input.AssigneeID) == "" {
		return Exception{}, ErrInvalidInput
	}
	return s.store.AssignException(ctx, tenantID, exceptionID, actorID, input)
}

func (s *Service) Resolve(ctx context.Context, tenantID, exceptionID, actorID string, input ResolveInput) (Exception, error) {
	if s == nil || s.store == nil || strings.TrimSpace(tenantID) == "" || strings.TrimSpace(exceptionID) == "" || strings.TrimSpace(actorID) == "" || strings.TrimSpace(input.Resolution) == "" {
		return Exception{}, ErrInvalidInput
	}
	return s.store.ResolveException(ctx, tenantID, exceptionID, actorID, input)
}

// Retry delegates to the existing worker runtime. It performs no image, OCR
// or parser work in the API process. A duplicate click is harmless because
// Requeue is itself state-checked by the worker runtime.
func (s *Service) Retry(ctx context.Context, tenantID, exceptionID string) (workerruntime.Task, error) {
	if s == nil || s.store == nil || s.runtime == nil || strings.TrimSpace(tenantID) == "" || strings.TrimSpace(exceptionID) == "" {
		return workerruntime.Task{}, ErrInvalidInput
	}
	target, err := s.store.RetryTarget(ctx, tenantID, exceptionID)
	if err != nil {
		return workerruntime.Task{}, err
	}
	task, err := s.runtime.GetBySource(ctx, tenantID, target.SourceType, target.SourceID)
	if err != nil {
		return workerruntime.Task{}, err
	}
	if task.Status == workerruntime.StatusQueued || task.Status == workerruntime.StatusLeased || task.Status == workerruntime.StatusRunning {
		return task, nil
	}
	if task.Status != workerruntime.StatusFailed && task.Status != workerruntime.StatusDeadLetter {
		return workerruntime.Task{}, ErrRetryForbidden
	}
	return s.runtime.Requeue(ctx, tenantID, task.ID)
}

// ParserQualityForSegment is the A26-to-A14 adapter. It returns the quality
// dimension matching the frozen subject/archetype and intentionally returns
// nil for absent or unsupported parser evidence. That makes A14 abstain
// instead of treating OCR text quality as a mathematical/diagram parse.
func (s *Service) ParserQualityForSegment(ctx context.Context, tenantID, segmentID string, subject assessment.SubjectCode, archetype string) (*float64, error) {
	if s == nil || s.store == nil || strings.TrimSpace(tenantID) == "" || strings.TrimSpace(segmentID) == "" || !subject.Valid() {
		return nil, ErrInvalidInput
	}
	return s.store.ParserQualityForSegment(ctx, tenantID, segmentID, subject, archetype)
}
