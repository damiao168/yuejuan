package paper

import (
	"context"
	"errors"
)

var (
	ErrNotFound       = errors.New("paper resource not found")
	ErrInvalidInput   = errors.New("invalid paper input")
	ErrRubricMismatch = errors.New("rubric score does not equal question score")
	ErrRubricLocked   = errors.New("locked rubric cannot be modified")
)

type FileAssetInput struct {
	OriginalName  string `json:"original_name"`
	ContentType   string `json:"content_type"`
	SizeBytes     int64  `json:"size_bytes"`
	HashSHA256    string `json:"hash_sha256"`
	StorageBucket string `json:"storage_bucket"`
	StorageKey    string `json:"storage_key"`
}

type Paper struct {
	ID          string         `json:"id"`
	TenantID    string         `json:"tenant_id"`
	ExamID      string         `json:"exam_id"`
	FileAssetID string         `json:"file_asset_id"`
	VersionNo   int            `json:"version_no"`
	Status      string         `json:"status"`
	File        FileAssetInput `json:"file"`
}

type CreatePaperInput struct {
	FileAssetID string         `json:"file_asset_id"`
	File        FileAssetInput `json:"file"`
}

type AnswerKeyInput struct {
	StandardAnswer    any   `json:"standard_answer"`
	EquivalentAnswers []any `json:"equivalent_answers"`
	Tolerance         any   `json:"tolerance"`
}

type Question struct {
	ID              string         `json:"id"`
	TenantID        string         `json:"tenant_id"`
	ExamID          string         `json:"exam_id"`
	ExamPaperID     string         `json:"exam_paper_id,omitempty"`
	QuestionNo      string         `json:"question_no"`
	QuestionType    string         `json:"question_type"`
	Score           float64        `json:"score"`
	Stem            string         `json:"stem,omitempty"`
	KnowledgePoints []string       `json:"knowledge_points"`
	AnswerArea      map[string]any `json:"answer_area,omitempty"`
	SortOrder       int            `json:"sort_order"`
	Status          string         `json:"status"`
	AnswerKey       *AnswerKey     `json:"answer_key,omitempty"`
	Rubric          *Rubric        `json:"rubric,omitempty"`
}

type AnswerKey struct {
	ID                string `json:"id"`
	QuestionID        string `json:"question_id"`
	AnswerVersion     string `json:"answer_version"`
	StandardAnswer    any    `json:"standard_answer"`
	EquivalentAnswers []any  `json:"equivalent_answers"`
	Tolerance         any    `json:"tolerance"`
}

type CreateQuestionInput struct {
	ExamPaperID     string          `json:"exam_paper_id"`
	QuestionNo      string          `json:"question_no"`
	QuestionType    string          `json:"question_type"`
	Score           float64         `json:"score"`
	Stem            string          `json:"stem"`
	KnowledgePoints []string        `json:"knowledge_points"`
	AnswerArea      map[string]any  `json:"answer_area"`
	SortOrder       int             `json:"sort_order"`
	AnswerKey       *AnswerKeyInput `json:"answer_key"`
}

type UpdateQuestionInput struct {
	QuestionNo      *string         `json:"question_no"`
	QuestionType    *string         `json:"question_type"`
	Score           *float64        `json:"score"`
	Stem            *string         `json:"stem"`
	KnowledgePoints *[]string       `json:"knowledge_points"`
	AnswerArea      *map[string]any `json:"answer_area"`
	SortOrder       *int            `json:"sort_order"`
	AnswerKey       *AnswerKeyInput `json:"answer_key"`
}

type RubricPoint struct {
	ID          string  `json:"id"`
	Description string  `json:"description"`
	Score       float64 `json:"score"`
	Required    bool    `json:"required"`
}

type Rubric struct {
	ID         string        `json:"id"`
	QuestionID string        `json:"question_id"`
	Version    string        `json:"version"`
	Status     string        `json:"status"`
	MaxScore   float64       `json:"max_score"`
	Points     []RubricPoint `json:"points"`
	Deductions []any         `json:"deductions"`
	Examples   []any         `json:"examples"`
}

type RubricInput struct {
	Status     string        `json:"status"`
	MaxScore   float64       `json:"max_score"`
	Points     []RubricPoint `json:"points"`
	Deductions []any         `json:"deductions"`
	Examples   []any         `json:"examples"`
}

type ValidationIssue struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ValidationResult struct {
	Valid  bool              `json:"valid"`
	Issues []ValidationIssue `json:"issues"`
}

type Store interface {
	CreatePaper(ctx context.Context, tenantID string, examID string, userID string, input CreatePaperInput) (Paper, error)
	ListPapers(ctx context.Context, tenantID string, examID string) ([]Paper, error)
	CreateQuestion(ctx context.Context, tenantID string, examID string, userID string, input CreateQuestionInput) (Question, error)
	ListQuestions(ctx context.Context, tenantID string, examID string) ([]Question, error)
	UpdateQuestion(ctx context.Context, tenantID string, id string, userID string, input UpdateQuestionInput) (Question, error)
	DeleteQuestion(ctx context.Context, tenantID string, id string) error
	CreateRubric(ctx context.Context, tenantID string, questionID string, userID string, input RubricInput) (Rubric, error)
	ValidateConfig(ctx context.Context, tenantID string, examID string) (ValidationResult, error)
}
