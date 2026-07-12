package segment

import (
	"context"
	"errors"
	"time"
)

var (
	ErrNotFound            = errors.New("answer segment not found")
	ErrInvalidInput        = errors.New("invalid answer segment input")
	ErrNotReady            = errors.New("submission is not ready for answer segmentation")
	ErrEvidenceUnavailable = errors.New("answer segment evidence is unavailable")
)

type Segment struct {
	ID               string     `json:"id"`
	TenantID         string     `json:"tenant_id"`
	SubmissionID     string     `json:"submission_id"`
	SubmissionPageID string     `json:"submission_page_id"`
	QuestionID       string     `json:"question_id"`
	QuestionNo       string     `json:"question_no"`
	BBox             []float64  `json:"bbox"`
	Source           string     `json:"source"`
	Status           string     `json:"status"`
	ReviewNotes      string     `json:"review_notes,omitempty"`
	ReviewedBy       string     `json:"reviewed_by,omitempty"`
	ReviewedAt       *time.Time `json:"reviewed_at,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
}

type SegmentEvidence struct {
	SegmentID              string         `json:"segment_id"`
	SubmissionID           string         `json:"submission_id"`
	SubmissionPageID       string         `json:"submission_page_id"`
	ExamID                 string         `json:"exam_id"`
	QuestionID             string         `json:"question_id"`
	QuestionNo             string         `json:"question_no"`
	TemplateID             string         `json:"template_id"`
	TemplateContentHash    string         `json:"template_content_hash"`
	RegistrationRunID      string         `json:"registration_run_id"`
	RegistrationMethod     string         `json:"registration_method"`
	RegistrationConfidence float64        `json:"registration_confidence"`
	NormalizedBBox         map[string]any `json:"normalized_bbox"`
	PixelBBox              map[string]any `json:"pixel_bbox"`
	CropFileAssetID        string         `json:"-"`
	CorrectionID           string         `json:"-"`
	CropSHA256             string         `json:"crop_sha256"`
	QuestionVersion        int            `json:"question_version"`
	ProcessingStatus       string         `json:"processing_status"`
	RegistrationStatus     string         `json:"registration_status"`
	Confidence             float64        `json:"confidence"`
}

type Issue struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type GenerateResult struct {
	Valid    bool      `json:"valid"`
	Issues   []Issue   `json:"issues"`
	Segments []Segment `json:"segments"`
}

type CreateSegmentInput struct {
	TenantID         string
	SubmissionID     string
	SubmissionPageID string
	QuestionID       string
	QuestionNo       string
	BBox             []float64
	Source           string
	Status           string
}

type UpdateSegmentInput struct {
	BBox        *[]float64 `json:"bbox"`
	Status      *string    `json:"status"`
	ReviewNotes *string    `json:"review_notes"`
}

type Store interface {
	CreateSegments(ctx context.Context, inputs []CreateSegmentInput) ([]Segment, error)
	ListBySubmission(ctx context.Context, tenantID string, submissionID string) ([]Segment, error)
	Update(ctx context.Context, tenantID string, id string, actorID string, input UpdateSegmentInput) (Segment, error)
	GetEvidence(ctx context.Context, tenantID string, id string) (SegmentEvidence, error)
}
