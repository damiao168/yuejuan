// Package gradingevaluation records real, offline comparisons between a
// governed scoring run and an already-aligned Gold or human reference.  It is
// intentionally evidence-only: it does not invoke a model, expose answers, or
// make a production routing decision.
package gradingevaluation

import (
	"context"
	"errors"
	"time"
)

var (
	ErrNotFound      = errors.New("grading evaluation resource not found")
	ErrInvalidInput  = errors.New("invalid grading evaluation input")
	ErrStateConflict = errors.New("grading evaluation state conflict")
	ErrConflict      = errors.New("grading evaluation conflict")
)

type RunStatus string

const (
	RunDraft     RunStatus = "draft"
	RunCompleted RunStatus = "completed"
	RunInvalid   RunStatus = "invalidated"
)

type ReferenceKind string

const (
	ReferenceGold             ReferenceKind = "gold"
	ReferenceHumanAdjudicated ReferenceKind = "human_adjudicated"
)

const (
	SliceSubject          = "subject"
	SliceArchetype        = "archetype"
	SliceScoreBand        = "score_band"
	SliceOCRQuality       = "ocr_quality"
	SliceAnswerLength     = "answer_length"
	SliceRubricComplexity = "rubric_complexity"
)

type Run struct {
	ID                 string     `json:"id"`
	TenantID           string     `json:"tenant_id,omitempty"`
	Key                string     `json:"key"`
	DisplayName        string     `json:"display_name"`
	ModelReference     string     `json:"model_reference"`
	PromptVersion      string     `json:"prompt_version"`
	RubricVersion      string     `json:"rubric_version"`
	DatasetReference   string     `json:"dataset_reference"`
	DatasetSHA256      string     `json:"dataset_sha256"`
	Status             RunStatus  `json:"status"`
	ObservationCount   int        `json:"observation_count"`
	CompletedAt        *time.Time `json:"completed_at,omitempty"`
	InvalidatedAt      *time.Time `json:"invalidated_at,omitempty"`
	InvalidationReason string     `json:"invalidation_reason,omitempty"`
	CreatedBy          string     `json:"created_by,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
}

// Observation contains only an opaque response key/fingerprint and aligned
// scores. Answer text, images and Gold content never enter this component.
// The six slice fields are recorded at observation time so a later metadata
// change cannot rewrite historical evaluation evidence.
type Observation struct {
	ID                  string        `json:"id"`
	RunID               string        `json:"run_id"`
	ResponseKey         string        `json:"response_key"`
	ResponseFingerprint string        `json:"response_fingerprint"`
	ReferenceKind       ReferenceKind `json:"reference_kind"`
	Subject             string        `json:"subject"`
	Archetype           string        `json:"archetype"`
	OCRQuality          string        `json:"ocr_quality"`
	AnswerLength        string        `json:"answer_length"`
	RubricComplexity    string        `json:"rubric_complexity"`
	ReferenceScore      float64       `json:"reference_score"`
	ModelScore          float64       `json:"model_score"`
	MaxScore            float64       `json:"max_score"`
	ReferenceScoreBand  string        `json:"reference_score_band"`
	ObservedAt          time.Time     `json:"observed_at"`
}

type Metrics struct {
	SampleCount          int      `json:"sample_count"`
	MAE                  float64  `json:"mae"`
	ExactRate            float64  `json:"exact_rate"`
	WithinOneRate        float64  `json:"within_one_rate"`
	SevereErrorRate      float64  `json:"severe_error_rate"`
	FalseZeroRate        float64  `json:"false_zero_rate"`
	FalseFullRate        float64  `json:"false_full_rate"`
	QWK                  *float64 `json:"qwk,omitempty"`
	QWKAvailable         bool     `json:"qwk_available"`
	QWKUnavailableReason string   `json:"qwk_unavailable_reason,omitempty"`
}

type SliceMetric struct {
	ID         string    `json:"id"`
	RunID      string    `json:"run_id"`
	Dimension  string    `json:"dimension"`
	Value      string    `json:"value"`
	Metrics    Metrics   `json:"metrics"`
	ComputedAt time.Time `json:"computed_at"`
}

// ResponseDifficulty is a run-scoped, empirical hard-case indicator. It is
// not a label of student ability and must not be used as a production score.
type ResponseDifficulty struct {
	ID                  string    `json:"id"`
	RunID               string    `json:"run_id"`
	ResponseKey         string    `json:"response_key"`
	ResponseFingerprint string    `json:"response_fingerprint"`
	DifficultyScore     float64   `json:"difficulty_score"`
	DifficultyBand      string    `json:"difficulty_band"`
	NormalizedError     float64   `json:"normalized_error"`
	SevereError         bool      `json:"severe_error"`
	OCRQuality          string    `json:"ocr_quality"`
	AnswerLength        string    `json:"answer_length"`
	RubricComplexity    string    `json:"rubric_complexity"`
	EvidenceNote        string    `json:"evidence_note"`
	ComputedAt          time.Time `json:"computed_at"`
}

type CreateRunInput struct {
	Key              string `json:"key"`
	DisplayName      string `json:"display_name"`
	ModelReference   string `json:"model_reference"`
	PromptVersion    string `json:"prompt_version"`
	RubricVersion    string `json:"rubric_version"`
	DatasetReference string `json:"dataset_reference"`
	DatasetSHA256    string `json:"dataset_sha256"`
}

type AddObservationInput struct {
	ResponseKey         string        `json:"response_key"`
	ResponseFingerprint string        `json:"response_fingerprint"`
	ReferenceKind       ReferenceKind `json:"reference_kind"`
	Subject             string        `json:"subject"`
	Archetype           string        `json:"archetype"`
	OCRQuality          string        `json:"ocr_quality"`
	AnswerLength        string        `json:"answer_length"`
	RubricComplexity    string        `json:"rubric_complexity"`
	ReferenceScore      float64       `json:"reference_score"`
	ModelScore          float64       `json:"model_score"`
	MaxScore            float64       `json:"max_score"`
}

type RunFilter struct {
	Limit int
}

type Store interface {
	CreateRun(context.Context, string, string, CreateRunInput) (Run, error)
	GetRun(context.Context, string, string) (Run, error)
	ListRuns(context.Context, string, RunFilter) ([]Run, error)
	AddObservation(context.Context, string, string, AddObservationInput) (Observation, error)
	ListObservations(context.Context, string, string) ([]Observation, error)
	ReplaceComputed(context.Context, string, string, int, []SliceMetric, []ResponseDifficulty, time.Time) (Run, error)
	ListSliceMetrics(context.Context, string, string) ([]SliceMetric, error)
	ListResponseDifficulty(context.Context, string, string) ([]ResponseDifficulty, error)
	InvalidateRun(context.Context, string, string, string, time.Time) (Run, error)
}
