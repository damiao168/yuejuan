package grading

import (
	"context"
	"errors"
	"time"

	"edugrade-enterprise/services/api-gateway/internal/paper"
)

const (
	RuleVersion = "objective-rules-v1"
	GraderType  = "rule_based_objective"
)

var (
	ErrNotFound                = errors.New("grading resource not found")
	ErrInvalidInput            = errors.New("invalid grading input")
	ErrUnsupportedQuestionType = errors.New("unsupported question type for rule grading")
	ErrAnswerMissing           = errors.New("answer segment has no recorded answer")
	ErrAnswerKeyMissing        = errors.New("question has no answer key")
)

type SegmentAnswer struct {
	ID              string         `json:"id"`
	TenantID        string         `json:"tenant_id"`
	AnswerSegmentID string         `json:"answer_segment_id"`
	AnswerText      string         `json:"answer_text"`
	AnswerPayload   map[string]any `json:"answer_payload"`
	Source          string         `json:"source"`
	Confidence      *float64       `json:"confidence,omitempty"`
	RecordedBy      string         `json:"recorded_by"`
	CreatedAt       time.Time      `json:"created_at"`
}

type RecordAnswerInput struct {
	AnswerText    string         `json:"answer_text"`
	AnswerPayload map[string]any `json:"answer_payload"`
	Source        string         `json:"source"`
	Confidence    *float64       `json:"confidence"`
}

type PointResult struct {
	Code  string  `json:"code"`
	Label string  `json:"label"`
	Score float64 `json:"score"`
}

type Evidence struct {
	Type           string    `json:"type"`
	AnswerSegment  string    `json:"answer_segment_id,omitempty"`
	AnswerText     string    `json:"answer_text,omitempty"`
	StandardAnswer string    `json:"standard_answer,omitempty"`
	Rule           string    `json:"rule,omitempty"`
	BBox           []float64 `json:"bbox,omitempty"`
}

type Grade struct {
	ID               string         `json:"id"`
	TenantID         string         `json:"tenant_id"`
	AnswerSegmentID  string         `json:"answer_segment_id"`
	QuestionID       string         `json:"question_id"`
	QuestionNo       string         `json:"question_no"`
	QuestionType     string         `json:"question_type"`
	AnswerVersion    string         `json:"answer_version"`
	GraderType       string         `json:"grader_type"`
	RuleVersion      string         `json:"rule_version"`
	SuggestedScore   float64        `json:"suggested_score"`
	MaxScore         float64        `json:"max_score"`
	Confidence       float64        `json:"confidence"`
	MatchedPoints    []PointResult  `json:"matched_points"`
	MissingPoints    []PointResult  `json:"missing_points"`
	Evidence         []Evidence     `json:"evidence"`
	RiskFlags        []string       `json:"risk_flags"`
	NeedsHumanReview bool           `json:"needs_human_review"`
	AutoPass         bool           `json:"auto_pass"`
	Mock             bool           `json:"mock"`
	RawOutput        map[string]any `json:"raw_output"`
	CreatedBy        string         `json:"created_by"`
	CreatedAt        time.Time      `json:"created_at"`
}

type Context struct {
	SegmentID string
	Question  paper.Question
	AnswerKey paper.AnswerKey
	Answer    SegmentAnswer
}

type Store interface {
	RecordAnswer(ctx context.Context, tenantID string, segmentID string, actorID string, input RecordAnswerInput) (SegmentAnswer, error)
	LoadContext(ctx context.Context, tenantID string, segmentID string) (Context, error)
	CreateGrade(ctx context.Context, tenantID string, actorID string, grade Grade) (Grade, error)
	ListGrades(ctx context.Context, tenantID string, segmentID string) ([]Grade, error)
}
