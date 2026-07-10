package subjective

import (
	"context"
	"fmt"
	"sync"
	"time"

	"edugrade-enterprise/services/api-gateway/internal/grading"
	"edugrade-enterprise/services/api-gateway/internal/paper"
)

type MemoryStore struct {
	mu       sync.RWMutex
	next     int
	contexts map[string]Context
	grades   map[string][]Grade
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{next: 1, contexts: map[string]Context{}, grades: map[string][]Grade{}}
}

func (s *MemoryStore) AddContext(tenantID string, segmentID string, ctx Context) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx.SegmentID = segmentID
	s.contexts[key(tenantID, segmentID)] = ctx
}

func (s *MemoryStore) LoadContext(_ context.Context, tenantID string, segmentID string) (Context, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ctx, ok := s.contexts[key(tenantID, segmentID)]
	if !ok {
		return Context{}, ErrNotFound
	}
	if ctx.AnswerText == "" {
		return Context{}, ErrAnswerMissing
	}
	if ctx.Rubric.ID == "" {
		return Context{}, ErrRubricMissing
	}
	return ctx, nil
}

func (s *MemoryStore) CreateGrade(_ context.Context, tenantID string, actorID string, grade Grade) (Grade, error) {
	if grade.AnswerSegmentID == "" || grade.QuestionID == "" {
		return Grade{}, ErrInvalidInput
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	grade.ID = s.id("ai-grade")
	grade.TenantID = tenantID
	grade.CreatedBy = actorID
	grade.CreatedAt = time.Now().UTC()
	grade.RawOutput = cloneMap(grade.RawOutput)
	grade.MatchedPoints = clonePoints(grade.MatchedPoints)
	grade.MissingPoints = clonePoints(grade.MissingPoints)
	grade.Evidence = cloneEvidence(grade.Evidence)
	grade.RiskFlags = cloneStrings(grade.RiskFlags)
	s.grades[key(tenantID, grade.AnswerSegmentID)] = append(s.grades[key(tenantID, grade.AnswerSegmentID)], grade)
	return grade, nil
}

func ContextForTest(kind string, score float64, answerText string, ocrConfidence *float64) Context {
	return Context{
		Question: paper.Question{
			ID:           "question-" + kind,
			TenantID:     "00000000-0000-0000-0000-000000000002",
			ExamID:       "exam-1",
			QuestionNo:   "Q1",
			QuestionType: kind,
			Score:        score,
		},
		Rubric: paper.Rubric{
			ID:         "rubric-" + kind,
			QuestionID: "question-" + kind,
			Version:    "v1",
			Status:     "approved",
			MaxScore:   score,
			Points: []paper.RubricPoint{
				{ID: "p1", Description: "synthetic rubric point", Score: score, Required: true},
			},
		},
		AnswerText:     answerText,
		AnswerImageRef: map[string]any{"answer_segment_id": "segment-1"},
		OCRConfidence:  ocrConfidence,
	}
}

func key(tenantID string, segmentID string) string {
	return tenantID + "|" + segmentID
}

func (s *MemoryStore) id(prefix string) string {
	id := fmt.Sprintf("%s-%d", prefix, s.next)
	s.next++
	return id
}

func cloneMap(in map[string]any) map[string]any {
	if in == nil {
		return map[string]any{}
	}
	out := map[string]any{}
	for key, value := range in {
		out[key] = value
	}
	return out
}

func cloneStrings(in []string) []string {
	out := make([]string, len(in))
	copy(out, in)
	return out
}

func clonePoints(in []grading.PointResult) []grading.PointResult {
	out := make([]grading.PointResult, len(in))
	copy(out, in)
	return out
}

func cloneEvidence(in []grading.Evidence) []grading.Evidence {
	out := make([]grading.Evidence, len(in))
	copy(out, in)
	return out
}
