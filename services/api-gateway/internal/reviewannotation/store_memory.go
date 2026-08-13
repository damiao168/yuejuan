package reviewannotation

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

type MemoryStore struct {
	mu                         sync.RWMutex
	next                       int
	annotations                map[string]Annotation
	templates                  map[string]CommentTemplate
	taskRefs                   map[string]memoryTaskReference
	published                  map[string]bool
	studentQuestionAnnotations map[string][]StudentAnnotation
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		next: 1, annotations: map[string]Annotation{}, templates: map[string]CommentTemplate{},
		taskRefs: map[string]memoryTaskReference{}, published: map[string]bool{},
		studentQuestionAnnotations: map[string][]StudentAnnotation{},
	}
}

type memoryTaskReference struct {
	submissionID, answerSegmentID, submissionPageID string
}

// SetTaskReference and SetSubmissionPublished seed dependencies owned by other
// domains when MemoryStore is used by an integrated in-memory server or tests.
func (s *MemoryStore) SetTaskReference(tenantID, taskID, submissionID, answerSegmentID, submissionPageID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.taskRefs[memoryKey(tenantID, taskID)] = memoryTaskReference{submissionID, answerSegmentID, submissionPageID}
}

func (s *MemoryStore) SetSubmissionPublished(tenantID, submissionID string, published bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.published[memoryKey(tenantID, submissionID)] = published
}

// SetStudentQuestionAnnotations is an in-memory fixture seam for the
// immutable release projection that the PostgreSQL implementation joins at
// read time. It accepts internal annotations but persists only the safe DTO,
// so unit/router tests cannot accidentally make a private annotation visible.
func (s *MemoryStore) SetStudentQuestionAnnotations(tenantID, examID, studentID, questionID string, annotations []Annotation) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.studentQuestionAnnotations[studentQuestionKey(tenantID, examID, studentID, questionID)] = cloneStudentAnnotations(StudentAnnotations(annotations))
}

func (s *MemoryStore) CreateAnnotation(_ context.Context, tenantID, reviewTaskID, actorID string, input CreateAnnotationInput) (Annotation, error) {
	input = normalizeAnnotationInput(input)
	if tenantID == "" || reviewTaskID == "" || actorID == "" || validateAnnotationInput(input) != nil {
		return Annotation{}, ErrInvalidInput
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	reference, ok := s.taskRefs[memoryKey(tenantID, reviewTaskID)]
	if !ok {
		reference = memoryTaskReference{reviewTaskID, reviewTaskID, reviewTaskID}
	}
	item := Annotation{
		ID: s.id("review-annotation"), TenantID: tenantID, ReviewTaskID: reviewTaskID,
		AnswerSegmentID: reference.answerSegmentID, SubmissionPageID: reference.submissionPageID,
		Type: input.Type, Geometry: input.Geometry, Payload: clonePayload(input.Payload),
		Content: input.Content, Visibility: input.Visibility, Revision: 1,
		CreatedBy: actorID, UpdatedBy: actorID, CreatedAt: now, UpdatedAt: now,
	}
	s.annotations[memoryKey(tenantID, item.ID)] = item
	return cloneAnnotation(item), nil
}

func (s *MemoryStore) ListStudentAnnotations(_ context.Context, tenantID, submissionID string) ([]StudentAnnotation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.published[memoryKey(tenantID, submissionID)] {
		return []StudentAnnotation{}, nil
	}
	items := []Annotation{}
	for _, item := range s.annotations {
		reference := s.taskRefs[memoryKey(tenantID, item.ReviewTaskID)]
		if item.TenantID == tenantID && reference.submissionID == submissionID {
			items = append(items, cloneAnnotation(item))
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.Before(items[j].CreatedAt) })
	return StudentAnnotations(items), nil
}

func (s *MemoryStore) ListStudentQuestionAnnotations(_ context.Context, tenantID, examID, studentID, questionID string) ([]StudentAnnotation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneStudentAnnotations(s.studentQuestionAnnotations[studentQuestionKey(tenantID, examID, studentID, questionID)]), nil
}

func (s *MemoryStore) ListAnnotations(_ context.Context, tenantID, reviewTaskID string) ([]Annotation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []Annotation{}
	for _, item := range s.annotations {
		if item.TenantID == tenantID && item.ReviewTaskID == reviewTaskID {
			out = append(out, cloneAnnotation(item))
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].ID < out[j].ID
		}
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out, nil
}

func (s *MemoryStore) GetAnnotation(_ context.Context, tenantID, id string) (Annotation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	item, ok := s.annotations[memoryKey(tenantID, id)]
	if !ok {
		return Annotation{}, ErrNotFound
	}
	return cloneAnnotation(item), nil
}

func (s *MemoryStore) UpdateAnnotation(_ context.Context, tenantID, id, actorID string, input UpdateAnnotationInput) (Annotation, error) {
	normalized := normalizeAnnotationInput(CreateAnnotationInput{
		Type: input.Type, Geometry: input.Geometry, Payload: input.Payload,
		Content: input.Content, Visibility: input.Visibility,
	})
	if tenantID == "" || id == "" || actorID == "" || input.ExpectedRevision <= 0 || validateAnnotationInput(normalized) != nil {
		return Annotation{}, ErrInvalidInput
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	key := memoryKey(tenantID, id)
	item, ok := s.annotations[key]
	if !ok {
		return Annotation{}, ErrNotFound
	}
	if item.Revision != input.ExpectedRevision {
		return Annotation{}, ErrRevisionConflict
	}
	item.Type, item.Geometry = normalized.Type, normalized.Geometry
	item.Payload, item.Content, item.Visibility = clonePayload(normalized.Payload), normalized.Content, normalized.Visibility
	item.Revision++
	item.UpdatedBy, item.UpdatedAt = actorID, time.Now().UTC()
	s.annotations[key] = item
	return cloneAnnotation(item), nil
}

func (s *MemoryStore) DeleteAnnotation(_ context.Context, tenantID, id, _ string, expectedRevision int64) error {
	if tenantID == "" || id == "" || expectedRevision <= 0 {
		return ErrInvalidInput
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	key := memoryKey(tenantID, id)
	item, ok := s.annotations[key]
	if !ok {
		return ErrNotFound
	}
	if item.Revision != expectedRevision {
		return ErrRevisionConflict
	}
	delete(s.annotations, key)
	return nil
}

func (s *MemoryStore) CreateCommentTemplate(_ context.Context, tenantID, actorID string, input CreateCommentTemplateInput) (CommentTemplate, error) {
	input.Title, input.Content, input.Shortcut = normalizeTemplate(input.Title, input.Content, input.Shortcut)
	if tenantID == "" || actorID == "" || validateTemplate(input.Title, input.Content, input.Shortcut) != nil {
		return CommentTemplate{}, ErrInvalidInput
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.shortcutExists(tenantID, actorID, input.Shortcut, "") {
		return CommentTemplate{}, ErrShortcutConflict
	}
	now := time.Now().UTC()
	item := CommentTemplate{
		ID: s.id("comment-template"), TenantID: tenantID, OwnerID: actorID,
		Title: input.Title, Content: input.Content, Shortcut: input.Shortcut,
		UsageCount: 0, Revision: 1, CreatedAt: now, UpdatedAt: now,
	}
	s.templates[memoryKey(tenantID, item.ID)] = item
	return item, nil
}

func (s *MemoryStore) ListCommentTemplates(_ context.Context, tenantID, ownerID string) ([]CommentTemplate, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []CommentTemplate{}
	for _, item := range s.templates {
		if item.TenantID == tenantID && item.OwnerID == ownerID {
			out = append(out, item)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].UsageCount == out[j].UsageCount {
			return out[i].UpdatedAt.After(out[j].UpdatedAt)
		}
		return out[i].UsageCount > out[j].UsageCount
	})
	return out, nil
}

func (s *MemoryStore) GetCommentTemplate(_ context.Context, tenantID, ownerID, id string) (CommentTemplate, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	item, ok := s.templates[memoryKey(tenantID, id)]
	if !ok || item.OwnerID != ownerID {
		return CommentTemplate{}, ErrNotFound
	}
	return item, nil
}

func (s *MemoryStore) UpdateCommentTemplate(_ context.Context, tenantID, ownerID, id string, input UpdateCommentTemplateInput) (CommentTemplate, error) {
	input.Title, input.Content, input.Shortcut = normalizeTemplate(input.Title, input.Content, input.Shortcut)
	if tenantID == "" || ownerID == "" || id == "" || input.ExpectedRevision <= 0 || validateTemplate(input.Title, input.Content, input.Shortcut) != nil {
		return CommentTemplate{}, ErrInvalidInput
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	key := memoryKey(tenantID, id)
	item, ok := s.templates[key]
	if !ok || item.OwnerID != ownerID {
		return CommentTemplate{}, ErrNotFound
	}
	if item.Revision != input.ExpectedRevision {
		return CommentTemplate{}, ErrRevisionConflict
	}
	if s.shortcutExists(tenantID, ownerID, input.Shortcut, id) {
		return CommentTemplate{}, ErrShortcutConflict
	}
	item.Title, item.Content, item.Shortcut = input.Title, input.Content, input.Shortcut
	item.Revision++
	item.UpdatedAt = time.Now().UTC()
	s.templates[key] = item
	return item, nil
}

func (s *MemoryStore) DeleteCommentTemplate(_ context.Context, tenantID, ownerID, id string, expectedRevision int64) error {
	if tenantID == "" || ownerID == "" || id == "" || expectedRevision <= 0 {
		return ErrInvalidInput
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	key := memoryKey(tenantID, id)
	item, ok := s.templates[key]
	if !ok || item.OwnerID != ownerID {
		return ErrNotFound
	}
	if item.Revision != expectedRevision {
		return ErrRevisionConflict
	}
	delete(s.templates, key)
	return nil
}

func (s *MemoryStore) UseCommentTemplate(_ context.Context, tenantID, ownerID, shortcut string) (CommentTemplate, error) {
	_, _, shortcut = normalizeTemplate("", "", shortcut)
	s.mu.Lock()
	defer s.mu.Unlock()
	for key, item := range s.templates {
		if item.TenantID == tenantID && item.OwnerID == ownerID && item.Shortcut == shortcut {
			item.UsageCount++
			item.Revision++
			item.UpdatedAt = time.Now().UTC()
			s.templates[key] = item
			return item, nil
		}
	}
	return CommentTemplate{}, ErrNotFound
}

func (s *MemoryStore) shortcutExists(tenantID, ownerID, shortcut, exceptID string) bool {
	for _, item := range s.templates {
		if item.TenantID == tenantID && item.OwnerID == ownerID && item.Shortcut == shortcut && item.ID != exceptID {
			return true
		}
	}
	return false
}

func (s *MemoryStore) id(prefix string) string {
	id := fmt.Sprintf("%s-%d", prefix, s.next)
	s.next++
	return id
}

func memoryKey(tenantID, id string) string { return tenantID + "\x00" + id }

func studentQuestionKey(tenantID, examID, studentID, questionID string) string {
	return tenantID + "\x00" + examID + "\x00" + studentID + "\x00" + questionID
}

func cloneAnnotation(item Annotation) Annotation {
	item.Payload = clonePayload(item.Payload)
	return item
}

func cloneStudentAnnotations(items []StudentAnnotation) []StudentAnnotation {
	return append([]StudentAnnotation(nil), items...)
}
