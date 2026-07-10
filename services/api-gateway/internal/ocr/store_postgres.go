package ocr

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
)

type PostgresStore struct {
	db *sql.DB
}

func NewPostgresStore(db *sql.DB) *PostgresStore {
	return &PostgresStore{db: db}
}

func (s *PostgresStore) CreateTask(ctx context.Context, tenantID string, submissionID string, actorID string, input CreateTaskInput) (Task, error) {
	input = NormalizeCreateInput(input)
	if err := ValidateCreateInput(input); err != nil {
		return Task{}, err
	}
	row := s.db.QueryRowContext(ctx, `
INSERT INTO ocr_task (tenant_id, submission_id, status, engine, engine_version, min_confidence, requested_by)
VALUES ($1, $2, 'queued', $3, $4, $5, $6)
RETURNING id::text, tenant_id::text, submission_id::text, status, engine, engine_version,
  COALESCE(model_version, ''), COALESCE(config_hash, ''), COALESCE(input_hash, ''),
  COALESCE(duration_ms, 0), COALESCE(worker_id, ''), attempt_count,
  min_confidence::float8, result_count, requires_human_review, COALESCE(error_message, ''),
  requested_by::text, started_at, completed_at, created_at
`, tenantID, submissionID, input.Engine, input.EngineVersion, input.MinConfidence, actorID)
	var task Task
	if err := scanTask(row, &task); err != nil {
		return Task{}, err
	}
	return task, nil
}

func (s *PostgresStore) ListPending(ctx context.Context, tenantID string, limit int) ([]Task, error) {
	if limit <= 0 || limit > 100 {
		limit = 10
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT id::text, tenant_id::text, submission_id::text, status, engine, engine_version,
  COALESCE(model_version, ''), COALESCE(config_hash, ''), COALESCE(input_hash, ''),
  COALESCE(duration_ms, 0), COALESCE(worker_id, ''), attempt_count,
  min_confidence::float8, result_count, requires_human_review, COALESCE(error_message, ''),
  requested_by::text, started_at, completed_at, created_at
FROM ocr_task
WHERE tenant_id = $1 AND status = 'queued' AND deleted_at IS NULL
ORDER BY created_at ASC
LIMIT $2
`, tenantID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Task{}
	for rows.Next() {
		var task Task
		if err := scanTask(rows, &task); err != nil {
			return nil, err
		}
		out = append(out, task)
	}
	return out, rows.Err()
}

func (s *PostgresStore) ListBySubmission(ctx context.Context, tenantID string, submissionID string) ([]Task, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id::text, tenant_id::text, submission_id::text, status, engine, engine_version,
  COALESCE(model_version, ''), COALESCE(config_hash, ''), COALESCE(input_hash, ''),
  COALESCE(duration_ms, 0), COALESCE(worker_id, ''), attempt_count,
  min_confidence::float8, result_count, requires_human_review, COALESCE(error_message, ''),
  requested_by::text, started_at, completed_at, created_at
FROM ocr_task
WHERE tenant_id = $1 AND submission_id = $2 AND deleted_at IS NULL
ORDER BY created_at DESC
`, tenantID, submissionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Task{}
	for rows.Next() {
		var task Task
		if err := scanTask(rows, &task); err != nil {
			return nil, err
		}
		out = append(out, task)
	}
	return out, rows.Err()
}

func (s *PostgresStore) GetTask(ctx context.Context, tenantID string, id string) (Task, error) {
	task, err := s.getTask(ctx, tenantID, id)
	if err != nil {
		return Task{}, err
	}
	results, err := s.results(ctx, tenantID, id)
	if err != nil {
		return Task{}, err
	}
	task.Results = results
	return task, nil
}

func (s *PostgresStore) StartTask(ctx context.Context, tenantID string, id string) (Task, error) {
	task, err := s.getTask(ctx, tenantID, id)
	if err != nil {
		return Task{}, err
	}
	if !CanStart(task.Status) {
		return Task{}, ErrInvalidTransition
	}
	row := s.db.QueryRowContext(ctx, `
UPDATE ocr_task
SET status = 'processing', started_at = now(), updated_at = now()
WHERE tenant_id = $1 AND id::text = $2 AND deleted_at IS NULL
RETURNING id::text, tenant_id::text, submission_id::text, status, engine, engine_version,
  COALESCE(model_version, ''), COALESCE(config_hash, ''), COALESCE(input_hash, ''),
  COALESCE(duration_ms, 0), COALESCE(worker_id, ''), attempt_count,
  min_confidence::float8, result_count, requires_human_review, COALESCE(error_message, ''),
  requested_by::text, started_at, completed_at, created_at
`, tenantID, id)
	var out Task
	if err := scanTask(row, &out); err != nil {
		return Task{}, err
	}
	return out, nil
}

func (s *PostgresStore) CompleteTask(ctx context.Context, tenantID string, id string, input CompleteTaskInput) (Task, error) {
	task, err := s.getTask(ctx, tenantID, id)
	if err != nil {
		return Task{}, err
	}
	if !CanComplete(task.Status) {
		return Task{}, ErrInvalidTransition
	}
	if len(input.Results) == 0 {
		return Task{}, ErrInvalidInput
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Task{}, err
	}
	defer tx.Rollback()
	requiresReview := false
	resultCount := 0
	for _, item := range input.Results {
		if err := ValidateResultInput(item); err != nil {
			return Task{}, err
		}
		if err := ensurePageBelongsToSubmission(ctx, tx, tenantID, task.SubmissionID, item.SubmissionPageID); err != nil {
			return Task{}, err
		}
		if item.Confidence < task.MinConfidence {
			requiresReview = true
		}
		bbox, _ := json.Marshal(item.BBox)
		if _, err := tx.ExecContext(ctx, `
INSERT INTO ocr_result (
  tenant_id, ocr_task_id, submission_id, submission_page_id,
  text, bbox, confidence, ocr_engine, ocr_version,
  model_version, config_hash, input_hash, preprocess_profile, source_image_file_id
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NULLIF($10, ''), NULLIF($11, ''), NULLIF($12, ''), NULLIF($13, ''), NULLIF($14, '')::uuid)
`, tenantID, id, task.SubmissionID, item.SubmissionPageID,
			item.Text, bbox, item.Confidence, task.Engine, task.EngineVersion,
			input.ModelVersion, input.ConfigHash, input.InputHash, input.PreprocessProfile, item.SourceImageFileID); err != nil {
			return Task{}, err
		}
		resultCount++
	}
	row := tx.QueryRowContext(ctx, `
UPDATE ocr_task
SET status = 'completed',
  result_count = $3,
  requires_human_review = $4,
  model_version = NULLIF($5, ''),
  config_hash = NULLIF($6, ''),
  input_hash = NULLIF($7, ''),
  duration_ms = $8,
  worker_id = NULLIF($9, ''),
  attempt_count = attempt_count + 1,
  completed_at = now(),
  updated_at = now()
WHERE tenant_id = $1 AND id::text = $2 AND deleted_at IS NULL
RETURNING id::text, tenant_id::text, submission_id::text, status, engine, engine_version,
  COALESCE(model_version, ''), COALESCE(config_hash, ''), COALESCE(input_hash, ''),
  COALESCE(duration_ms, 0), COALESCE(worker_id, ''), attempt_count,
  min_confidence::float8, result_count, requires_human_review, COALESCE(error_message, ''),
  requested_by::text, started_at, completed_at, created_at
`, tenantID, id, resultCount, requiresReview, input.ModelVersion, input.ConfigHash, input.InputHash, input.DurationMS, input.WorkerID)
	var out Task
	if err := scanTask(row, &out); err != nil {
		return Task{}, err
	}
	if err := tx.Commit(); err != nil {
		return Task{}, err
	}
	results, err := s.results(ctx, tenantID, id)
	if err != nil {
		return Task{}, err
	}
	out.Results = results
	return out, nil
}

func (s *PostgresStore) FailTask(ctx context.Context, tenantID string, id string, errorMessage string) (Task, error) {
	task, err := s.getTask(ctx, tenantID, id)
	if err != nil {
		return Task{}, err
	}
	if !CanFail(task.Status) {
		return Task{}, ErrInvalidTransition
	}
	row := s.db.QueryRowContext(ctx, `
UPDATE ocr_task
SET status = 'failed', error_message = $3, completed_at = now(), updated_at = now()
WHERE tenant_id = $1 AND id::text = $2 AND deleted_at IS NULL
RETURNING id::text, tenant_id::text, submission_id::text, status, engine, engine_version,
  COALESCE(model_version, ''), COALESCE(config_hash, ''), COALESCE(input_hash, ''),
  COALESCE(duration_ms, 0), COALESCE(worker_id, ''), attempt_count,
  min_confidence::float8, result_count, requires_human_review, COALESCE(error_message, ''),
  requested_by::text, started_at, completed_at, created_at
`, tenantID, id, errorMessage)
	var out Task
	if err := scanTask(row, &out); err != nil {
		return Task{}, err
	}
	return out, nil
}

func (s *PostgresStore) getTask(ctx context.Context, tenantID string, id string) (Task, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id::text, tenant_id::text, submission_id::text, status, engine, engine_version,
  COALESCE(model_version, ''), COALESCE(config_hash, ''), COALESCE(input_hash, ''),
  COALESCE(duration_ms, 0), COALESCE(worker_id, ''), attempt_count,
  min_confidence::float8, result_count, requires_human_review, COALESCE(error_message, ''),
  requested_by::text, started_at, completed_at, created_at
FROM ocr_task
WHERE tenant_id = $1 AND id::text = $2 AND deleted_at IS NULL
`, tenantID, id)
	var task Task
	if err := scanTask(row, &task); err != nil {
		return Task{}, err
	}
	return task, nil
}

func (s *PostgresStore) results(ctx context.Context, tenantID string, taskID string) ([]Result, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id::text, tenant_id::text, ocr_task_id::text, submission_id::text, submission_page_id::text,
  text, bbox, confidence::float8, ocr_engine, ocr_version,
  COALESCE(model_version, ''), COALESCE(config_hash, ''), COALESCE(input_hash, ''),
  COALESCE(preprocess_profile, ''), COALESCE(source_image_file_id::text, ''), created_at
FROM ocr_result
WHERE tenant_id = $1 AND ocr_task_id::text = $2 AND deleted_at IS NULL
ORDER BY created_at, id
`, tenantID, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Result{}
	for rows.Next() {
		var result Result
		if err := scanResult(rows, &result); err != nil {
			return nil, err
		}
		out = append(out, result)
	}
	return out, rows.Err()
}

func ensurePageBelongsToSubmission(ctx context.Context, tx *sql.Tx, tenantID string, submissionID string, pageID string) error {
	var exists int
	err := tx.QueryRowContext(ctx, `
SELECT 1
FROM submission_page
WHERE tenant_id = $1 AND submission_id = $2 AND id::text = $3 AND deleted_at IS NULL
`, tenantID, submissionID, pageID).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrInvalidInput
	}
	return err
}

type taskScanner interface {
	Scan(dest ...any) error
}

func scanTask(row taskScanner, out *Task) error {
	var startedAt, completedAt sql.NullTime
	if err := row.Scan(
		&out.ID,
		&out.TenantID,
		&out.SubmissionID,
		&out.Status,
		&out.Engine,
		&out.EngineVersion,
		&out.ModelVersion,
		&out.ConfigHash,
		&out.InputHash,
		&out.DurationMS,
		&out.WorkerID,
		&out.AttemptCount,
		&out.MinConfidence,
		&out.ResultCount,
		&out.RequiresHumanReview,
		&out.ErrorMessage,
		&out.RequestedBy,
		&startedAt,
		&completedAt,
		&out.CreatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if startedAt.Valid {
		value := startedAt.Time.UTC()
		out.StartedAt = &value
	}
	if completedAt.Valid {
		value := completedAt.Time.UTC()
		out.CompletedAt = &value
	}
	return nil
}

func scanResult(row taskScanner, out *Result) error {
	var bbox []byte
	if err := row.Scan(
		&out.ID,
		&out.TenantID,
		&out.TaskID,
		&out.SubmissionID,
		&out.SubmissionPageID,
		&out.Text,
		&bbox,
		&out.Confidence,
		&out.OCREngine,
		&out.OCRVersion,
		&out.ModelVersion,
		&out.ConfigHash,
		&out.InputHash,
		&out.PreprocessProfile,
		&out.SourceImageFileID,
		&out.CreatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	_ = json.Unmarshal(bbox, &out.BBox)
	return nil
}
