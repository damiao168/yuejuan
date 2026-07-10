package evidence

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type MemoryStore struct {
	mu       sync.RWMutex
	next     int
	contexts map[string]Context
	jobs     map[string][]AgentJob
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{next: 1, contexts: map[string]Context{}, jobs: map[string][]AgentJob{}}
}

func (s *MemoryStore) AddContext(tenantID string, gradeID string, ctx Context) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx.Grade.ID = gradeID
	ctx.Grade.TenantID = tenantID
	s.contexts[key(tenantID, gradeID)] = ctx
}

func (s *MemoryStore) LoadContext(_ context.Context, tenantID string, gradeID string) (Context, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ctx, ok := s.contexts[key(tenantID, gradeID)]
	if !ok {
		return Context{}, ErrNotFound
	}
	return ctx, nil
}

func (s *MemoryStore) CreateJob(_ context.Context, tenantID string, actorID string, gradeID string, result VerificationResult) (AgentJob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.contexts[key(tenantID, gradeID)]; !ok {
		return AgentJob{}, ErrNotFound
	}
	job := AgentJob{
		ID:               s.id("agent-job"),
		TenantID:         tenantID,
		JobType:          "evidence_check",
		TargetType:       "ai_grade",
		TargetID:         gradeID,
		Status:           statusFromResult(result),
		Result:           result,
		NeedsHumanReview: result.NeedsHumanReview,
		CreatedBy:        actorID,
		CreatedAt:        time.Now().UTC(),
	}
	s.jobs[key(tenantID, gradeID)] = append(s.jobs[key(tenantID, gradeID)], job)
	return job, nil
}

func key(tenantID string, gradeID string) string {
	return tenantID + "|" + gradeID
}

func (s *MemoryStore) id(prefix string) string {
	id := fmt.Sprintf("%s-%d", prefix, s.next)
	s.next++
	return id
}

func statusFromResult(result VerificationResult) string {
	if !result.Passed {
		return "failed"
	}
	if len(result.Warnings) > 0 || result.NeedsHumanReview {
		return "warning"
	}
	return "passed"
}
