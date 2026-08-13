package appeal

import (
	"context"
	"errors"
	"time"
)

// PublishedQuestionAppeal is a separately versioned appeal record.  The
// older Appeal model is retained for backwards-compatible operational data,
// while this record is intentionally anchored to a Score Release snapshot.
// Nothing in this type names a mutable final grade as the authoritative score.
type PublishedQuestionAppeal struct {
	ID                   string         `json:"id"`
	TenantID             string         `json:"tenant_id"`
	ExamID               string         `json:"exam_id"`
	StudentID            string         `json:"student_id,omitempty"`
	SubmissionID         string         `json:"submission_id"`
	SourceReleaseID      string         `json:"source_release_id"`
	SourceReleaseVersion int            `json:"source_release_version"`
	QuestionID           string         `json:"question_id"`
	QuestionNo           string         `json:"question_no"`
	SourceFinalGradeID   string         `json:"-"`
	SourceScore          float64        `json:"source_score"`
	SourceMaxScore       float64        `json:"source_max_score"`
	ReasonCode           string         `json:"reason_code"`
	Reason               string         `json:"reason"`
	SelectedRegion       map[string]any `json:"selected_region,omitempty"`
	Status               string         `json:"status"`
	AssignedTo           string         `json:"assigned_to,omitempty"`
	Decision             string         `json:"decision,omitempty"`
	PublicResponse       string         `json:"public_response,omitempty"`
	PrivateNote          string         `json:"-"`
	RegradeJobID         string         `json:"regrade_job_id,omitempty"`
	NewReleaseID         string         `json:"new_release_id,omitempty"`
	CreatedBy            string         `json:"created_by,omitempty"`
	DecidedBy            string         `json:"decided_by,omitempty"`
	DecidedAt            *time.Time     `json:"decided_at,omitempty"`
	CreatedAt            time.Time      `json:"created_at"`
	UpdatedAt            time.Time      `json:"updated_at"`
	Revision             int64          `json:"revision"`
}

// StudentQuestionAppeal is deliberately narrower than PublishedQuestionAppeal
// so a student cannot receive staff identity, private discussion or regrade
// implementation details while checking the outcome.
type StudentQuestionAppeal struct {
	ID                   string         `json:"id"`
	ExamID               string         `json:"exam_id"`
	SourceReleaseID      string         `json:"source_release_id"`
	SourceReleaseVersion int            `json:"source_release_version"`
	QuestionID           string         `json:"question_id"`
	QuestionNo           string         `json:"question_no"`
	SourceScore          float64        `json:"source_score"`
	SourceMaxScore       float64        `json:"source_max_score"`
	ReasonCode           string         `json:"reason_code"`
	Reason               string         `json:"reason"`
	SelectedRegion       map[string]any `json:"selected_region,omitempty"`
	Status               string         `json:"status"`
	Decision             string         `json:"decision,omitempty"`
	PublicResponse       string         `json:"public_response,omitempty"`
	NewReleaseID         string         `json:"new_release_id,omitempty"`
	CreatedAt            time.Time      `json:"created_at"`
	UpdatedAt            time.Time      `json:"updated_at"`
}

type PublishedQuestionAppealEvent struct {
	ID        string         `json:"id"`
	AppealID  string         `json:"appeal_id"`
	Type      string         `json:"type"`
	ActorID   string         `json:"actor_id,omitempty"`
	Payload   map[string]any `json:"payload,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
}

// QuestionAppealReleaseVersion is a staff-facing audit summary. It exposes
// the release history for the same exam without exposing another student's
// score facts.
type QuestionAppealReleaseVersion struct {
	ID          string     `json:"id"`
	Version     int        `json:"version"`
	Source      string     `json:"source"`
	Status      string     `json:"status"`
	Reason      string     `json:"reason"`
	PublishedAt *time.Time `json:"published_at,omitempty"`
}

// PublishedQuestionAppealContext is the staff work surface for one immutable
// appeal. AnswerSegmentID is intentionally private and is only used by the
// image proxy after staff authorization.
type PublishedQuestionAppealContext struct {
	Appeal           PublishedQuestionAppeal        `json:"appeal"`
	SourceType       string                         `json:"source_type"`
	RubricSnapshot   map[string]any                 `json:"rubric_snapshot,omitempty"`
	ReleaseHistory   []QuestionAppealReleaseVersion `json:"release_history"`
	AnswerSegmentID  string                         `json:"-"`
	AnswerImageReady bool                           `json:"answer_image_ready"`
}

type CreatePublishedQuestionAppealInput struct {
	ExamID          string         `json:"exam_id"`
	SourceReleaseID string         `json:"source_release_id"`
	QuestionID      string         `json:"question_id"`
	ReasonCode      string         `json:"reason_code"`
	Reason          string         `json:"reason"`
	SelectedRegion  map[string]any `json:"selected_region,omitempty"`
}

type StartQuestionAppealReviewInput struct {
	AssignedTo       string `json:"assigned_to"`
	ExpectedRevision int64  `json:"expected_revision"`
}

// DecideQuestionAppealInput contains no score field by design.  An upheld
// appeal can only open a governed regrade, which is then materialised as a
// later Score Release by the release workflow.
type DecideQuestionAppealInput struct {
	Decision         string `json:"decision"`
	PublicResponse   string `json:"public_response"`
	PrivateNote      string `json:"private_note,omitempty"`
	RegradeJobID     string `json:"regrade_job_id,omitempty"`
	ExpectedRevision int64  `json:"expected_revision"`
}

type ResolveQuestionAppealInput struct {
	NewReleaseID     string `json:"new_release_id"`
	PublicResponse   string `json:"public_response,omitempty"`
	PrivateNote      string `json:"private_note,omitempty"`
	ExpectedRevision int64  `json:"expected_revision"`
}

type QuestionAppealFilter struct {
	ExamID     string
	StudentID  string
	AssignedTo string
	Status     string
}

const (
	QuestionAppealSubmitted            = "submitted"
	QuestionAppealUnderReview          = "under_review"
	QuestionAppealRejected             = "rejected"
	QuestionAppealUpheldPendingRegrade = "upheld_pending_regrade"
	QuestionAppealResolved             = "resolved"

	QuestionAppealDecisionReject       = "reject"
	QuestionAppealDecisionReferRegrade = "refer_regrade"

	ReasonRecognitionError   = "recognition_error"
	ReasonMissingStepCredit  = "missing_step_credit"
	ReasonRubricDisagreement = "rubric_disagreement"
	ReasonCalculationError   = "calculation_error"
	ReasonAnnotationIssue    = "annotation_issue"
	ReasonOther              = "other"
)

var (
	ErrAppealWindowClosed = errors.New("appeal window is closed")
	ErrSourceRelease      = errors.New("appeal source is not a published score release question")
	ErrResolutionRelease  = errors.New("appeal resolution release is invalid")
)

// PublishedQuestionAppealStore owns only immutable-release appeal evidence.
// It must never update score, human_grade or final_grade rows.
type PublishedQuestionAppealStore interface {
	CreatePublishedQuestionAppeal(context.Context, string, string, string, CreatePublishedQuestionAppealInput) (PublishedQuestionAppeal, error)
	GetPublishedQuestionAppeal(context.Context, string, string) (PublishedQuestionAppeal, error)
	GetPublishedQuestionAppealContext(context.Context, string, string) (PublishedQuestionAppealContext, error)
	ListPublishedQuestionAppeals(context.Context, string, QuestionAppealFilter) ([]PublishedQuestionAppeal, error)
	StartPublishedQuestionAppealReview(context.Context, string, string, string, StartQuestionAppealReviewInput) (PublishedQuestionAppeal, error)
	DecidePublishedQuestionAppeal(context.Context, string, string, string, DecideQuestionAppealInput) (PublishedQuestionAppeal, error)
	ResolvePublishedQuestionAppeal(context.Context, string, string, string, ResolveQuestionAppealInput) (PublishedQuestionAppeal, error)
	ListPublishedQuestionAppealEvents(context.Context, string, string) ([]PublishedQuestionAppealEvent, error)
}
