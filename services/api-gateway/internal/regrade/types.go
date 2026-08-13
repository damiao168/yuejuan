// Package regrade owns the controlled question-level correction workflow.
// It keeps the source-release facts, proposed scores and reviewer decisions
// separate.  A completed job is only an input to a *new* score release; this
// package intentionally cannot write human_grade, final_grade or a release.
package regrade

import (
	"context"
	"errors"
	"time"

	"edugrade-enterprise/services/api-gateway/internal/paper"
)

var (
	ErrNotFound            = errors.New("regrade resource not found")
	ErrInvalidInput        = errors.New("invalid regrade input")
	ErrStateConflict       = errors.New("regrade state conflict")
	ErrRevisionConflict    = errors.New("regrade item revision conflict")
	ErrNoAffectedItems     = errors.New("regrade selector matched no source release facts")
	ErrSourceRelease       = errors.New("regrade source release is not a published release for the requested exam")
	ErrAssignmentForbidden = errors.New("regrade item is not assigned to this reviewer")
)

const (
	ReasonAnswerKeyError = "answer_key_error"
	ReasonRubricError    = "rubric_error"
	ReasonOCRCorrection  = "ocr_correction"
	ReasonParserBug      = "parser_bug"
	ReasonModelIssue     = "model_issue"
	ReasonQualityIssue   = "quality_incident"
	ReasonAppealPattern  = "appeal_pattern"
	ReasonOther          = "other"

	StrategyRuleRecompute  = "rule_recompute"
	StrategyAIRecompute    = "ai_recompute_then_review"
	StrategyHumanRecheck   = "human_recheck"
	StrategyBackmarkImport = "backmark_import"

	StatusAwaitingApproval = "awaiting_approval"
	StatusApproved         = "approved"
	StatusRunning          = "running"
	StatusPaused           = "paused"
	StatusDiffReview       = "diff_review"
	StatusReadyForRelease  = "ready_for_release"
	StatusCancelled        = "cancelled"

	ItemPending        = "pending"
	ItemClaimed        = "claimed"
	ItemCandidateReady = "candidate_ready"
	ItemAwaitingReview = "awaiting_review"
	ItemResolved       = "resolved"
	ItemException      = "exception"
	ItemCancelled      = "cancelled"

	ReviewAccept    = "accept"
	ReviewReject    = "reject"
	ReviewException = "exception"
)

type ScoreBand struct {
	Min *float64 `json:"min,omitempty"`
	Max *float64 `json:"max,omitempty"`
}

// Selector is deliberately limited to stable source-release values. It makes
// an affected population replayable and avoids accepting raw client SQL.
type Selector struct {
	SubmissionIDs []string   `json:"submission_ids,omitempty"`
	ScoreBand     *ScoreBand `json:"score_band,omitempty"`
}

type SourceItem struct {
	SubmissionID    string  `json:"submission_id"`
	StudentID       string  `json:"student_id,omitempty"`
	OldFinalGradeID string  `json:"old_final_grade_id"`
	OldScore        float64 `json:"old_score"`
	MaxScore        float64 `json:"max_score"`
}

type ScoreBandCount struct {
	Score float64 `json:"score"`
	Count int     `json:"count"`
}

type PotentialDelta struct {
	Min float64 `json:"min"`
	Max float64 `json:"max"`
}

type Preview struct {
	ExamID                string           `json:"exam_id"`
	QuestionID            string           `json:"question_id"`
	SourceReleaseID       string           `json:"source_release_id"`
	SourceReleaseVersion  int              `json:"source_release_version"`
	CurrentReleaseID      string           `json:"current_release_id,omitempty"`
	CurrentReleaseVersion int              `json:"current_release_version,omitempty"`
	AffectedCount         int              `json:"affected_count"`
	ScoreBands            []ScoreBandCount `json:"old_score_bands"`
	PotentialDelta        PotentialDelta   `json:"possible_delta"`
}

type Job struct {
	ID                  string     `json:"id"`
	TenantID            string     `json:"tenant_id"`
	ExamID              string     `json:"exam_id"`
	QuestionID          string     `json:"question_id"`
	SourceReleaseID     string     `json:"source_release_id"`
	ReasonCode          string     `json:"reason_code"`
	ReasonText          string     `json:"reason_text"`
	Strategy            string     `json:"strategy"`
	Selector            Selector   `json:"selector"`
	NewRubricSnapshotID string     `json:"new_rubric_snapshot_id,omitempty"`
	NewPolicyVersion    string     `json:"new_policy_version,omitempty"`
	SeverityDelta       float64    `json:"severity_delta"`
	IdempotencyKey      string     `json:"-"`
	Status              string     `json:"status"`
	AffectedCount       int        `json:"affected_count"`
	CreatedBy           string     `json:"created_by"`
	ApprovedBy          string     `json:"approved_by,omitempty"`
	ApprovedAt          *time.Time `json:"approved_at,omitempty"`
	FinalizedBy         string     `json:"finalized_by,omitempty"`
	FinalizedAt         *time.Time `json:"finalized_at,omitempty"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

type Item struct {
	ID                        string            `json:"id"`
	JobID                     string            `json:"job_id"`
	SubmissionID              string            `json:"submission_id"`
	OldFinalGradeID           string            `json:"old_final_grade_id"`
	OldScore                  float64           `json:"old_score"`
	MaxScore                  float64           `json:"max_score"`
	CandidateGradeID          string            `json:"candidate_grade_id,omitempty"`
	CandidateScore            *float64          `json:"candidate_score,omitempty"`
	CandidateRubricSelections []RubricSelection `json:"candidate_rubric_selections,omitempty"`
	CandidateComment          string            `json:"candidate_comment,omitempty"`
	ReviewedGradeID           string            `json:"reviewed_grade_id,omitempty"`
	ReviewedScore             *float64          `json:"reviewed_score,omitempty"`
	Delta                     *float64          `json:"delta,omitempty"`
	Status                    string            `json:"status"`
	AssignedTo                string            `json:"assigned_to,omitempty"`
	ClaimedBy                 string            `json:"claimed_by,omitempty"`
	ReviewedBy                string            `json:"reviewed_by,omitempty"`
	ReviewNote                string            `json:"review_note,omitempty"`
	Revision                  int64             `json:"revision"`
	CreatedAt                 time.Time         `json:"created_at"`
	UpdatedAt                 time.Time         `json:"updated_at"`
}

// WorkItem is the reviewer-safe queue shape. It deliberately excludes the
// old released score, final-grade identifier and submission identifier so a
// human recheck is not anchored by the source outcome or able to traverse a
// student's submission through another workflow.
type WorkItem struct {
	ID           string    `json:"id"`
	JobID        string    `json:"job_id"`
	SubmissionID string    `json:"-"`
	Status       string    `json:"status"`
	MaxScore     float64   `json:"max_score"`
	Revision     int64     `json:"revision"`
	CreatedAt    time.Time `json:"created_at"`
}

// GraderContext is the independent work surface for a regrade item. It is
// built from the job's selected assessment snapshot, not from the historic
// score release or a previous human-grade record.
type GraderContext struct {
	Item             WorkItem       `json:"item"`
	ExpectedRevision int64          `json:"expected_revision"`
	Question         GraderQuestion `json:"question"`
	FrozenRubric     paper.Rubric   `json:"frozen_rubric"`
	Answer           GraderAnswer   `json:"answer"`
}

type GraderQuestion struct {
	ID              string   `json:"id"`
	QuestionNo      string   `json:"question_no"`
	QuestionType    string   `json:"question_type"`
	Score           float64  `json:"score"`
	Stem            string   `json:"stem,omitempty"`
	KnowledgePoints []string `json:"knowledge_points,omitempty"`
}

type GraderAnswer struct {
	RawAnswer       string `json:"raw_answer,omitempty"`
	OCRText         string `json:"ocr_text,omitempty"`
	SegmentStatus   string `json:"segment_status"`
	SegmentImageURL string `json:"segment_image_url"`
}

// ContextSource keeps the answer evidence bridge narrow. Implementations may
// return only an already-claimed, reviewer-assigned item's content; no source
// score, historic grade or release fact crosses this boundary.
type ContextSource interface {
	GetGraderContext(context.Context, string, string, string) (GraderContext, error)
	GetSegmentID(context.Context, string, string, string) (string, error)
}

type Event struct {
	ID        string         `json:"id"`
	JobID     string         `json:"job_id"`
	ItemID    string         `json:"item_id,omitempty"`
	Type      string         `json:"type"`
	ActorID   string         `json:"actor_id"`
	Payload   map[string]any `json:"payload"`
	CreatedAt time.Time      `json:"created_at"`
}

type Histogram struct {
	Delta float64 `json:"delta"`
	Count int     `json:"count"`
}

type SevereChange struct {
	SubmissionID string  `json:"submission_id"`
	OldScore     float64 `json:"old_score"`
	NewScore     float64 `json:"new_score"`
	Delta        float64 `json:"delta"`
}

// ReleasePlan is a deliberate hand-off, not a public score mutation. A18 can
// build a later immutable score release from it; A20 can block publication
// while a job is not ready. Existing published score_release rows remain
// untouched throughout the A19 workflow.
type ReleasePlan struct {
	JobID           string          `json:"job_id"`
	ExamID          string          `json:"exam_id"`
	QuestionID      string          `json:"question_id"`
	SourceReleaseID string          `json:"source_release_id"`
	AffectedCount   int             `json:"affected_count"`
	ResolvedItems   []ReleaseChange `json:"resolved_items"`
}

type ReleaseChange struct {
	SubmissionID    string  `json:"submission_id"`
	OldFinalGradeID string  `json:"old_final_grade_id"`
	SourceScore     float64 `json:"source_score"`
	RegradedScore   float64 `json:"regraded_score"`
	MaxScore        float64 `json:"max_score"`
	ReviewedGradeID string  `json:"reviewed_grade_id,omitempty"`
}

type Summary struct {
	Job           Job            `json:"job"`
	Items         []Item         `json:"items"`
	Events        []Event        `json:"events"`
	DiffHistogram []Histogram    `json:"diff_histogram"`
	SevereChanges []SevereChange `json:"severe_changes"`
	ReleasePlan   *ReleasePlan   `json:"release_plan,omitempty"`
}

type CreateInput struct {
	SourceReleaseID     string   `json:"source_release_id"`
	ReasonCode          string   `json:"reason_code"`
	ReasonText          string   `json:"reason_text"`
	Strategy            string   `json:"strategy"`
	Selector            Selector `json:"selector"`
	NewRubricSnapshotID string   `json:"new_rubric_snapshot_id,omitempty"`
	NewPolicyVersion    string   `json:"new_policy_version,omitempty"`
	SeverityDelta       *float64 `json:"severity_delta,omitempty"`
	IdempotencyKey      string   `json:"idempotency_key"`
	AssigneeID          string   `json:"assignee_id,omitempty"`
}

type CandidateInput struct {
	Score               float64           `json:"score"`
	CandidateGradeID    string            `json:"candidate_grade_id,omitempty"`
	RubricSelections    []RubricSelection `json:"rubric_selections"`
	Comment             string            `json:"comment,omitempty"`
	ExpectedRevision    int64             `json:"expected_revision"`
	RequireManualReview bool              `json:"require_manual_review"`
}

type RubricSelection struct {
	PointID string  `json:"point_id"`
	Score   float64 `json:"score"`
}

type ReviewInput struct {
	Decision         string   `json:"decision"`
	ReviewedScore    *float64 `json:"reviewed_score,omitempty"`
	ReviewedGradeID  string   `json:"reviewed_grade_id,omitempty"`
	Note             string   `json:"note,omitempty"`
	ExpectedRevision int64    `json:"expected_revision"`
}

type Store interface {
	Preview(context.Context, string, string, string, string, Selector) (Preview, error)
	Create(context.Context, string, string, string, string, CreateInput, []SourceItem) (Job, []Item, error)
	Get(context.Context, string, string) (Summary, error)
	List(context.Context, string, string, string) ([]Job, error)
	ListAssigned(context.Context, string, string) ([]Item, error)
	Approve(context.Context, string, string, string) (Job, error)
	Start(context.Context, string, string, string) (Job, error)
	Pause(context.Context, string, string, string) (Job, error)
	Resume(context.Context, string, string, string) (Job, error)
	Claim(context.Context, string, string, string) (Item, error)
	RecordCandidate(context.Context, string, string, string, CandidateInput) (Item, error)
	Review(context.Context, string, string, string, ReviewInput) (Item, error)
	Finalize(context.Context, string, string, string) (Job, error)
}
