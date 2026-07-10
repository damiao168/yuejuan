package exam

import (
	"context"
	"errors"
)

var (
	ErrNotFound          = errors.New("exam not found")
	ErrInvalidTransition = errors.New("invalid exam status transition")
	ErrLocked            = errors.New("exam is locked")
	ErrInvalidInput      = errors.New("invalid exam input")
)

type Exam struct {
	ID            string   `json:"id"`
	TenantID      string   `json:"tenant_id"`
	SchoolID      string   `json:"school_id"`
	Name          string   `json:"name"`
	Subject       string   `json:"subject"`
	ExamType      string   `json:"exam_type"`
	TotalScore    float64  `json:"total_score"`
	Status        string   `json:"status"`
	GradingMode   string   `json:"grading_mode"`
	AppealEnabled bool     `json:"appeal_enabled"`
	PublishPolicy string   `json:"publish_policy"`
	CreatedBy     string   `json:"created_by"`
	ClassIDs      []string `json:"class_ids"`
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
	SchoolID      *string   `json:"school_id"`
	Name          *string   `json:"name"`
	Subject       *string   `json:"subject"`
	ExamType      *string   `json:"exam_type"`
	TotalScore    *float64  `json:"total_score"`
	GradingMode   *string   `json:"grading_mode"`
	AppealEnabled *bool     `json:"appeal_enabled"`
	PublishPolicy *string   `json:"publish_policy"`
	ClassIDs      *[]string `json:"class_ids"`
}

type ListFilter struct {
	Status   string
	SchoolID string
}

type Store interface {
	CreateExam(ctx context.Context, tenantID string, createdBy string, input CreateInput) (Exam, error)
	ListExams(ctx context.Context, tenantID string, filter ListFilter) ([]Exam, error)
	GetExam(ctx context.Context, tenantID string, id string) (Exam, error)
	UpdateExam(ctx context.Context, tenantID string, id string, input UpdateInput) (Exam, error)
	UpdateStatus(ctx context.Context, tenantID string, id string, status string) (Exam, error)
}
