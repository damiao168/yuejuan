package ocr

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

type MemoryStore struct {
	mu      sync.RWMutex
	next    int
	tasks   map[string]Task
	results map[string][]Result
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{next: 1, tasks: map[string]Task{}, results: map[string][]Result{}}
}

func (s *MemoryStore) CreateTask(_ context.Context, tenantID string, submissionID string, actorID string, input CreateTaskInput) (Task, error) {
	input = NormalizeCreateInput(input)
	if err := ValidateCreateInput(input); err != nil {
		return Task{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	task := Task{
		ID:            s.id("ocr-task"),
		TenantID:      tenantID,
		SubmissionID:  submissionID,
		Status:        "queued",
		Engine:        input.Engine,
		EngineVersion: input.EngineVersion,
		MinConfidence: input.MinConfidence,
		RequestedBy:   actorID,
		CreatedAt:     time.Now().UTC(),
	}
	s.tasks[task.ID] = task
	return task, nil
}

func (s *MemoryStore) ListPending(_ context.Context, tenantID string, limit int) ([]Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if limit <= 0 || limit > 100 {
		limit = 10
	}
	out := []Task{}
	for _, task := range s.tasks {
		if task.TenantID == tenantID && task.Status == "queued" {
			out = append(out, task)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (s *MemoryStore) ListBySubmission(_ context.Context, tenantID string, submissionID string) ([]Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []Task{}
	for _, task := range s.tasks {
		if task.TenantID == tenantID && task.SubmissionID == submissionID {
			out = append(out, task)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

func (s *MemoryStore) GetTask(_ context.Context, tenantID string, id string) (Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	task, ok := s.tasks[id]
	if !ok || task.TenantID != tenantID {
		return Task{}, ErrNotFound
	}
	task.Results = append([]Result{}, s.results[id]...)
	return task, nil
}

func (s *MemoryStore) StartTask(_ context.Context, tenantID string, id string) (Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	task, ok := s.tasks[id]
	if !ok || task.TenantID != tenantID {
		return Task{}, ErrNotFound
	}
	if !CanStart(task.Status) {
		return Task{}, ErrInvalidTransition
	}
	now := time.Now().UTC()
	task.Status = "processing"
	task.StartedAt = &now
	s.tasks[id] = task
	return task, nil
}

func (s *MemoryStore) CompleteTask(_ context.Context, tenantID string, id string, input CompleteTaskInput) (Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	task, ok := s.tasks[id]
	if !ok || task.TenantID != tenantID {
		return Task{}, ErrNotFound
	}
	if !CanComplete(task.Status) {
		return Task{}, ErrInvalidTransition
	}
	if len(input.Results) == 0 {
		return Task{}, ErrInvalidInput
	}
	results := make([]Result, 0, len(input.Results))
	requiresReview := false
	for _, item := range input.Results {
		if err := ValidateResultInput(item); err != nil {
			return Task{}, err
		}
		if item.Confidence < task.MinConfidence {
			requiresReview = true
		}
		results = append(results, Result{
			ID:                s.id("ocr-result"),
			TenantID:          tenantID,
			TaskID:            id,
			SubmissionID:      task.SubmissionID,
			SubmissionPageID:  item.SubmissionPageID,
			Text:              item.Text,
			BBox:              append([]float64{}, item.BBox...),
			Confidence:        item.Confidence,
			OCREngine:         task.Engine,
			OCRVersion:        task.EngineVersion,
			ModelVersion:      input.ModelVersion,
			ConfigHash:        input.ConfigHash,
			InputHash:         input.InputHash,
			PreprocessProfile: input.PreprocessProfile,
			SourceImageFileID: item.SourceImageFileID,
			CreatedAt:         time.Now().UTC(),
		})
	}
	now := time.Now().UTC()
	task.Status = "completed"
	task.ModelVersion = input.ModelVersion
	task.ConfigHash = input.ConfigHash
	task.InputHash = input.InputHash
	task.DurationMS = input.DurationMS
	task.WorkerID = input.WorkerID
	task.AttemptCount++
	task.ResultCount = len(results)
	task.RequiresHumanReview = requiresReview
	task.CompletedAt = &now
	task.Results = results
	s.tasks[id] = task
	s.results[id] = results
	return task, nil
}

func (s *MemoryStore) FailTask(_ context.Context, tenantID string, id string, errorMessage string) (Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	task, ok := s.tasks[id]
	if !ok || task.TenantID != tenantID {
		return Task{}, ErrNotFound
	}
	if !CanFail(task.Status) {
		return Task{}, ErrInvalidTransition
	}
	now := time.Now().UTC()
	task.Status = "failed"
	task.ErrorMessage = errorMessage
	task.CompletedAt = &now
	s.tasks[id] = task
	return task, nil
}

func (s *MemoryStore) id(prefix string) string {
	id := fmt.Sprintf("%s-%d", prefix, s.next)
	s.next++
	return id
}
