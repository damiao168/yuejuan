// Package aidisagreement records an auditable, human-only follow-up when a
// real subjective-AI suggestion differs from the human grading fact that was
// created from the same review task. It deliberately never updates a grade or
// exposes answer content, Gold content, or blind-seed material.
package aidisagreement

import (
	"context"
	"errors"
	"time"
)

var (
	ErrNotFound     = errors.New("AI-human disagreement not found")
	ErrInvalidInput = errors.New("invalid AI-human disagreement input")
	ErrConflict     = errors.New("AI-human disagreement revision conflict")
	ErrInvalidState = errors.New("AI-human disagreement state does not allow this action")
)

type Severity string

const (
	SeverityWarning Severity = "warning"
	SeveritySevere  Severity = "severe"
)

type Status string

const (
	StatusNeedsReview Status = "needs_review"
	StatusClassified  Status = "classified"
	StatusRouted      Status = "routed"
)

type DifferenceType string

const (
	DifferenceScore          DifferenceType = "score"
	DifferenceCriterion      DifferenceType = "criterion"
	DifferenceEvidence       DifferenceType = "evidence"
	DifferenceScoreCriterion DifferenceType = "score_and_criterion"
	DifferenceScoreEvidence  DifferenceType = "score_and_evidence"
	DifferenceCombined       DifferenceType = "combined"
)

type Taxonomy string

const (
	TaxonomyAIScoringError       Taxonomy = "ai_scoring_error"
	TaxonomyHumanScoringError    Taxonomy = "human_scoring_error"
	TaxonomyOCRError             Taxonomy = "ocr_error"
	TaxonomyParserError          Taxonomy = "parser_error"
	TaxonomyRubricAmbiguity      Taxonomy = "rubric_ambiguity"
	TaxonomyReferenceAnswerIssue Taxonomy = "reference_answer_issue"
	TaxonomyQuestionIssue        Taxonomy = "question_issue"
	TaxonomyInsufficientEvidence Taxonomy = "insufficient_evidence"
	TaxonomyAcceptableVariation  Taxonomy = "acceptable_variation"
)

func (t Taxonomy) Valid() bool {
	switch t {
	case TaxonomyAIScoringError, TaxonomyHumanScoringError, TaxonomyOCRError,
		TaxonomyParserError, TaxonomyRubricAmbiguity, TaxonomyReferenceAnswerIssue,
		TaxonomyQuestionIssue, TaxonomyInsufficientEvidence, TaxonomyAcceptableVariation:
		return true
	default:
		return false
	}
}

// ActualComparison is derived by the Store from tenant-scoped ai_grade,
// human_grade and review_task records. It is intentionally aggregate-only:
// evidence is represented as counts, not answer text, image URLs or Gold.
type ActualComparison struct {
	ExamID                  string
	QuestionID              string
	SubmissionID            string
	AnswerSegmentID         string
	AICandidateID           string
	HumanGradeID            string
	AISuggestedScore        float64
	HumanScore              float64
	MaxScore                float64
	AIConfidence            float64
	AIEvidenceCount         int
	AIMatchedCriterionCount int
	HumanCriterionCount     int
	RiskTier                string
}

type Disagreement struct {
	ID                 string          `json:"id"`
	TenantID           string          `json:"tenant_id,omitempty"`
	ExamID             string          `json:"exam_id"`
	QuestionID         string          `json:"question_id"`
	SubmissionID       string          `json:"submission_id"`
	AnswerSegmentID    string          `json:"answer_segment_id"`
	AICandidateID      string          `json:"ai_candidate_id"`
	HumanGradeID       string          `json:"human_grade_id"`
	AICandidateScore   float64         `json:"ai_candidate_score"`
	HumanScore         float64         `json:"human_score"`
	MaxScore           float64         `json:"max_score"`
	Delta              float64         `json:"delta"`
	AbsoluteDelta      float64         `json:"absolute_delta"`
	DifferenceType     DifferenceType  `json:"difference_type"`
	Severity           Severity        `json:"severity"`
	RiskTier           string          `json:"risk_tier"`
	TriggerRules       []string        `json:"trigger_rules"`
	EvidenceSummary    EvidenceSummary `json:"evidence_summary"`
	Status             Status          `json:"status"`
	Taxonomy           Taxonomy        `json:"taxonomy,omitempty"`
	ReviewerID         string          `json:"reviewer_id,omitempty"`
	ReviewedAt         *time.Time      `json:"reviewed_at,omitempty"`
	Notes              string          `json:"notes,omitempty"`
	RoutedReviewTaskID string          `json:"routed_review_task_id,omitempty"`
	RoutedBy           string          `json:"routed_by,omitempty"`
	Revision           int64           `json:"revision"`
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
}

type EvidenceSummary struct {
	AIEvidenceCount         int `json:"ai_evidence_count"`
	AIMatchedCriterionCount int `json:"ai_matched_criterion_count"`
	HumanCriterionCount     int `json:"human_criterion_count"`
}

type CaptureInput struct {
	AICandidateID string `json:"ai_candidate_id"`
	HumanGradeID  string `json:"human_grade_id"`
}

type ClassifyInput struct {
	Taxonomy         Taxonomy `json:"taxonomy"`
	Notes            string   `json:"notes,omitempty"`
	ExpectedRevision int64    `json:"expected_revision"`
}

type RouteInput struct {
	ReviewTaskID     string `json:"review_task_id"`
	ExpectedRevision int64  `json:"expected_revision"`
}

type Filter struct {
	ExamID     string
	QuestionID string
	Status     Status
	Severity   Severity
	Limit      int
}

// DatasetEntry is safe for evaluation exports: it is de-identified and has
// no student response, image, comments, Gold answer or reviewer identity.
// LineageRef is one-way so a privileged service can join back internally while
// a dataset consumer cannot use it to identify a student.
type DatasetEntry struct {
	LineageRef       string         `json:"lineage_ref"`
	QuestionID       string         `json:"question_id"`
	RiskTier         string         `json:"risk_tier"`
	AICandidateScore float64        `json:"ai_candidate_score"`
	HumanScore       float64        `json:"human_score"`
	MaxScore         float64        `json:"max_score"`
	Delta            float64        `json:"delta"`
	Severity         Severity       `json:"severity"`
	DifferenceType   DifferenceType `json:"difference_type"`
	Taxonomy         Taxonomy       `json:"taxonomy"`
	TriggerRules     []string       `json:"trigger_rules"`
}

type Store interface {
	LoadActualComparison(context.Context, string, CaptureInput) (ActualComparison, error)
	Upsert(context.Context, string, Disagreement) (Disagreement, bool, error)
	Get(context.Context, string, string) (Disagreement, error)
	List(context.Context, string, Filter) ([]Disagreement, error)
	Classify(context.Context, string, string, string, ClassifyInput, time.Time) (Disagreement, error)
	Route(context.Context, string, string, string, RouteInput, time.Time) (Disagreement, error)
}
