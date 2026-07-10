package exam

import (
	"context"
	"fmt"
	"sync"
)

type MemoryStore struct {
	mu    sync.RWMutex
	next  int
	items map[string]Exam
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{next: 1, items: map[string]Exam{}}
}

func (s *MemoryStore) CreateExam(_ context.Context, tenantID string, createdBy string, input CreateInput) (Exam, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	appealEnabled := true
	if input.AppealEnabled != nil {
		appealEnabled = *input.AppealEnabled
	}
	item := Exam{
		ID:            s.id(),
		TenantID:      tenantID,
		SchoolID:      input.SchoolID,
		Name:          input.Name,
		Subject:       input.Subject,
		ExamType:      input.ExamType,
		TotalScore:    input.TotalScore,
		Status:        "draft",
		GradingMode:   input.GradingMode,
		AppealEnabled: appealEnabled,
		PublishPolicy: input.PublishPolicy,
		CreatedBy:     createdBy,
		ClassIDs:      cloneStrings(input.ClassIDs),
	}
	s.items[item.ID] = item
	return item, nil
}

func (s *MemoryStore) ListExams(_ context.Context, tenantID string, filter ListFilter) ([]Exam, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []Exam{}
	for _, item := range s.items {
		if item.TenantID != tenantID {
			continue
		}
		if filter.Status != "" && item.Status != filter.Status {
			continue
		}
		if filter.SchoolID != "" && item.SchoolID != filter.SchoolID {
			continue
		}
		out = append(out, item)
	}
	return out, nil
}

func (s *MemoryStore) GetExam(_ context.Context, tenantID string, id string) (Exam, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	item, ok := s.items[id]
	if !ok || item.TenantID != tenantID {
		return Exam{}, ErrNotFound
	}
	return item, nil
}

func (s *MemoryStore) UpdateExam(_ context.Context, tenantID string, id string, input UpdateInput) (Exam, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.items[id]
	if !ok || item.TenantID != tenantID {
		return Exam{}, ErrNotFound
	}
	if IsCoreLocked(item.Status) {
		return Exam{}, ErrLocked
	}
	if input.SchoolID != nil {
		item.SchoolID = *input.SchoolID
	}
	if input.Name != nil {
		item.Name = *input.Name
	}
	if input.Subject != nil {
		item.Subject = *input.Subject
	}
	if input.ExamType != nil {
		item.ExamType = *input.ExamType
	}
	if input.TotalScore != nil {
		item.TotalScore = *input.TotalScore
	}
	if input.GradingMode != nil {
		item.GradingMode = *input.GradingMode
	}
	if input.AppealEnabled != nil {
		item.AppealEnabled = *input.AppealEnabled
	}
	if input.PublishPolicy != nil {
		item.PublishPolicy = *input.PublishPolicy
	}
	if input.ClassIDs != nil {
		item.ClassIDs = cloneStrings(*input.ClassIDs)
	}
	s.items[id] = item
	return item, nil
}

func (s *MemoryStore) UpdateStatus(_ context.Context, tenantID string, id string, status string) (Exam, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.items[id]
	if !ok || item.TenantID != tenantID {
		return Exam{}, ErrNotFound
	}
	if !CanTransition(item.Status, status) {
		return Exam{}, ErrInvalidTransition
	}
	item.Status = status
	s.items[id] = item
	return item, nil
}

func (s *MemoryStore) id() string {
	id := fmt.Sprintf("exam-%d", s.next)
	s.next++
	return id
}

func cloneStrings(in []string) []string {
	out := make([]string, len(in))
	copy(out, in)
	return out
}
