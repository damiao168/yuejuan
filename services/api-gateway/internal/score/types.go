package score

import (
	"context"
	"errors"
	"time"
)

var (
	ErrNotFound          = errors.New("score resource not found")
	ErrInvalidInput      = errors.New("invalid score input")
	ErrInvalidTransition = errors.New("invalid score transition")
	ErrQualityGateFailed = errors.New("score quality gate failed")
)

type FinalGrade struct {
	ID              string    `json:"id"`
	TenantID        string    `json:"tenant_id"`
	ExamID          string    `json:"exam_id"`
	QuestionID      string    `json:"question_id"`
	QuestionNo      string    `json:"question_no"`
	AnswerSegmentID string    `json:"answer_segment_id"`
	SubmissionID    string    `json:"submission_id"`
	AnonymousCode   string    `json:"anonymous_code"`
	Score           float64   `json:"score"`
	MaxScore        float64   `json:"max_score"`
	Source          string    `json:"source"`
	Status          string    `json:"status"`
	Locked          bool      `json:"locked"`
	CreatedBy       string    `json:"created_by"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type SubmissionGrade struct {
	ID            string       `json:"id"`
	TenantID      string       `json:"tenant_id"`
	ExamID        string       `json:"exam_id"`
	SubmissionID  string       `json:"submission_id"`
	StudentID     string       `json:"student_id,omitempty"`
	AnonymousCode string       `json:"anonymous_code"`
	TotalScore    float64      `json:"total_score"`
	MaxScore      float64      `json:"max_score"`
	Status        string       `json:"status"`
	Locked        bool         `json:"locked"`
	ConfirmedBy   string       `json:"confirmed_by,omitempty"`
	ConfirmedAt   *time.Time   `json:"confirmed_at,omitempty"`
	PublishedBy   string       `json:"published_by,omitempty"`
	PublishedAt   *time.Time   `json:"published_at,omitempty"`
	CreatedBy     string       `json:"created_by"`
	CreatedAt     time.Time    `json:"created_at"`
	UpdatedAt     time.Time    `json:"updated_at"`
	Items         []FinalGrade `json:"items,omitempty"`
}

type QualityIssue struct {
	Code     string `json:"code"`
	Message  string `json:"message"`
	Blocking bool   `json:"blocking"`
	Count    int    `json:"count"`
}

type QualityReport struct {
	Passed bool           `json:"passed"`
	Issues []QualityIssue `json:"issues"`
}

type FinalizeResult struct {
	Status            string            `json:"status"`
	CreatedFinals     int               `json:"created_finals"`
	SubmissionGrades  []SubmissionGrade `json:"submission_grades"`
	Quality           QualityReport     `json:"quality"`
	AvailableStatuses []string          `json:"available_statuses"`
}

type ConfirmInput struct {
	Reason string `json:"reason"`
}

type PublishInput struct {
	Reason string `json:"reason"`
}

type PublishResult struct {
	Status           string            `json:"status"`
	SubmissionGrades []SubmissionGrade `json:"submission_grades"`
	Quality          QualityReport     `json:"quality"`
	PublishedAt      time.Time         `json:"published_at"`
}

type ExportResult struct {
	Filename    string
	ContentType string
	Content     []byte
	RowCount    int
	Watermark   string
}

type Store interface {
	FinalizeExam(ctx context.Context, tenantID string, examID string, actorID string) (FinalizeResult, error)
	ListExamGrades(ctx context.Context, tenantID string, examID string) ([]SubmissionGrade, error)
	CheckQuality(ctx context.Context, tenantID string, examID string, requirePendingPublish bool) (QualityReport, error)
	ConfirmGrades(ctx context.Context, tenantID string, examID string, actorID string, input ConfirmInput) ([]SubmissionGrade, error)
	PublishGrades(ctx context.Context, tenantID string, examID string, actorID string, input PublishInput) (PublishResult, error)
	GetStudentGrade(ctx context.Context, tenantID string, studentID string, examID string) (SubmissionGrade, error)
	ExportGradesCSV(ctx context.Context, tenantID string, examID string, actorID string) (ExportResult, error)
}

func Statuses() []string {
	return []string{"calculating", "pending_confirmation", "confirmed", "pending_publish", "published", "locked"}
}
