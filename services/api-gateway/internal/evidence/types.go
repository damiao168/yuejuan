package evidence

import (
	"context"
	"errors"
	"time"

	"edugrade-enterprise/services/api-gateway/internal/grading"
	"edugrade-enterprise/services/api-gateway/internal/paper"
)

var (
	ErrNotFound     = errors.New("evidence resource not found")
	ErrInvalidInput = errors.New("invalid evidence input")
)

type Grade struct {
	ID               string                `json:"id"`
	TenantID         string                `json:"tenant_id"`
	AnswerSegmentID  string                `json:"answer_segment_id"`
	QuestionID       string                `json:"question_id"`
	QuestionNo       string                `json:"question_no"`
	QuestionType     string                `json:"question_type"`
	SuggestedScore   float64               `json:"suggested_score"`
	MaxScore         float64               `json:"max_score"`
	MatchedPoints    []grading.PointResult `json:"matched_points"`
	MissingPoints    []grading.PointResult `json:"missing_points"`
	Evidence         []grading.Evidence    `json:"evidence"`
	RiskFlags        []string              `json:"risk_flags"`
	NeedsHumanReview bool                  `json:"needs_human_review"`
	Status           string                `json:"status"`
}

type Context struct {
	Grade             Grade
	AnswerText        string
	OCRConfidence     *float64
	AnswerSegmentBBox []float64
	Rubric            paper.Rubric
}

type Issue struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type VerificationResult struct {
	Passed           bool     `json:"passed"`
	Failed           []Issue  `json:"failed"`
	Warnings         []Issue  `json:"warnings"`
	CorrectedFlags   []string `json:"corrected_flags"`
	NeedsHumanReview bool     `json:"needs_human_review"`
}

type AgentJob struct {
	ID               string             `json:"id"`
	TenantID         string             `json:"tenant_id"`
	JobType          string             `json:"job_type"`
	TargetType       string             `json:"target_type"`
	TargetID         string             `json:"target_id"`
	Status           string             `json:"status"`
	Result           VerificationResult `json:"result"`
	NeedsHumanReview bool               `json:"needs_human_review"`
	CreatedBy        string             `json:"created_by"`
	CreatedAt        time.Time          `json:"created_at"`
}

type Store interface {
	LoadContext(ctx context.Context, tenantID string, gradeID string) (Context, error)
	CreateJob(ctx context.Context, tenantID string, actorID string, gradeID string, result VerificationResult) (AgentJob, error)
}
