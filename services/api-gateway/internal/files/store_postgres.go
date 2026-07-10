package files

import (
	"context"
	"database/sql"
	"errors"
)

type PostgresStore struct {
	db *sql.DB
}

func NewPostgresStore(db *sql.DB) *PostgresStore {
	return &PostgresStore{db: db}
}

func (s *PostgresStore) Create(ctx context.Context, input CreateAssetInput) (FileAsset, error) {
	if existing, ok, err := s.FindDuplicate(ctx, input.TenantID, input.OwnerType, input.OwnerID, input.HashSHA256); err != nil {
		return FileAsset{}, err
	} else if ok {
		return existing, ErrDuplicateFile
	}
	row := s.db.QueryRowContext(ctx, `
INSERT INTO file_asset (
  tenant_id, school_id, exam_id, submission_id, owner_type, owner_id,
  original_name, content_type, size_bytes, hash_sha256,
  storage_bucket, storage_key, visibility, uploaded_by
)
VALUES (
  $1, NULLIF($2, '')::uuid, NULLIF($3, '')::uuid, NULLIF($4, '')::uuid, $5, NULLIF($6, '')::uuid,
  $7, $8, $9, $10,
  $11, $12, $13, NULLIF($14, '')::uuid
)
RETURNING id::text, tenant_id::text, COALESCE(school_id::text, ''), COALESCE(exam_id::text, ''),
  COALESCE(submission_id::text, ''), owner_type, COALESCE(owner_id::text, ''),
  original_name, content_type, size_bytes, hash_sha256, storage_bucket, storage_key,
  visibility, COALESCE(uploaded_by::text, ''), created_at
`, input.TenantID, input.SchoolID, input.ExamID, input.SubmissionID, input.OwnerType, input.OwnerID,
		input.OriginalName, input.ContentType, input.SizeBytes, input.HashSHA256,
		input.StorageBucket, input.StorageKey, input.Visibility, input.UploadedBy)
	var asset FileAsset
	if err := scanAsset(row, &asset); err != nil {
		return FileAsset{}, err
	}
	return asset, nil
}

func (s *PostgresStore) FindDuplicate(ctx context.Context, tenantID string, ownerType string, ownerID string, hashSHA256 string) (FileAsset, bool, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id::text, tenant_id::text, COALESCE(school_id::text, ''), COALESCE(exam_id::text, ''),
  COALESCE(submission_id::text, ''), owner_type, COALESCE(owner_id::text, ''),
  original_name, content_type, size_bytes, hash_sha256, storage_bucket, storage_key,
  visibility, COALESCE(uploaded_by::text, ''), created_at
FROM file_asset
WHERE tenant_id = $1
  AND owner_type = $2
  AND (($3 = '' AND owner_id IS NULL) OR owner_id::text = $3)
  AND hash_sha256 = $4
  AND deleted_at IS NULL
ORDER BY created_at DESC
LIMIT 1
`, tenantID, ownerType, ownerID, hashSHA256)
	var asset FileAsset
	if err := scanAsset(row, &asset); err != nil {
		if errors.Is(err, ErrNotFound) {
			return FileAsset{}, false, nil
		}
		return FileAsset{}, false, err
	}
	return asset, true, nil
}

func (s *PostgresStore) Get(ctx context.Context, tenantID string, id string) (FileAsset, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id::text, tenant_id::text, COALESCE(school_id::text, ''), COALESCE(exam_id::text, ''),
  COALESCE(submission_id::text, ''), owner_type, COALESCE(owner_id::text, ''),
  original_name, content_type, size_bytes, hash_sha256, storage_bucket, storage_key,
  visibility, COALESCE(uploaded_by::text, ''), created_at
FROM file_asset
WHERE tenant_id = $1 AND id::text = $2 AND deleted_at IS NULL
`, tenantID, id)
	var asset FileAsset
	if err := scanAsset(row, &asset); err != nil {
		return FileAsset{}, err
	}
	return asset, nil
}

func (s *PostgresStore) Delete(ctx context.Context, tenantID string, id string) (FileAsset, error) {
	row := s.db.QueryRowContext(ctx, `
UPDATE file_asset
SET deleted_at = now(), updated_at = now()
WHERE tenant_id = $1 AND id::text = $2 AND deleted_at IS NULL
RETURNING id::text, tenant_id::text, COALESCE(school_id::text, ''), COALESCE(exam_id::text, ''),
  COALESCE(submission_id::text, ''), owner_type, COALESCE(owner_id::text, ''),
  original_name, content_type, size_bytes, hash_sha256, storage_bucket, storage_key,
  visibility, COALESCE(uploaded_by::text, ''), created_at
`, tenantID, id)
	var asset FileAsset
	if err := scanAsset(row, &asset); err != nil {
		return FileAsset{}, err
	}
	return asset, nil
}

type assetScanner interface {
	Scan(dest ...any) error
}

func scanAsset(row assetScanner, asset *FileAsset) error {
	if err := row.Scan(
		&asset.ID,
		&asset.TenantID,
		&asset.SchoolID,
		&asset.ExamID,
		&asset.SubmissionID,
		&asset.OwnerType,
		&asset.OwnerID,
		&asset.OriginalName,
		&asset.ContentType,
		&asset.SizeBytes,
		&asset.HashSHA256,
		&asset.StorageBucket,
		&asset.StorageKey,
		&asset.Visibility,
		&asset.UploadedBy,
		&asset.CreatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	return nil
}
