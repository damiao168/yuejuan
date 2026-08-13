// Package modelcalibration turns a model's reported confidence into a
// version-scoped empirical estimate. It is deliberately evidence-only: a
// calibration artifact can make a candidate abstain, but can never publish a
// score or grant a model final-score authority.
package modelcalibration

import (
	"context"
	"errors"
	"time"
)

var (
	ErrNotFound            = errors.New("model calibration resource not found")
	ErrInvalidInput        = errors.New("invalid model calibration input")
	ErrStateConflict       = errors.New("model calibration state conflict")
	ErrConflict            = errors.New("model calibration conflict")
	ErrEvaluationRequired  = errors.New("completed aligned evaluation is required")
	ErrEvidenceUnavailable = errors.New("aligned evaluation observation is unavailable")
)

type Method string

const (
	MethodAuto      Method = "auto"
	MethodIsotonic  Method = "isotonic"
	MethodLogistic  Method = "logistic"
	MethodConformal Method = "conformal"
)

func (m Method) Valid() bool {
	return m == MethodAuto || m == MethodIsotonic || m == MethodLogistic || m == MethodConformal
}

type Status string

const (
	StatusDraft       Status = "draft"
	StatusCompleted   Status = "completed"
	StatusApproved    Status = "approved"
	StatusInvalidated Status = "invalidated"
)

// Axis is intentionally complete. In particular, prompt and rubric versions
// are part of the identity: a later model, prompt, or rubric cannot inherit
// calibration evidence from an earlier deployment.
type Axis struct {
	ModelReference string `json:"model_reference"`
	PromptVersion  string `json:"prompt_version"`
	RubricVersion  string `json:"rubric_version"`
	Subject        string `json:"subject"`
	Archetype      string `json:"archetype"`
	// SliceKey is all, score_band:{zero|partial|full}, or
	// ocr_quality:{high|medium|low|unknown}. It leaves a reproducible record
	// of whether a more narrow safety slice was used.
	SliceKey string `json:"slice_key"`
}

type Calibration struct {
	ID                 string     `json:"id"`
	TenantID           string     `json:"tenant_id,omitempty"`
	Key                string     `json:"key"`
	EvaluationRunID    string     `json:"evaluation_run_id"`
	Axis               Axis       `json:"axis"`
	Method             Method     `json:"method"`
	Status             Status     `json:"status"`
	CalibrationN       int        `json:"calibration_n"`
	ArtifactURI        string     `json:"artifact_uri,omitempty"`
	ArtifactSHA256     string     `json:"artifact_sha256,omitempty"`
	Artifact           Artifact   `json:"artifact,omitempty"`
	CreatedBy          string     `json:"created_by,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	CompletedAt        *time.Time `json:"completed_at,omitempty"`
	ApprovedAt         *time.Time `json:"approved_at,omitempty"`
	ApprovedBy         string     `json:"approved_by,omitempty"`
	InvalidatedAt      *time.Time `json:"invalidated_at,omitempty"`
	InvalidatedBy      string     `json:"invalidated_by,omitempty"`
	InvalidationReason string     `json:"invalidation_reason,omitempty"`
}

// CalibrationEvidence is a legal, aligned A16 observation decorated with the
// confidence emitted by the exact governed model/prompt being calibrated.
// It contains no answer body, image, OCR text or Gold rationale.
type CalibrationEvidence struct {
	ID              string    `json:"id"`
	CalibrationID   string    `json:"calibration_id"`
	EvaluationRunID string    `json:"evaluation_run_id"`
	ResponseKey     string    `json:"response_key"`
	RawConfidence   float64   `json:"raw_confidence"`
	Correct         bool      `json:"correct"`
	SevereError     bool      `json:"severe_error"`
	ScoreBand       string    `json:"score_band"`
	OCRQuality      string    `json:"ocr_quality"`
	ObservedAt      time.Time `json:"observed_at"`
}

type Bin struct {
	MinRawConfidence     float64 `json:"min_raw_confidence"`
	MaxRawConfidence     float64 `json:"max_raw_confidence"`
	CalibratedConfidence float64 `json:"calibrated_confidence"`
	SampleCount          int     `json:"sample_count"`
	CorrectCount         int     `json:"correct_count"`
}

// RiskCoveragePoint is empirical and comes only from the aligned observations
// stored for this artifact. Coverage is not a promise of automatic scoring.
type RiskCoveragePoint struct {
	Threshold       float64 `json:"threshold"`
	Coverage        float64 `json:"coverage"`
	SampleCount     int     `json:"sample_count"`
	EmpiricalRisk   float64 `json:"empirical_risk"`
	SevereErrorRate float64 `json:"severe_error_rate"`
}

type Metrics struct {
	SampleCount              int      `json:"sample_count"`
	BrierScore               float64  `json:"brier_score"`
	ExpectedCalibrationError float64  `json:"expected_calibration_error"`
	MiddleScoreSampleCount   int      `json:"middle_score_sample_count"`
	MiddleScoreBrierScore    *float64 `json:"middle_score_brier_score,omitempty"`
}

type Artifact struct {
	SchemaVersion     int                 `json:"schema_version"`
	Method            Method              `json:"method"`
	Bins              []Bin               `json:"bins"`
	Metrics           Metrics             `json:"metrics"`
	RiskCoverageCurve []RiskCoveragePoint `json:"risk_coverage_curve"`
}

type CreateInput struct {
	Key             string `json:"key"`
	EvaluationRunID string `json:"evaluation_run_id"`
	Axis            Axis   `json:"axis"`
	Method          Method `json:"method"`
}

type AddEvidenceInput struct {
	ResponseKey   string  `json:"response_key"`
	RawConfidence float64 `json:"raw_confidence"`
}

// Candidate is an immutable confidence decision for an opaque scoring
// candidate. SuggestedScore is intentionally absent: this table is not a
// grade fact and cannot be used as one.
type Candidate struct {
	ID                   string    `json:"id"`
	TenantID             string    `json:"tenant_id,omitempty"`
	CandidateKey         string    `json:"candidate_key"`
	Axis                 Axis      `json:"axis"`
	RawConfidence        float64   `json:"raw_confidence"`
	CalibratedConfidence *float64  `json:"calibrated_confidence,omitempty"`
	CalibrationID        string    `json:"calibration_id,omitempty"`
	TargetRisk           *float64  `json:"target_risk,omitempty"`
	AbstainReason        string    `json:"abstain_reason,omitempty"`
	CreatedAt            time.Time `json:"created_at"`
}

type RecordCandidateInput struct {
	CandidateKey  string  `json:"candidate_key"`
	Axis          Axis    `json:"axis"`
	RawConfidence float64 `json:"raw_confidence"`
	// TargetRisk is a maximum observed severe-error rate. When it is provided,
	// the candidate abstains unless its calibrated confidence reaches a
	// threshold whose empirical severe risk meets this target.
	TargetRisk *float64 `json:"target_risk,omitempty"`
}

// ApprovedEvidence is intentionally a small projection for the A14 gate.
// Callers get an artifact reference, not its training observations.
type ApprovedEvidence struct {
	Available       bool   `json:"available"`
	CalibrationID   string `json:"calibration_id,omitempty"`
	CalibrationRef  string `json:"calibration_ref,omitempty"`
	CalibrationN    int    `json:"calibration_n,omitempty"`
	EvaluationRunID string `json:"evaluation_run_id,omitempty"`
	// SevereErrorRate is the threshold-0 empirical rate from the immutable
	// artifact. It is supplied so A14 can use the same evidence rather than
	// manufacture a separate overall metric from unrelated slices.
	SevereErrorRate float64 `json:"severe_error_rate,omitempty"`
}

// EvaluationReader is implemented by gradingevaluation.Service. Keeping the
// interface here avoids duplicate access paths and ensures this package can
// never manufacture alignment data on its own.
type EvaluationReader interface {
	GetRun(context.Context, string, string) (EvaluationRun, error)
	ListObservations(context.Context, string, string) ([]EvaluationObservation, error)
}

type EvaluationRun struct {
	ID             string
	ModelReference string
	PromptVersion  string
	RubricVersion  string
	Status         string
}

type EvaluationObservation struct {
	ResponseKey    string
	Subject        string
	Archetype      string
	OCRQuality     string
	ScoreBand      string
	ReferenceScore float64
	ModelScore     float64
	MaxScore       float64
}

type Store interface {
	Create(context.Context, string, string, CreateInput) (Calibration, error)
	Get(context.Context, string, string) (Calibration, error)
	List(context.Context, string, Axis, int) ([]Calibration, error)
	AddEvidence(context.Context, string, string, CalibrationEvidence) (CalibrationEvidence, error)
	ListEvidence(context.Context, string, string) ([]CalibrationEvidence, error)
	Complete(context.Context, string, string, int, Artifact, string, string, time.Time) (Calibration, error)
	Approve(context.Context, string, string, string, time.Time) (Calibration, error)
	Invalidate(context.Context, string, string, string, string, time.Time) (Calibration, error)
	FindApproved(context.Context, string, Axis) (Calibration, error)
	CreateOrGetCandidate(context.Context, string, Candidate) (Candidate, error)
}
