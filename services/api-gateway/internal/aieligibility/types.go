// Package aieligibility is the server-side admission control for any external
// AI grading call.  It is deliberately separate from subjective: the latter
// executes a request only after this package has recorded a decision.
package aieligibility

import (
	"context"
	"errors"
	"time"

	"edugrade-enterprise/services/api-gateway/internal/assessment"
)

var (
	ErrNotFound       = errors.New("AI eligibility resource not found")
	ErrInvalidInput   = errors.New("invalid AI eligibility input")
	ErrPolicyConflict = errors.New("AI eligibility policy version conflict")
)

type PolicyStatus string

const (
	PolicyActive   PolicyStatus = "active"
	PolicyDisabled PolicyStatus = "disabled"
)

func (s PolicyStatus) Valid() bool { return s == PolicyActive || s == PolicyDisabled }

// Policy is a versioned tenant-owned policy for one assessment axis.  A
// policy never names a model: model-specific empirical evidence belongs in
// DecisionInput so each actual scoring run remains auditable.
type Policy struct {
	ID                 string                    `json:"id"`
	TenantID           string                    `json:"tenant_id"`
	SubjectCode        assessment.SubjectCode    `json:"subject_code"`
	EducationStage     assessment.EducationStage `json:"education_stage"`
	ArchetypeCode      string                    `json:"archetype_code"`
	RiskTier           assessment.RiskTier       `json:"risk_tier"`
	MinOCRQuality      float64                   `json:"min_ocr_quality"`
	MinParserQuality   float64                   `json:"min_parser_quality"`
	MinEvalN           int                       `json:"min_eval_n"`
	MaxSevereErrorRate float64                   `json:"max_severe_error_rate"`
	AllowedModes       []assessment.ScoringMode  `json:"allowed_modes"`
	Version            int                       `json:"version"`
	Status             PolicyStatus              `json:"status"`
	CreatedAt          time.Time                 `json:"created_at"`
}

type PutPolicyInput struct {
	SubjectCode        assessment.SubjectCode    `json:"subject_code"`
	EducationStage     assessment.EducationStage `json:"education_stage"`
	ArchetypeCode      string                    `json:"archetype_code"`
	RiskTier           assessment.RiskTier       `json:"risk_tier"`
	MinOCRQuality      float64                   `json:"min_ocr_quality"`
	MinParserQuality   float64                   `json:"min_parser_quality"`
	MinEvalN           int                       `json:"min_eval_n"`
	MaxSevereErrorRate float64                   `json:"max_severe_error_rate"`
	AllowedModes       []assessment.ScoringMode  `json:"allowed_modes"`
	Status             PolicyStatus              `json:"status"`
	ExpectedVersion    int                       `json:"expected_version"`
}

type EvaluationEvidence struct {
	Approved        bool    `json:"approved"`
	SampleCount     int     `json:"sample_count"`
	SevereErrorRate float64 `json:"severe_error_rate"`
	EvaluationRef   string  `json:"evaluation_ref,omitempty"`
}

// CalibrationEvidence answers only whether the model/prompt/deployment is
// usable for this frozen question context. It intentionally does not expose
// Gold answers, calibration submissions, or a teacher's quality record.
type CalibrationEvidence struct {
	Available      bool   `json:"available"`
	CalibrationRef string `json:"calibration_ref,omitempty"`
}

type DecisionInput struct {
	// RunItemID is the idempotency/audit key. It is an opaque caller-owned id:
	// it may be a subjective grading run, a worker item, or a dry-run item.
	RunItemID          string                          `json:"run_item_id"`
	AssessmentSnapshot assessment.ExamQuestionSnapshot `json:"assessment_snapshot"`
	RequestedMode      assessment.ScoringMode          `json:"requested_mode"`
	OCRQuality         *float64                        `json:"ocr_quality,omitempty"`
	ParserQuality      *float64                        `json:"parser_quality,omitempty"`
	RubricComplete     bool                            `json:"rubric_complete"`
	EvidenceAvailable  bool                            `json:"evidence_available"`
	Evaluation         EvaluationEvidence              `json:"evaluation"`
	Calibration        CalibrationEvidence             `json:"calibration"`
}

type Reason struct {
	Code     string `json:"code"`
	Message  string `json:"message"`
	Blocking bool   `json:"blocking"`
}

// OutputConstraint makes the product boundary explicit: even when an AI call
// is admitted, the model returns criterion/evidence observations. Scores are
// derived by the server from the frozen rubric or settled by a human flow.
type OutputConstraint struct {
	CriteriaEvidenceOnly bool   `json:"criteria_evidence_only"`
	AllowModelFinalScore bool   `json:"allow_model_final_score"`
	FinalScoreAuthority  string `json:"final_score_authority"`
	// MaxSevereErrorRisk is copied from the active immutable policy and is
	// consumed by A15 when recording the model's confidence candidate.
	// It is not a score threshold and never grants final-score authority.
	MaxSevereErrorRisk float64 `json:"max_severe_error_risk"`
}

type Decision struct {
	ID                string                 `json:"id"`
	TenantID          string                 `json:"tenant_id"`
	RunItemID         string                 `json:"run_item_id"`
	PolicyID          string                 `json:"policy_id,omitempty"`
	PolicyVersion     int                    `json:"policy_version"`
	Decision          assessment.ScoringMode `json:"decision"`
	ExternalAIAllowed bool                   `json:"external_ai_allowed"`
	InputSnapshot     map[string]any         `json:"input_snapshot"`
	Reasons           []Reason               `json:"reasons"`
	OutputConstraint  OutputConstraint       `json:"output_constraint"`
	CreatedAt         time.Time              `json:"created_at"`
}

func (d Decision) CanCallExternalAI() bool { return d.ExternalAIAllowed }

// Gate is the only dependency a grading pipeline needs. A caller must invoke
// Decide before an adapter is called, then use CanCallExternalAI rather than
// re-implementing risk logic in a handler or worker.
type Gate interface {
	Decide(context.Context, string, DecisionInput) (Decision, error)
}

type Store interface {
	PutPolicy(context.Context, string, PutPolicyInput) (Policy, error)
	GetActivePolicy(context.Context, string, assessment.SubjectCode, assessment.EducationStage, string, assessment.RiskTier) (Policy, error)
	CreateOrGetDecision(context.Context, string, Decision) (Decision, error)
	GetDecision(context.Context, string, string) (Decision, error)
}
