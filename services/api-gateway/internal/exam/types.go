package exam

import (
	"context"
	"errors"
	"time"

	"edugrade-enterprise/services/api-gateway/internal/auth"
)

var (
	ErrNotFound          = errors.New("exam not found")
	ErrInvalidTransition = errors.New("invalid exam status transition")
	ErrLocked            = errors.New("exam is locked")
	ErrInvalidInput      = errors.New("invalid exam input")
	ErrRevisionConflict  = errors.New("exam revision conflict")
	ErrScopeForbidden    = errors.New("exam access scope forbidden")
)

type Exam struct {
	ID            string    `json:"id"`
	TenantID      string    `json:"tenant_id"`
	SchoolID      string    `json:"school_id"`
	Name          string    `json:"name"`
	Subject       string    `json:"subject"`
	ExamType      string    `json:"exam_type"`
	TotalScore    float64   `json:"total_score"`
	Status        string    `json:"status"`
	GradingMode   string    `json:"grading_mode"`
	AppealEnabled bool      `json:"appeal_enabled"`
	PublishPolicy string    `json:"publish_policy"`
	CreatedBy     string    `json:"created_by"`
	ClassIDs      []string  `json:"class_ids"`
	Revision      int64     `json:"revision"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type CreateInput struct {
	SchoolID      string   `json:"school_id"`
	Name          string   `json:"name"`
	Subject       string   `json:"subject"`
	ExamType      string   `json:"exam_type"`
	TotalScore    float64  `json:"total_score"`
	GradingMode   string   `json:"grading_mode"`
	AppealEnabled *bool    `json:"appeal_enabled"`
	PublishPolicy string   `json:"publish_policy"`
	ClassIDs      []string `json:"class_ids"`
}

type UpdateInput struct {
	SchoolID         *string   `json:"school_id"`
	Name             *string   `json:"name"`
	Subject          *string   `json:"subject"`
	ExamType         *string   `json:"exam_type"`
	TotalScore       *float64  `json:"total_score"`
	GradingMode      *string   `json:"grading_mode"`
	AppealEnabled    *bool     `json:"appeal_enabled"`
	PublishPolicy    *string   `json:"publish_policy"`
	ClassIDs         *[]string `json:"class_ids"`
	ExpectedRevision int64     `json:"expected_revision"`
}

type ListFilter struct {
	Status   string
	SchoolID string
	Limit    int
	CursorAt time.Time
	CursorID string
}

type Store interface {
	CreateExam(ctx context.Context, scope auth.AccessScope, createdBy string, input CreateInput) (Exam, error)
	ListExams(ctx context.Context, scope auth.AccessScope, filter ListFilter) ([]Exam, error)
	GetExam(ctx context.Context, scope auth.AccessScope, id string) (Exam, error)
	UpdateExam(ctx context.Context, scope auth.AccessScope, id string, input UpdateInput) (Exam, error)
	UpdateStatus(ctx context.Context, scope auth.AccessScope, id string, status string, expectedRevision int64) (Exam, error)
}
