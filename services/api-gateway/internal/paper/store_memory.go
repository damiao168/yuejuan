package paper

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type MemoryStore struct {
	mu         sync.RWMutex
	next       int
	examTotals map[string]float64
	papers     map[string]Paper
	questions  map[string]Question
	rubrics    map[string][]Rubric
	templates  map[string]AnswerSheetTemplate
	readiness  map[string]ReadinessResult
	imports    map[string]PaperImportJob
	examState  map[string]memoryExamState
}

type memoryExamState struct {
	Total        float64
	ClassCount   int
	StudentCount int
	Status       string
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		next:       1,
		examTotals: map[string]float64{},
		papers:     map[string]Paper{},
		questions:  map[string]Question{},
		rubrics:    map[string][]Rubric{},
		templates:  map[string]AnswerSheetTemplate{},
		readiness:  map[string]ReadinessResult{},
		imports:    map[string]PaperImportJob{},
		examState:  map[string]memoryExamState{},
	}
}

func (s *MemoryStore) CreatePaperImport(_ context.Context, tenantID, examID, userID string, input CreatePaperImportInput) (PaperImportJob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.papers[input.ExamPaperID]
	if !ok || p.TenantID != tenantID || p.ExamID != examID || input.PaperFileAssetID == "" || input.AnswerFileAssetID == "" {
		return PaperImportJob{}, ErrInvalidInput
	}
	now := time.Now().UTC()
	job := PaperImportJob{ID: s.id("paper-import"), TenantID: tenantID, ExamID: examID, ExamPaperID: input.ExamPaperID, PaperFileAssetID: input.PaperFileAssetID, AnswerFileAssetID: input.AnswerFileAssetID, Status: "processing", Subject: input.Subject, Questions: []PaperImportDraftQuestion{}, Issues: []string{}, CreatedBy: userID, CreatedAt: now, UpdatedAt: now}
	s.imports[job.ID] = job
	return job, nil
}

func (s *MemoryStore) CompletePaperImport(_ context.Context, tenantID, id string, questions []PaperImportDraftQuestion, issues []string) (PaperImportJob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.imports[id]
	if !ok || job.TenantID != tenantID {
		return PaperImportJob{}, ErrNotFound
	}
	job.Status, job.Questions, job.Issues, job.UpdatedAt = "review_required", questions, issues, time.Now().UTC()
	s.imports[id] = job
	return job, nil
}

func (s *MemoryStore) FailPaperImport(_ context.Context, tenantID, id, code string, issues []string) (PaperImportJob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.imports[id]
	if !ok || job.TenantID != tenantID {
		return PaperImportJob{}, ErrNotFound
	}
	job.Status, job.ErrorCode, job.Issues, job.UpdatedAt = "failed", code, issues, time.Now().UTC()
	s.imports[id] = job
	return job, nil
}

func (s *MemoryStore) GetPaperImport(_ context.Context, tenantID, id string) (PaperImportJob, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	job, ok := s.imports[id]
	if !ok || job.TenantID != tenantID {
		return PaperImportJob{}, ErrNotFound
	}
	return job, nil
}

func (s *MemoryStore) ListPaperImports(_ context.Context, tenantID, examID string) ([]PaperImportJob, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []PaperImportJob{}
	for _, job := range s.imports {
		if job.TenantID == tenantID && job.ExamID == examID {
			out = append(out, job)
		}
	}
	return out, nil
}

func (s *MemoryStore) ApplyPaperImport(_ context.Context, tenantID, id, userID string) (PaperImportJob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.imports[id]
	if !ok || job.TenantID != tenantID {
		return PaperImportJob{}, ErrNotFound
	}
	if job.Status != "review_required" {
		return PaperImportJob{}, ErrConflict
	}
	for i, draft := range job.Questions {
		if err := validateQuestionInput(draft.QuestionNo, draft.QuestionType, draft.Score); err != nil {
			return PaperImportJob{}, ErrInvalidInput
		}
		if draft.Rubric != nil && (!scoreEqual(SumRubricPoints(draft.Rubric.Points), draft.Score) || !scoreEqual(draft.Rubric.MaxScore, draft.Score)) {
			return PaperImportJob{}, ErrRubricMismatch
		}
		qid := s.id("question")
		q := Question{ID: qid, TenantID: tenantID, ExamID: job.ExamID, ExamPaperID: job.ExamPaperID, QuestionNo: draft.QuestionNo, QuestionType: draft.QuestionType, Score: draft.Score, Stem: draft.Stem, KnowledgePoints: cloneStrings(draft.KnowledgePoints), AnswerArea: map[string]any{}, SortOrder: i + 1, Status: "active"}
		if draft.AnswerKey != nil {
			q.AnswerKey = &AnswerKey{ID: s.id("answer"), QuestionID: qid, AnswerVersion: "v1", StandardAnswer: draft.AnswerKey.StandardAnswer, EquivalentAnswers: draft.AnswerKey.EquivalentAnswers, Tolerance: draft.AnswerKey.Tolerance}
		}
		s.questions[qid] = q
		if draft.Rubric != nil {
			r := Rubric{ID: s.id("rubric"), QuestionID: qid, Version: "v1", Status: "draft", MaxScore: draft.Rubric.MaxScore, Points: draft.Rubric.Points, Deductions: draft.Rubric.Deductions, Examples: draft.Rubric.Examples}
			s.rubrics[qid] = []Rubric{r}
		}
	}
	now := time.Now().UTC()
	job.Status, job.AppliedAt, job.UpdatedAt = "applied", &now, now
	s.imports[id] = job
	_ = userID
	return job, nil
}

func (s *MemoryStore) SetExamTotal(examID string, total float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.examTotals[examID] = total
	state := s.examState[examID]
	state.Total = total
	if state.Status == "" {
		state.Status = "configured"
	}
	s.examState[examID] = state
}

func (s *MemoryStore) SetReadinessContext(examID string, total float64, classCount int, studentCount int, status string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.examTotals[examID] = total
	s.examState[examID] = memoryExamState{Total: total, ClassCount: classCount, StudentCount: studentCount, Status: status}
}

func (s *MemoryStore) ExamStatus(examID string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.examState[examID].Status
}

func (s *MemoryStore) CreatePaper(_ context.Context, tenantID string, examID string, _ string, input CreatePaperInput) (Paper, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	version := 1
	for _, item := range s.papers {
		if item.TenantID == tenantID && item.ExamID == examID && item.VersionNo >= version {
			version = item.VersionNo + 1
		}
	}
	paper := Paper{
		ID:          s.id("paper"),
		TenantID:    tenantID,
		ExamID:      examID,
		FileAssetID: input.FileAssetID,
		VersionNo:   version,
		Status:      "uploaded",
		File:        input.File,
	}
	if paper.FileAssetID == "" {
		paper.FileAssetID = s.id("file")
	}
	s.papers[paper.ID] = paper
	return paper, nil
}

func (s *MemoryStore) ListPapers(_ context.Context, tenantID string, examID string) ([]Paper, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []Paper{}
	for _, item := range s.papers {
		if item.TenantID == tenantID && item.ExamID == examID {
			out = append(out, item)
		}
	}
	return out, nil
}

func (s *MemoryStore) CreateQuestion(_ context.Context, tenantID string, examID string, _ string, input CreateQuestionInput) (Question, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if input.ExamPaperID != "" {
		paper, ok := s.papers[input.ExamPaperID]
		if !ok || paper.TenantID != tenantID || paper.ExamID != examID {
			return Question{}, ErrNotFound
		}
	}
	question := Question{
		ID:              s.id("question"),
		TenantID:        tenantID,
		ExamID:          examID,
		ExamPaperID:     input.ExamPaperID,
		QuestionNo:      input.QuestionNo,
		QuestionType:    input.QuestionType,
		Score:           input.Score,
		Stem:            input.Stem,
		KnowledgePoints: cloneStrings(input.KnowledgePoints),
		AnswerArea:      cloneMap(input.AnswerArea),
		SortOrder:       input.SortOrder,
		Status:          "active",
	}
	if input.AnswerKey != nil {
		question.AnswerKey = &AnswerKey{
			ID:                s.id("answer"),
			QuestionID:        question.ID,
			AnswerVersion:     "v1",
			StandardAnswer:    input.AnswerKey.StandardAnswer,
			EquivalentAnswers: input.AnswerKey.EquivalentAnswers,
			Tolerance:         input.AnswerKey.Tolerance,
		}
	}
	s.questions[question.ID] = question
	return question, nil
}

func (s *MemoryStore) ListQuestions(_ context.Context, tenantID string, examID string) ([]Question, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []Question{}
	for _, item := range s.questions {
		if item.TenantID == tenantID && item.ExamID == examID && item.Status != "deleted" {
			if rubrics := s.rubrics[item.ID]; len(rubrics) > 0 {
				latest := rubrics[len(rubrics)-1]
				item.Rubric = &latest
			}
			out = append(out, item)
		}
	}
	return out, nil
}

func (s *MemoryStore) UpdateQuestion(_ context.Context, tenantID string, id string, _ string, input UpdateQuestionInput) (Question, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.questions[id]
	if !ok || item.TenantID != tenantID || item.Status == "deleted" {
		return Question{}, ErrNotFound
	}
	if input.QuestionNo != nil {
		item.QuestionNo = *input.QuestionNo
	}
	if input.QuestionType != nil {
		item.QuestionType = *input.QuestionType
	}
	if input.Score != nil {
		item.Score = *input.Score
	}
	if input.Stem != nil {
		item.Stem = *input.Stem
	}
	if input.KnowledgePoints != nil {
		item.KnowledgePoints = cloneStrings(*input.KnowledgePoints)
	}
	if input.AnswerArea != nil {
		item.AnswerArea = cloneMap(*input.AnswerArea)
	}
	if input.SortOrder != nil {
		item.SortOrder = *input.SortOrder
	}
	if input.AnswerKey != nil {
		item.AnswerKey = &AnswerKey{ID: s.id("answer"), QuestionID: item.ID, AnswerVersion: "v2", StandardAnswer: input.AnswerKey.StandardAnswer, EquivalentAnswers: input.AnswerKey.EquivalentAnswers, Tolerance: input.AnswerKey.Tolerance}
	}
	s.questions[id] = item
	return item, nil
}

func (s *MemoryStore) DeleteQuestion(_ context.Context, tenantID string, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.questions[id]
	if !ok || item.TenantID != tenantID {
		return ErrNotFound
	}
	item.Status = "deleted"
	s.questions[id] = item
	return nil
}

func (s *MemoryStore) CreateRubric(_ context.Context, tenantID string, questionID string, _ string, input RubricInput) (Rubric, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	question, ok := s.questions[questionID]
	if !ok || question.TenantID != tenantID || question.Status == "deleted" {
		return Rubric{}, ErrNotFound
	}
	existing := s.rubrics[questionID]
	if len(existing) > 0 && existing[len(existing)-1].Status == "locked" {
		return Rubric{}, ErrRubricLocked
	}
	if !ValidRubricEvidenceRequirements(input.Points) {
		return Rubric{}, ErrInvalidInput
	}
	if !scoreEqual(SumRubricPoints(input.Points), question.Score) || !scoreEqual(input.MaxScore, question.Score) {
		return Rubric{}, ErrRubricMismatch
	}
	status := input.Status
	if status == "" {
		status = "draft"
	}
	rubric := Rubric{
		ID:         s.id("rubric"),
		QuestionID: questionID,
		Version:    fmt.Sprintf("v%d", len(existing)+1),
		Status:     status,
		MaxScore:   input.MaxScore,
		Points:     input.Points,
		Deductions: input.Deductions,
		Examples:   input.Examples,
	}
	s.rubrics[questionID] = append(existing, rubric)
	return rubric, nil
}

func (s *MemoryStore) ValidateConfig(_ context.Context, tenantID string, examID string) (ValidationResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := ValidationResult{Valid: true, Issues: []ValidationIssue{}}
	total := 0.0
	questionCount := 0
	for _, question := range s.questions {
		if question.TenantID != tenantID || question.ExamID != examID || question.Status == "deleted" {
			continue
		}
		questionCount++
		total += question.Score
		if question.QuestionType == "" || question.QuestionNo == "" {
			result.Issues = append(result.Issues, ValidationIssue{Code: "question_incomplete", Message: "question is missing question_no or question_type"})
		}
		if question.AnswerArea == nil {
			result.Issues = append(result.Issues, ValidationIssue{Code: "answer_area_missing", Message: "question " + question.QuestionNo + " is missing answer_area"})
		}
	}
	if questionCount == 0 {
		result.Issues = append(result.Issues, ValidationIssue{Code: "no_questions", Message: "exam has no questions"})
	}
	if expected, ok := s.examTotals[examID]; ok && !scoreEqual(total, expected) {
		result.Issues = append(result.Issues, ValidationIssue{Code: "total_score_mismatch", Message: fmt.Sprintf("question total %.2f does not equal exam total %.2f", total, expected)})
	}
	result.Valid = len(result.Issues) == 0
	return result, nil
}

func (s *MemoryStore) id(prefix string) string {
	id := fmt.Sprintf("%s-%d", prefix, s.next)
	s.next++
	return id
}

func cloneStrings(in []string) []string {
	out := make([]string, len(in))
	copy(out, in)
	return out
}

func cloneMap(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := map[string]any{}
	for key, value := range in {
		out[key] = value
	}
	return out
}
