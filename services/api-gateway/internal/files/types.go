package files

import (
	"context"
	"errors"
	"io"
	"time"
)

var (
	ErrNotFound       = errors.New("file asset not found")
	ErrDuplicateFile  = errors.New("duplicate file upload")
	ErrInvalidFile    = errors.New("invalid file")
	ErrStorageFailure = errors.New("object storage failure")
)

type FileAsset struct {
	ID            string
	TenantID      string
	SchoolID      string
	ExamID        string
	SubmissionID  string
	OwnerType     string
	OwnerID       string
	OriginalName  string
	ContentType   string
	SizeBytes     int64
	HashSHA256    string
	StorageBucket string
	StorageKey    string
	Visibility    string
	UploadedBy    string
	CreatedAt     time.Time
	DeletedAt     *time.Time
}

type FileResponse struct {
	ID           string    `json:"id"`
	TenantID     string    `json:"tenant_id"`
	SchoolID     string    `json:"school_id,omitempty"`
	ExamID       string    `json:"exam_id,omitempty"`
	SubmissionID string    `json:"submission_id,omitempty"`
	OwnerType    string    `json:"owner_type"`
	OwnerID      string    `json:"owner_id,omitempty"`
	OriginalName string    `json:"original_name"`
	ContentType  string    `json:"content_type"`
	SizeBytes    int64     `json:"size_bytes"`
	HashSHA256   string    `json:"hash_sha256"`
	Visibility   string    `json:"visibility"`
	UploadedBy   string    `json:"uploaded_by"`
	CreatedAt    time.Time `json:"created_at"`
}

func (a FileAsset) Response() FileResponse {
	return FileResponse{
		ID:           a.ID,
		TenantID:     a.TenantID,
		SchoolID:     a.SchoolID,
		ExamID:       a.ExamID,
		SubmissionID: a.SubmissionID,
		OwnerType:    a.OwnerType,
		OwnerID:      a.OwnerID,
		OriginalName: a.OriginalName,
		ContentType:  a.ContentType,
		SizeBytes:    a.SizeBytes,
		HashSHA256:   a.HashSHA256,
		Visibility:   a.Visibility,
		UploadedBy:   a.UploadedBy,
		CreatedAt:    a.CreatedAt,
	}
}

type CreateAssetInput struct {
	TenantID      string
	SchoolID      string
	ExamID        string
	SubmissionID  string
	OwnerType     string
	OwnerID       string
	OriginalName  string
	ContentType   string
	SizeBytes     int64
	HashSHA256    string
	StorageBucket string
	StorageKey    string
	Visibility    string
	UploadedBy    string
}

type Store interface {
	Create(ctx context.Context, input CreateAssetInput) (FileAsset, error)
	FindDuplicate(ctx context.Context, tenantID string, ownerType string, ownerID string, hashSHA256 string) (FileAsset, bool, error)
	Get(ctx context.Context, tenantID string, id string) (FileAsset, error)
	Delete(ctx context.Context, tenantID string, id string) (FileAsset, error)
}

type ObjectStorage interface {
	Put(ctx context.Context, bucket string, key string, body io.Reader, size int64, contentType string) error
	Get(ctx context.Context, bucket string, key string) (io.ReadCloser, error)
	Remove(ctx context.Context, bucket string, key string) error
}
