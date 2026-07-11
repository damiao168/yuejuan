package capture

import (
	"context"
	"errors"
	"time"
)

var (
	ErrNotFound          = errors.New("capture resource not found")
	ErrInvalidInput      = errors.New("invalid capture input")
	ErrInvalidTransition = errors.New("invalid capture transition")
	ErrConflict          = errors.New("capture revision conflict")
	ErrDuplicateFile     = errors.New("capture file already registered")
)

type Batch struct {
	ID              string     `json:"id"`
	TenantID        string     `json:"tenant_id"`
	ExamID          string     `json:"exam_id"`
	Name            string     `json:"name"`
	SourceType      string     `json:"source_type"`
	Status          string     `json:"status"`
	Revision        int        `json:"revision"`
	OperatorID      string     `json:"operator_id"`
	ScannerDevice   string     `json:"scanner_device,omitempty"`
	FileCount       int        `json:"file_count"`
	PageCount       int        `json:"page_count"`
	SubmissionCount int        `json:"submission_count"`
	NormalCount     int        `json:"normal_count"`
	ReviewCount     int        `json:"review_count"`
	FailedCount     int        `json:"failed_count"`
	StartedAt       *time.Time `json:"started_at,omitempty"`
	CompletedAt     *time.Time `json:"completed_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
}

type File struct {
	ID             string    `json:"id"`
	TenantID       string    `json:"tenant_id"`
	CaptureBatchID string    `json:"capture_batch_id"`
	FileAssetID    string    `json:"file_asset_id"`
	OriginalName   string    `json:"original_name"`
	ContentType    string    `json:"content_type"`
	SHA256         string    `json:"sha256"`
	ByteSize       int64     `json:"byte_size"`
	PageCount      int       `json:"page_count"`
	Status         string    `json:"status"`
	ErrorCode      string    `json:"error_code,omitempty"`
	IdempotencyKey string    `json:"idempotency_key"`
	UploadedBy     string    `json:"uploaded_by"`
	CreatedAt      time.Time `json:"created_at"`
}

type Page struct {
	ID                 string         `json:"id"`
	TenantID           string         `json:"tenant_id"`
	CaptureBatchID     string         `json:"capture_batch_id"`
	CaptureFileID      string         `json:"capture_file_id"`
	SourceIndex        int            `json:"source_index"`
	SubmissionID       string         `json:"submission_id,omitempty"`
	SubmissionPageID   string         `json:"submission_page_id,omitempty"`
	AssignedPageNo     int            `json:"assigned_page_no,omitempty"`
	SequenceNo         int            `json:"sequence_no"`
	RotationDegrees    int            `json:"rotation_degrees"`
	DecodedFileAssetID string         `json:"decoded_file_asset_id"`
	Status             string         `json:"status"`
	DuplicateOfPageID  string         `json:"duplicate_of_page_id,omitempty"`
	Revision           int            `json:"revision"`
	PageIdentity       map[string]any `json:"page_identity"`
	MatchCandidates    []any          `json:"match_candidates"`
	ManualOverride     map[string]any `json:"manual_override"`
	CreatedAt          time.Time      `json:"created_at"`
}

type CreateBatchInput struct {
	Name           string `json:"name"`
	SourceType     string `json:"source_type"`
	ScannerDevice  string `json:"scanner_device"`
	IdempotencyKey string `json:"idempotency_key"`
}

type RegisterFileInput struct {
	FileAssetID    string `json:"file_asset_id"`
	IdempotencyKey string `json:"idempotency_key"`
}

type UpdatePageInput struct {
	Revision        int  `json:"revision"`
	RotationDegrees *int `json:"rotation_degrees,omitempty"`
	SequenceNo      *int `json:"sequence_no,omitempty"`
}

type DecodedPageInput struct {
	SourceIndex int    `json:"source_index"`
	FileAssetID string `json:"file_asset_id"`
	SHA256      string `json:"sha256"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
}

type FileResultInput struct {
	TaskID              string             `json:"task_id"`
	LeaseToken          string             `json:"lease_token"`
	ResultVersion       string             `json:"result_version"`
	DurationMS          int                `json:"duration_ms"`
	DecoderProfile      string             `json:"decoder_profile"`
	Pages               []DecodedPageInput `json:"pages"`
	OriginalPageCount   int                `json:"original_page_count"`
	DetectedContentType string             `json:"detected_content_type"`
}

type FileFailureInput struct {
	TaskID      string         `json:"task_id"`
	LeaseToken  string         `json:"lease_token"`
	Retryable   bool           `json:"retryable"`
	ErrorCode   string         `json:"error_code"`
	ErrorDetail map[string]any `json:"error_detail"`
	DurationMS  int            `json:"duration_ms"`
}

type RegistrationRun struct {
	ID                    string    `json:"id"`
	CapturePageID         string    `json:"capture_page_id"`
	SubmissionPageID      string    `json:"submission_page_id"`
	SourceFileAssetID     string    `json:"source_file_asset_id"`
	TemplateID            string    `json:"template_id"`
	TemplateContentHash   string    `json:"template_content_hash"`
	PageNo                int       `json:"page_no"`
	ProcessingStatus      string    `json:"processing_status"`
	MatchStatus           string    `json:"match_status,omitempty"`
	Confidence            float64   `json:"confidence,omitempty"`
	Method                string    `json:"method,omitempty"`
	ProfileVersion        string    `json:"profile_version"`
	SourceToTemplate      []any     `json:"source_to_template_matrix"`
	TemplateToSource      []any     `json:"template_to_source_matrix"`
	FeatureCount          int       `json:"feature_count"`
	MatchCount            int       `json:"match_count"`
	InlierCount           int       `json:"inlier_count"`
	InlierRatio           float64   `json:"inlier_ratio,omitempty"`
	ReprojectionError     float64   `json:"reprojection_error,omitempty"`
	RegisteredFileAssetID string    `json:"registered_file_asset_id,omitempty"`
	RuntimeTaskID         string    `json:"runtime_task_id,omitempty"`
	ErrorCode             string    `json:"error_code,omitempty"`
	CreatedAt             time.Time `json:"created_at"`
}

type SegmentCropInput struct {
	QuestionID     string         `json:"question_id"`
	Label          string         `json:"label"`
	NormalizedBBox map[string]any `json:"normalized_bbox"`
	PixelBBox      map[string]any `json:"pixel_bbox"`
	FileAssetID    string         `json:"file_asset_id"`
	SHA256         string         `json:"sha256"`
}

type RegistrationResultInput struct {
	TaskID                string             `json:"task_id"`
	LeaseToken            string             `json:"lease_token"`
	ResultVersion         string             `json:"result_version"`
	DurationMS            int                `json:"duration_ms"`
	RegisteredFileAssetID string             `json:"registered_file_asset_id"`
	RegisteredSHA256      string             `json:"registered_sha256"`
	Method                string             `json:"method"`
	Confidence            float64            `json:"confidence"`
	SourceToTemplate      []any              `json:"source_to_template_matrix"`
	TemplateToSource      []any              `json:"template_to_source_matrix"`
	FeatureCount          int                `json:"feature_count"`
	MatchCount            int                `json:"match_count"`
	InlierCount           int                `json:"inlier_count"`
	InlierRatio           float64            `json:"inlier_ratio"`
	ReprojectionError     float64            `json:"reprojection_error"`
	Coverage              float64            `json:"coverage"`
	Segments              []SegmentCropInput `json:"segments"`
}

type RegistrationFailureInput struct {
	TaskID      string         `json:"task_id"`
	LeaseToken  string         `json:"lease_token"`
	Retryable   bool           `json:"retryable"`
	ErrorCode   string         `json:"error_code"`
	ErrorDetail map[string]any `json:"error_detail"`
	DurationMS  int            `json:"duration_ms"`
}

type BatchDetail struct {
	Batch Batch  `json:"batch"`
	Files []File `json:"files"`
	Pages []Page `json:"pages"`
}

type Store interface {
	CreateBatch(ctx context.Context, tenantID, examID, actorID string, input CreateBatchInput) (Batch, error)
	ListBatches(ctx context.Context, tenantID, examID string) ([]Batch, error)
	GetBatch(ctx context.Context, tenantID, batchID string) (Batch, error)
	RegisterFile(ctx context.Context, tenantID, batchID, actorID string, input RegisterFileInput, asset FileAssetSnapshot) (File, error)
	GetFile(ctx context.Context, tenantID, fileID string) (File, error)
	ListFiles(ctx context.Context, tenantID, batchID string) ([]File, error)
	ListPages(ctx context.Context, tenantID, batchID string) ([]Page, error)
	QueueBatch(ctx context.Context, tenantID, batchID, actorID string) (Batch, error)
	ApplyFileResult(ctx context.Context, tenantID, fileID string, pages []DecodedPageInput) (File, error)
	ApplyFileFailure(ctx context.Context, tenantID, fileID, errorCode string, retryable bool) (File, error)
	UpdatePage(ctx context.Context, tenantID, pageID, actorID string, input UpdatePageInput) (Page, error)
	SetBatchStatus(ctx context.Context, tenantID, batchID, actorID, status, reason string) (Batch, error)
	QueueSubmissionPages(ctx context.Context, tenantID, submissionID, actorID string) ([]RegistrationRun, error)
	GetRegistrationRun(ctx context.Context, tenantID, runID string) (RegistrationRun, error)
	ListRegistrationRuns(ctx context.Context, tenantID, submissionPageID string) ([]RegistrationRun, error)
	ApplyRegistrationResult(ctx context.Context, tenantID, runID string, input RegistrationResultInput) (RegistrationRun, error)
	ApplyRegistrationFailure(ctx context.Context, tenantID, runID, errorCode string, errorDetail map[string]any, retryable bool) (RegistrationRun, error)
}

type FileAssetSnapshot struct {
	ID           string
	ExamID       string
	OriginalName string
	ContentType  string
	SizeBytes    int64
	SHA256       string
}
