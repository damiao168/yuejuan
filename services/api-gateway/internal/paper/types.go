package paper

import (
	"context"
	"errors"
	"time"
)

var (
	ErrNotFound       = errors.New("paper resource not found")
	ErrInvalidInput   = errors.New("invalid paper input")
	ErrRubricMismatch = errors.New("rubric score does not equal question score")
	ErrRubricLocked   = errors.New("locked rubric cannot be modified")
	ErrTemplateLocked = errors.New("locked template cannot be modified")
	ErrConflict       = errors.New("configuration revision conflict")
	ErrNotReady       = errors.New("exam configuration is not ready")
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

type LayoutRegion struct {
	ID         string  `json:"id"`
	QuestionID string  `json:"question_id,omitempty"`
	Label      string  `json:"label,omitempty"`
	X          float64 `json:"x"`
	Y          float64 `json:"y"`
	Width      float64 `json:"width"`
	Height     float64 `json:"height"`
}

type TemplatePage struct {
	PageNo            int            `json:"page_no"`
	Width             int            `json:"width"`
	Height            int            `json:"height"`
	RegistrationMarks []LayoutRegion `json:"registration_marks"`
	IdentityRegions   []LayoutRegion `json:"identity_regions"`
	QuestionRegions   []LayoutRegion `json:"question_regions"`
}

type TemplateLayout struct {
	Pages []TemplatePage `json:"pages"`
}

type AnswerSheetTemplate struct {
	ID          string         `json:"id"`
	TenantID    string         `json:"tenant_id"`
	ExamID      string         `json:"exam_id"`
	ExamPaperID string         `json:"exam_paper_id"`
	VersionNo   int            `json:"version_no"`
	Revision    int            `json:"revision"`
	Name        string         `json:"name"`
	Status      string         `json:"status"`
	PageCount   int            `json:"page_count"`
	Layout      TemplateLayout `json:"layout"`
	ContentHash string         `json:"content_hash"`
	CreatedBy   string         `json:"created_by"`
	LockedBy    string         `json:"locked_by,omitempty"`
	LockedAt    *time.Time     `json:"locked_at,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

type CreateTemplateInput struct {
	ExamPaperID string         `json:"exam_paper_id"`
	Name        string         `json:"name"`
	PageCount   int            `json:"page_count"`
	Layout      TemplateLayout `json:"layout"`
}

type UpdateTemplateInput struct {
	Name             string         `json:"name"`
	PageCount        int            `json:"page_count"`
	Layout           TemplateLayout `json:"layout"`
	ExpectedRevision int            `json:"expected_revision"`
}

type ReadinessCheck struct {
	Code     string `json:"code"`
	Label    string `json:"label"`
	Passed   bool   `json:"passed"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
	Section  string `json:"section"`
}

type ReadinessResult struct {
	Ready             bool             `json:"ready"`
	Confirmed         bool             `json:"confirmed"`
	ConfigurationHash string           `json:"configuration_hash"`
	Checks            []ReadinessCheck `json:"checks"`
	ConfirmedAt       *time.Time       `json:"confirmed_at,omitempty"`
	ConfirmedBy       string           `json:"confirmed_by,omitempty"`
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
	ListTemplates(ctx context.Context, tenantID string, examID string) ([]AnswerSheetTemplate, error)
	CreateTemplate(ctx context.Context, tenantID string, examID string, userID string, input CreateTemplateInput) (AnswerSheetTemplate, error)
	UpdateTemplate(ctx context.Context, tenantID string, id string, input UpdateTemplateInput) (AnswerSheetTemplate, error)
	LockTemplate(ctx context.Context, tenantID string, id string, userID string) (AnswerSheetTemplate, error)
	CloneTemplate(ctx context.Context, tenantID string, id string, userID string) (AnswerSheetTemplate, error)
	Readiness(ctx context.Context, tenantID string, examID string) (ReadinessResult, error)
	ConfirmReadiness(ctx context.Context, tenantID string, examID string, userID string) (ReadinessResult, error)
	StartCollection(ctx context.Context, tenantID string, examID string, userID string) (ReadinessResult, error)
}
