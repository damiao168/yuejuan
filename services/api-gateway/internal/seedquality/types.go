package seedquality

import (
	"context"
	"edugrade-enterprise/services/api-gateway/internal/commandreceipt"
	"errors"
	"time"
)

var (
	ErrNotFound            = errors.New("seed quality resource not found")
	ErrInvalidInput        = errors.New("invalid seed quality input")
	ErrConflict            = errors.New("seed quality state conflict")
	ErrGoldSetMissing      = errors.New("active approved Gold Set is required")
	ErrGoldSetChanged      = errors.New("active Gold Set changed after policy configuration")
	ErrQualificationNeeded = errors.New("current grader qualification is required")
	ErrSeedTaskForbidden   = errors.New("seed task does not belong to grader")
)

type PolicyStatus string

const (
	PolicyActive PolicyStatus = "active"
	PolicyPaused PolicyStatus = "paused"
)

// Policy is question scoped. ActiveGoldFingerprint binds sampling to the
// approved Gold versions that were reviewed when the policy was enabled.
type Policy struct {
	ID                    string       `json:"id"`
	ExamID                string       `json:"exam_id"`
	QuestionID            string       `json:"question_id"`
	Rate                  float64      `json:"rate"`
	MinInterval           int          `json:"min_interval"`
	MaxInterval           int          `json:"max_interval"`
	ActiveGoldFingerprint string       `json:"active_gold_fingerprint"`
	Status                PolicyStatus `json:"status"`
	Revision              int64        `json:"revision"`
	CreatedBy             string       `json:"created_by"`
	CreatedAt             time.Time    `json:"created_at"`
	UpdatedAt             time.Time    `json:"updated_at"`
}

type PutPolicyInput struct {
	Rate             float64      `json:"rate"`
	MinInterval      int          `json:"min_interval"`
	MaxInterval      int          `json:"max_interval"`
	Status           PolicyStatus `json:"status"`
	ExpectedRevision int64        `json:"expected_revision"`
}

// Task is deliberately shaped like a normal grading work item. Its JSON omits
// Gold identifiers, reference scores, submission IDs and the fact that it is a
// Seed. The review composition root can return this through the normal claim
// response and route its opaque ID back through TrySubmit.
type Task struct {
	ID             string    `json:"id"`
	ExamID         string    `json:"exam_id"`
	QuestionID     string    `json:"question_id"`
	QuestionNo     string    `json:"question_no"`
	AnonymousCode  string    `json:"anonymous_code"`
	AnswerImageURL string    `json:"answer_image_url"`
	Source         string    `json:"source"`
	Status         string    `json:"status"`
	Priority       int       `json:"priority"`
	AssignedTo     string    `json:"assigned_to"`
	MaxScore       float64   `json:"max_score"`
	Revision       int64     `json:"revision"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`

	GoldPaperID      string         `json:"-"`
	GoldVersion      int            `json:"-"`
	SnapshotID       string         `json:"-"`
	ReferenceScore   float64        `json:"-"`
	ExpectedCriteria map[string]any `json:"-"`
	ArchetypeCode    string         `json:"-"`
	SourceImageURL   string         `json:"-"`
}

type SubmitInput struct {
	Score            float64        `json:"score"`
	RubricSelections map[string]any `json:"rubric_selections"`
	ExpectedRevision int64          `json:"expected_revision"`
}

// SubmitReceipt intentionally reveals neither the reference answer nor whether
// the completed item was a Seed. Quality observations are manager-only facts.
type SubmitReceipt struct {
	TaskID   string `json:"task_id"`
	Status   string `json:"status"`
	Revision int64  `json:"revision"`
}

type ObservationKind string

const (
	ObservationTrait     ObservationKind = "trait"
	ObservationCriterion ObservationKind = "criterion"
)

type Observation struct {
	ID             string  `json:"id"`
	ExamID         string  `json:"exam_id"`
	QuestionID     string  `json:"question_id"`
	GraderID       string  `json:"grader_id"`
	GoldPaperID    string  `json:"gold_paper_id"`
	GoldVersion    int     `json:"gold_version"`
	SubmittedScore float64 `json:"submitted_score"`
	ReferenceScore float64 `json:"reference_score"`
	// MaxScore is server-only quality context. It keeps the A11 middle-score
	// slice honest without making any Gold fact visible on grader routes.
	MaxScore             float64         `json:"-"`
	RubricSelections     map[string]any  `json:"rubric_selections"`
	AbsoluteError        float64         `json:"error"`
	RubricAgreement      *float64        `json:"rubric_agreement,omitempty"`
	ObservationKind      ObservationKind `json:"observation_kind"`
	TraitObservation     map[string]any  `json:"trait_observation,omitempty"`
	CriterionObservation map[string]any  `json:"criterion_observation,omitempty"`
	ObservedAt           time.Time       `json:"observed_at"`
}

type ObservationFilter struct {
	ExamID     string
	QuestionID string
	GraderID   string
	Limit      int
}

type GoldSample struct {
	GoldPaperID      string
	GoldVersion      int
	SnapshotID       string
	AnswerImageURL   string
	ReferenceScore   float64
	MaxScore         float64
	ExpectedCriteria map[string]any
	ArchetypeCode    string
}

type IssueDecision struct {
	Policy       Policy
	QuestionNo   string
	GraderID     string
	Gold         GoldSample
	Probability  float64
	NextInterval int
	Now          time.Time
}

type Store interface {
	RecoverCommand(context.Context, string, string, string) (commandreceipt.Receipt, error)
	PutPolicy(context.Context, string, string, string, string, PutPolicyInput, string) (Policy, error)
	GetPolicy(context.Context, string, string, string) (Policy, error)
	AdvanceAndMaybeCreate(context.Context, string, IssueDecision) (Task, bool, error)
	GetTask(context.Context, string, string) (Task, error)
	CompleteTask(context.Context, string, string, string, SubmitInput, Observation) (Task, Observation, error)
	ListObservations(context.Context, string, ObservationFilter) ([]Observation, error)
}
