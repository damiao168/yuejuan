package paper

import (
	"context"
	"strings"
	"time"
)

func memoryBindingKey(tenantID, examID string) string { return tenantID + ":" + examID }

func (s *MemoryStore) GetExamTemplateBinding(_ context.Context, tenantID, examID string) (ExamTemplateBinding, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	item, ok := s.bindings[memoryBindingKey(tenantID, examID)]
	if !ok {
		return ExamTemplateBinding{}, ErrNotFound
	}
	return item, nil
}

func (s *MemoryStore) BindExamTemplate(_ context.Context, tenantID, examID, userID string, input BindExamTemplateInput) (ExamTemplateBinding, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	mode, ok := normalizeBindingMode(input.Mode)
	if !ok || input.TemplateID == "" {
		return ExamTemplateBinding{}, ErrInvalidInput
	}
	template, ok := s.templates[input.TemplateID]
	if !ok || template.TenantID != tenantID || template.ExamID != examID {
		return ExamTemplateBinding{}, ErrNotFound
	}
	if template.Status != "locked" {
		return ExamTemplateBinding{}, ErrTemplateNotLocked
	}
	key := memoryBindingKey(tenantID, examID)
	current, exists := s.bindings[key]
	if (!exists && input.ExpectedRevision != 0) || (exists && current.Revision != input.ExpectedRevision) {
		return ExamTemplateBinding{}, ErrConflict
	}
	now := time.Now().UTC()
	if !exists {
		current = ExamTemplateBinding{ID: s.id("template-binding"), TenantID: tenantID, ExamID: examID, Revision: 1}
	} else {
		current.Revision++
	}
	current.TemplateID = template.ID
	current.TemplateContentHash = template.ContentHash
	current.Mode = mode
	current.Source = "manual"
	current.BoundBy = userID
	current.BoundAt = now
	current.UpdatedAt = now
	s.bindings[key] = current
	return current, nil
}

func (s *MemoryStore) UnbindExamTemplate(_ context.Context, tenantID, examID string, input UnbindExamTemplateInput) (ExamTemplateBinding, error) {
	if strings.TrimSpace(input.Reason) == "" {
		return ExamTemplateBinding{}, ErrInvalidInput
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	key := memoryBindingKey(tenantID, examID)
	current, ok := s.bindings[key]
	if !ok {
		return ExamTemplateBinding{}, ErrNotFound
	}
	if input.ExpectedRevision < 1 || current.Revision != input.ExpectedRevision {
		return ExamTemplateBinding{}, ErrConflict
	}
	delete(s.bindings, key)
	return current, nil
}
