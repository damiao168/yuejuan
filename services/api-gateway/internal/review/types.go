package review

import (
	"context"
	"errors"
	"time"

	"edugrade-enterprise/services/api-gateway/internal/paper"
)

var (
	ErrNotFound          = errors.New("review resource not found")
	ErrInvalidInput      = errors.New("invalid review input")
	ErrForbidden         = errors.New("review action forbidden")
	ErrInvalidTransition = errors.New("invalid review task transition")
)

type Context struct {
	ExamID          string
	SubmissionID    string
	AnswerSegmentID string
	AnonymousCode   string
	Question        paper.Question
	Rubric          paper.Rubric
	RawAnswer       string
	OCRText         string
	AISuggestion    map[string]any
}

type ReviewTask struct {
	ID              string     `json:"id"`
	TenantID        string     `json:"tenant_id"`
	ExamID          string     `json:"exam_id"`
	QuestionID      string     `json:"question_id"`
	QuestionNo      string     `json:"question_no"`
	AnswerSegmentID string     `json:"answer_segment_id"`
	SubmissionID    string     `json:"submission_id"`
	AnonymousCode   string     `json:"anonymous_code"`
	Source          string     `json:"source"`
	Status          string     `json:"status"`
	Priority        int        `json:"priority"`
	AssignedTo      string     `json:"assigned_to,omitempty"`
	ReturnReason    string     `json:"return_reason,omitempty"`
	GradeRound      string     `json:"grade_round"`
	DueAt           *time.Time `json:"due_at,omitempty"`
	CreatedBy       string     `json:"created_by"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type HumanGrade struct {
	ID               string            `json:"id"`
	TenantID         string            `json:"tenant_id"`
	ReviewTaskID     string            `json:"review_task_id"`
	AnswerSegmentID  string            `json:"answer_segment_id"`
	ReviewerID       string            `json:"reviewer_id"`
	Score            float64           `json:"score"`
	MaxScore         float64           `json:"max_score"`
	RubricSelections []RubricSelection `json:"rubric_selections"`
	Comments         string            `json:"comments,omitempty"`
	PrivateNote      string            `json:"private_note,omitempty"`
	StudentFeedback  string            `json:"student_feedback,omitempty"`
	Reason           string            `json:"reason,omitempty"`
	GradeRound       string            `json:"grade_round"`
	CreatedAt        time.Time         `json:"created_at"`
}

type RubricSelection struct {
	PointID string  `json:"point_id"`
	Score   float64 `json:"score"`
}

type CreateTaskInput struct {
	AnswerSegmentID string     `json:"answer_segment_id"`
	Source          string     `json:"source"`
	Priority        int        `json:"priority"`
	AssignedTo      string     `json:"assigned_to"`
	GradeRound      string     `json:"grade_round"`
	DueAt           *time.Time `json:"due_at"`
}

type AssignTaskInput struct {
	AssignedTo string `json:"assigned_to"`
}

type BatchAssignInput struct {
	TaskIDs    []string `json:"task_ids"`
	AssignedTo string   `json:"assigned_to"`
}

type SubmitGradeInput struct {
	Score            float64           `json:"score"`
	RubricSelections []RubricSelection `json:"rubric_selections"`
	Comments         string            `json:"comments"`
	PrivateNote      string            `json:"private_note"`
	StudentFeedback  string            `json:"student_feedback"`
	Reason           string            `json:"reason"`
}

type ReturnTaskInput struct {
	Reason string `json:"reason"`
}

type SubmitResult struct {
	Task              ReviewTask         `json:"task"`
	Grade             HumanGrade         `json:"human_grade"`
	DoubleMarkSession *DoubleMarkSession `json:"double_mark_session,omitempty"`
	FinalGrade        *FinalGrade        `json:"final_grade,omitempty"`
	ArbitrationTask   *ArbitrationTask   `json:"arbitration_task,omitempty"`
}

type ListFilter struct {
	Status     string
	AssignedTo string
}

type DoubleMarkPolicy struct {
	ID                  string    `json:"id"`
	TenantID            string    `json:"tenant_id"`
	ExamID              string    `json:"exam_id"`
	QuestionID          string    `json:"question_id,omitempty"`
	Enabled             bool      `json:"enabled"`
	Threshold           float64   `json:"threshold"`
	ResolutionStrategy  string    `json:"resolution_strategy"`
	AllowSameArbitrator bool      `json:"allow_same_arbitrator"`
	CreatedBy           string    `json:"created_by"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

type SetDoubleMarkPolicyInput struct {
	Enabled             bool    `json:"enabled"`
	Threshold           float64 `json:"threshold"`
	ResolutionStrategy  string  `json:"resolution_strategy"`
	AllowSameArbitrator bool    `json:"allow_same_arbitrator"`
}

type PolicyFilter struct {
	ExamID     string
	QuestionID string
}

type CreateDoubleMarkSessionInput struct {
	AnswerSegmentID  string     `json:"answer_segment_id"`
	FirstReviewerID  string     `json:"first_reviewer_id"`
	SecondReviewerID string     `json:"second_reviewer_id"`
	Priority         int        `json:"priority"`
	DueAt            *time.Time `json:"due_at"`
}

type DoubleMarkSession struct {
	ID                 string    `json:"id"`
	TenantID           string    `json:"tenant_id"`
	ExamID             string    `json:"exam_id"`
	QuestionID         string    `json:"question_id"`
	QuestionNo         string    `json:"question_no"`
	AnswerSegmentID    string    `json:"answer_segment_id"`
	SubmissionID       string    `json:"submission_id"`
	AnonymousCode      string    `json:"anonymous_code"`
	FirstReviewTaskID  string    `json:"first_review_task_id"`
	SecondReviewTaskID string    `json:"second_review_task_id"`
	FirstReviewerID    string    `json:"first_reviewer_id"`
	SecondReviewerID   string    `json:"second_reviewer_id"`
	Threshold          float64   `json:"threshold"`
	ResolutionStrategy string    `json:"resolution_strategy"`
	Status             string    `json:"status"`
	ScoreDifference    *float64  `json:"score_difference,omitempty"`
	FinalGradeID       string    `json:"final_grade_id,omitempty"`
	ArbitrationTaskID  string    `json:"arbitration_task_id,omitempty"`
	CreatedBy          string    `json:"created_by"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type DoubleMarkSessionFilter struct {
	Status          string
	AnswerSegmentID string
}

type CreateArbitrationTaskInput struct {
	DoubleMarkSessionID string `json:"double_mark_session_id"`
	AssignedTo          string `json:"assigned_to"`
	DifferenceReason    string `json:"difference_reason"`
}

type AssignArbitrationTaskInput struct {
	AssignedTo string `json:"assigned_to"`
}

type SubmitArbitrationInput struct {
	FinalScore      float64 `json:"final_score"`
	Reason          string  `json:"reason"`
	StudentFeedback string  `json:"student_feedback"`
}

type ArbitrationTask struct {
	ID                  string        `json:"id"`
	TenantID            string        `json:"tenant_id"`
	DoubleMarkSessionID string        `json:"double_mark_session_id"`
	ExamID              string        `json:"exam_id"`
	QuestionID          string        `json:"question_id"`
	QuestionNo          string        `json:"question_no"`
	AnswerSegmentID     string        `json:"answer_segment_id"`
	SubmissionID        string        `json:"submission_id"`
	AnonymousCode       string        `json:"anonymous_code"`
	FirstReviewerID     string        `json:"first_reviewer_id"`
	SecondReviewerID    string        `json:"second_reviewer_id"`
	FirstScore          float64       `json:"first_score"`
	SecondScore         float64       `json:"second_score"`
	ScoreDifference     float64       `json:"score_difference"`
	DifferenceReason    string        `json:"difference_reason"`
	Status              string        `json:"status"`
	AssignedTo          string        `json:"assigned_to,omitempty"`
	FinalScore          *float64      `json:"final_score,omitempty"`
	Reason              string        `json:"reason,omitempty"`
	StudentFeedback     string        `json:"student_feedback,omitempty"`
	AllowSameArbitrator bool          `json:"allow_same_arbitrator"`
	Context             ReviewContext `json:"context"`
	CreatedBy           string        `json:"created_by"`
	CreatedAt           time.Time     `json:"created_at"`
	UpdatedAt           time.Time     `json:"updated_at"`
}

type ReviewContext struct {
	RawAnswer    string         `json:"raw_answer,omitempty"`
	OCRText      string         `json:"ocr_text,omitempty"`
	AISuggestion map[string]any `json:"ai_suggestion,omitempty"`
}

type ArbitrationFilter struct {
	Status     string
	AssignedTo string
}

type FinalGrade struct {
	ID                  string    `json:"id"`
	TenantID            string    `json:"tenant_id"`
	ExamID              string    `json:"exam_id"`
	QuestionID          string    `json:"question_id"`
	QuestionNo          string    `json:"question_no"`
	AnswerSegmentID     string    `json:"answer_segment_id"`
	SubmissionID        string    `json:"submission_id"`
	AnonymousCode       string    `json:"anonymous_code"`
	Score               float64   `json:"score"`
	MaxScore            float64   `json:"max_score"`
	Source              string    `json:"source"`
	DoubleMarkSessionID string    `json:"double_mark_session_id,omitempty"`
	ArbitrationTaskID   string    `json:"arbitration_task_id,omitempty"`
	ResolutionStrategy  string    `json:"resolution_strategy,omitempty"`
	Locked              bool      `json:"locked"`
	CreatedBy           string    `json:"created_by"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

type Store interface {
	CreateTask(ctx context.Context, tenantID string, actorID string, input CreateTaskInput) (ReviewTask, error)
	ListTasks(ctx context.Context, tenantID string, filter ListFilter) ([]ReviewTask, error)
	GetTask(ctx context.Context, tenantID string, id string) (ReviewTask, error)
	AssignTask(ctx context.Context, tenantID string, id string, actorID string, input AssignTaskInput) (ReviewTask, error)
	BatchAssignTasks(ctx context.Context, tenantID string, actorID string, input BatchAssignInput) ([]ReviewTask, error)
	SubmitGrade(ctx context.Context, tenantID string, id string, reviewerID string, input SubmitGradeInput) (SubmitResult, error)
	ReturnTask(ctx context.Context, tenantID string, id string, actorID string, input ReturnTaskInput) (ReviewTask, error)
	SetExamDoubleMarkPolicy(ctx context.Context, tenantID string, examID string, actorID string, input SetDoubleMarkPolicyInput) (DoubleMarkPolicy, error)
	SetQuestionDoubleMarkPolicy(ctx context.Context, tenantID string, questionID string, actorID string, input SetDoubleMarkPolicyInput) (DoubleMarkPolicy, error)
	ListDoubleMarkPolicies(ctx context.Context, tenantID string, filter PolicyFilter) ([]DoubleMarkPolicy, error)
	CreateDoubleMarkSession(ctx context.Context, tenantID string, actorID string, input CreateDoubleMarkSessionInput) (DoubleMarkSession, error)
	ListDoubleMarkSessions(ctx context.Context, tenantID string, filter DoubleMarkSessionFilter) ([]DoubleMarkSession, error)
	GetDoubleMarkSession(ctx context.Context, tenantID string, id string) (DoubleMarkSession, error)
	CreateArbitrationTask(ctx context.Context, tenantID string, actorID string, input CreateArbitrationTaskInput) (ArbitrationTask, error)
	ListArbitrationTasks(ctx context.Context, tenantID string, filter ArbitrationFilter) ([]ArbitrationTask, error)
	GetArbitrationTask(ctx context.Context, tenantID string, id string) (ArbitrationTask, error)
	AssignArbitrationTask(ctx context.Context, tenantID string, id string, actorID string, input AssignArbitrationTaskInput) (ArbitrationTask, error)
	SubmitArbitration(ctx context.Context, tenantID string, id string, arbitratorID string, input SubmitArbitrationInput) (ArbitrationTask, FinalGrade, error)
}
