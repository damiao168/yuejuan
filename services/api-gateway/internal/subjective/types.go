package subjective

import (
	"context"
	"errors"
	"time"

	"edugrade-enterprise/services/api-gateway/internal/grading"
	"edugrade-enterprise/services/api-gateway/internal/paper"
)

const (
	MockGraderType = "mock_llm_subjective"
	LLMGraderType  = "llm_subjective"
)

var (
	ErrNotFound                = errors.New("subjective grading resource not found")
	ErrInvalidInput            = errors.New("invalid subjective grading input")
	ErrUnsupportedQuestionType = errors.New("unsupported question type for subjective grading")
	ErrAnswerMissing           = errors.New("answer segment has no recorded answer")
	ErrRubricMissing           = errors.New("question has no rubric")
	ErrInvalidModelOutput      = errors.New("invalid model output")
)

type ModelPolicy struct {
	ModelVersion  string  `json:"model_version"`
	PromptVersion string  `json:"prompt_version"`
	MinConfidence float64 `json:"min_confidence"`
}

type GradeRequest struct {
	ModelPolicy ModelPolicy `json:"model_policy"`
}

type Context struct {
	SegmentID       string
	AnswerVersion   string
	Subject         string
	GradeLevel      string
	Question        paper.Question
	Rubric          paper.Rubric
	AnswerText      string
	AnswerImageRef  map[string]any
	OCRConfidence   *float64
	AnswerCreatedAt time.Time
}

type AdapterInput struct {
	RequestID      string         `json:"request_id"`
	SegmentID      string         `json:"answer_segment_id"`
	Subject        string         `json:"subject"`
	GradeLevel     string         `json:"grade_level"`
	Question       paper.Question `json:"question"`
	Rubric         paper.Rubric   `json:"rubric"`
	AnswerText     string         `json:"answer_text"`
	AnswerImageRef map[string]any `json:"answer_image_ref"`
	OCRConfidence  *float64       `json:"ocr_confidence,omitempty"`
	ModelPolicy    ModelPolicy    `json:"model_policy"`
	PromptGuard    PromptGuard    `json:"prompt_guard"`
}

type PromptGuard struct {
	StudentAnswerIsUntrusted bool     `json:"student_answer_is_untrusted"`
	SuspectedInjection       bool     `json:"suspected_injection"`
	Signals                  []string `json:"signals"`
	Instruction              string   `json:"instruction"`
}

type AdapterOutput struct {
	RequestID         string                `json:"request_id"`
	SuggestedScore    float64               `json:"suggested_score"`
	Confidence        float64               `json:"confidence"`
	MatchedPoints     []grading.PointResult `json:"matched_points"`
	MissingPoints     []grading.PointResult `json:"missing_points"`
	Evidence          []grading.Evidence    `json:"evidence"`
	RiskFlags         []string              `json:"risk_flags"`
	NeedsHumanReview  bool                  `json:"needs_human_review"`
	StudentFeedback   string                `json:"student_feedback"`
	TeacherNote       string                `json:"teacher_note"`
	ModelVersion      string                `json:"model_version"`
	PromptVersion     string                `json:"prompt_version"`
	RubricVersion     string                `json:"rubric_version"`
	DeliveryMode      string                `json:"delivery_mode"`
	CapabilityProfile string                `json:"capability_profile"`
	Telemetry         AdapterTelemetry      `json:"telemetry"`
	RawOutput         map[string]any        `json:"raw_output"`
	Mock              bool                  `json:"mock"`
}

type AdapterTelemetry struct {
	Adapter         string   `json:"adapter"`
	Provider        string   `json:"provider"`
	Deployment      string   `json:"deployment"`
	Region          string   `json:"region"`
	Attempts        int      `json:"attempts"`
	RepairAttempted bool     `json:"repair_attempted"`
	PriorErrorCodes []string `json:"prior_error_codes"`
	ElapsedMS       int64    `json:"elapsed_ms"`
}

type Grade struct {
	ID                     string                `json:"id"`
	TenantID               string                `json:"tenant_id"`
	AnswerSegmentID        string                `json:"answer_segment_id"`
	QuestionID             string                `json:"question_id"`
	QuestionNo             string                `json:"question_no"`
	QuestionType           string                `json:"question_type"`
	AnswerVersion          string                `json:"answer_version"`
	GraderType             string                `json:"grader_type"`
	ModelVersion           string                `json:"model_version"`
	PromptVersion          string                `json:"prompt_version"`
	RubricVersion          string                `json:"rubric_version"`
	DeliveryMode           string                `json:"delivery_mode"`
	CapabilityProfile      string                `json:"capability_profile"`
	AdapterRequestID       string                `json:"adapter_request_id"`
	AdapterName            string                `json:"adapter_name"`
	ProviderKey            string                `json:"provider_key"`
	DeploymentKey          string                `json:"deployment_key"`
	DeploymentRegion       string                `json:"deployment_region"`
	AdapterAttempts        int                   `json:"adapter_attempts"`
	AdapterLatencyMS       int64                 `json:"adapter_latency_ms"`
	AdapterRepairAttempted bool                  `json:"adapter_repair_attempted"`
	SuggestedScore         float64               `json:"suggested_score"`
	MaxScore               float64               `json:"max_score"`
	Confidence             float64               `json:"confidence"`
	MatchedPoints          []grading.PointResult `json:"matched_points"`
	MissingPoints          []grading.PointResult `json:"missing_points"`
	Evidence               []grading.Evidence    `json:"evidence"`
	RiskFlags              []string              `json:"risk_flags"`
	NeedsHumanReview       bool                  `json:"needs_human_review"`
	StudentFeedback        string                `json:"student_feedback"`
	TeacherNote            string                `json:"teacher_note"`
	Mock                   bool                  `json:"mock"`
	Status                 string                `json:"status"`
	FailureReason          string                `json:"failure_reason,omitempty"`
	RawOutput              map[string]any        `json:"raw_output"`
	CreatedBy              string                `json:"created_by"`
	CreatedAt              time.Time             `json:"created_at"`
}

type Store interface {
	LoadContext(ctx context.Context, tenantID string, segmentID string) (Context, error)
	CreateGrade(ctx context.Context, tenantID string, actorID string, grade Grade) (Grade, error)
}

type LLMGradingAdapter interface {
	Name() string
	Grade(ctx context.Context, input AdapterInput) (AdapterOutput, error)
}

type GovernedPolicyProvider interface {
	Policy() ModelPolicy
}
