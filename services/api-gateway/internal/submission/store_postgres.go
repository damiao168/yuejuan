package submission

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

func (s *PostgresStore) Create(ctx context.Context, tenantID string, examID string, actorID string, input CreateSubmissionInput) (Submission, error) {
	if err := ValidateCreateInput(input); err != nil {
		return Submission{}, err
	}
	row := s.db.QueryRowContext(ctx, `
INSERT INTO submission (
  tenant_id, exam_id, student_id, candidate_no, source_type, status,
  expected_page_count, actual_page_count, quality_status, quality_issues, collected_by
)
VALUES (
  $1, $2, NULLIF($3, '')::uuid, NULLIF($4, ''), $5, 'created',
  $6, 0, 'unchecked', '[]', $7
)
RETURNING id::text, tenant_id::text, exam_id::text, COALESCE(student_id::text, ''),
  COALESCE(candidate_no, ''), source_type, status, expected_page_count, actual_page_count,
  quality_status, quality_issues, collected_by::text, created_at
`, tenantID, examID, input.StudentID, input.CandidateNo, input.SourceType, input.ExpectedPageCount, actorID)
	var out Submission
	if err := scanSubmission(row, &out); err != nil {
		return Submission{}, err
	}
	return out, nil
}

func (s *PostgresStore) ListByExam(ctx context.Context, tenantID string, examID string) ([]Submission, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id::text, tenant_id::text, exam_id::text, COALESCE(student_id::text, ''),
  COALESCE(candidate_no, ''), source_type, status, expected_page_count, actual_page_count,
  quality_status, quality_issues, collected_by::text, created_at
FROM submission
WHERE tenant_id = $1 AND exam_id = $2 AND deleted_at IS NULL
ORDER BY created_at DESC
`, tenantID, examID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Submission{}
	for rows.Next() {
		var item Submission
		if err := scanSubmission(rows, &item); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *PostgresStore) Get(ctx context.Context, tenantID string, id string) (Submission, error) {
	item, err := s.getSubmission(ctx, tenantID, id)
	if err != nil {
		return Submission{}, err
	}
	pages, err := s.ListPages(ctx, tenantID, id)
	if err != nil {
		return Submission{}, err
	}
	item.Pages = pages
	return item, nil
}

func (s *PostgresStore) AddPage(ctx context.Context, tenantID string, submissionID string, _ string, input AddPageInput) (SubmissionPage, error) {
	if err := ValidatePageInput(input); err != nil {
		return SubmissionPage{}, err
	}
	item, err := s.getSubmission(ctx, tenantID, submissionID)
	if err != nil {
		return SubmissionPage{}, err
	}
	if item.Status == "ready_for_ocr" || item.Status == "rejected" {
		return SubmissionPage{}, ErrSubmissionLocked
	}
	if _, err := s.pageByNo(ctx, tenantID, submissionID, input.PageNo); err == nil {
		return SubmissionPage{}, ErrDuplicatePage
	} else if !errors.Is(err, ErrNotFound) {
		return SubmissionPage{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return SubmissionPage{}, err
	}
	defer tx.Rollback()
	row := tx.QueryRowContext(ctx, `
INSERT INTO submission_page (tenant_id, submission_id, file_asset_id, page_no, status, quality_status, quality_override, quality_issues)
VALUES ($1, $2, $3, $4, 'uploaded', 'unchecked', '{}', '[]')
RETURNING id::text, tenant_id::text, submission_id::text, file_asset_id::text, page_no, status,
  COALESCE(latest_quality_run_id::text, ''), COALESCE(normalized_file_asset_id::text, ''),
  quality_status, quality_override, quality_issues, created_at
`, tenantID, submissionID, input.FileAssetID, input.PageNo)
	var page SubmissionPage
	if err := scanPage(row, &page); err != nil {
		return SubmissionPage{}, err
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE submission
SET actual_page_count = (
    SELECT COUNT(*) FROM submission_page
    WHERE tenant_id = $1 AND submission_id = $2 AND deleted_at IS NULL
  ),
  status = 'pages_uploaded',
  quality_status = 'unchecked',
  quality_issues = '[]',
  updated_at = now()
WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL
`, tenantID, submissionID); err != nil {
		return SubmissionPage{}, err
	}
	if err := tx.Commit(); err != nil {
		return SubmissionPage{}, err
	}
	return page, nil
}

func (s *PostgresStore) ReplacePage(ctx context.Context, tenantID string, submissionID string, _ string, pageNo int, input AddPageInput) (SubmissionPage, error) {
	input.PageNo = pageNo
	if err := ValidatePageInput(input); err != nil {
		return SubmissionPage{}, err
	}
	item, err := s.getSubmission(ctx, tenantID, submissionID)
	if err != nil {
		return SubmissionPage{}, err
	}
	if item.Status == "ready_for_ocr" || item.Status == "rejected" {
		return SubmissionPage{}, ErrSubmissionLocked
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return SubmissionPage{}, err
	}
	defer tx.Rollback()
	row := tx.QueryRowContext(ctx, `
UPDATE submission_page
SET file_asset_id = $4,
  status = 'uploaded',
  latest_quality_run_id = NULL,
  normalized_file_asset_id = NULL,
  quality_status = 'unchecked',
  quality_override = '{}',
  quality_issues = '[]',
  updated_at = now()
WHERE tenant_id = $1 AND submission_id = $2 AND page_no = $3 AND deleted_at IS NULL
RETURNING id::text, tenant_id::text, submission_id::text, file_asset_id::text, page_no, status,
  COALESCE(latest_quality_run_id::text, ''), COALESCE(normalized_file_asset_id::text, ''),
  quality_status, quality_override, quality_issues, created_at
`, tenantID, submissionID, pageNo, input.FileAssetID)
	var page SubmissionPage
	if err := scanPage(row, &page); err != nil {
		return SubmissionPage{}, err
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE submission
SET actual_page_count = (
    SELECT COUNT(*) FROM submission_page
    WHERE tenant_id = $1 AND submission_id = $2 AND deleted_at IS NULL
  ),
  status = 'pages_uploaded',
  quality_status = 'unchecked',
  quality_issues = '[]',
  updated_at = now()
WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL
`, tenantID, submissionID); err != nil {
		return SubmissionPage{}, err
	}
	if err := tx.Commit(); err != nil {
		return SubmissionPage{}, err
	}
	return page, nil
}

func (s *PostgresStore) ListPages(ctx context.Context, tenantID string, submissionID string) ([]SubmissionPage, error) {
	if _, err := s.getSubmission(ctx, tenantID, submissionID); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT id::text, tenant_id::text, submission_id::text, file_asset_id::text, page_no, status,
  COALESCE(latest_quality_run_id::text, ''), COALESCE(normalized_file_asset_id::text, ''),
  quality_status, quality_override, quality_issues, created_at
FROM submission_page
WHERE tenant_id = $1 AND submission_id = $2 AND deleted_at IS NULL
ORDER BY page_no
`, tenantID, submissionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SubmissionPage{}
	for rows.Next() {
		var page SubmissionPage
		if err := scanPage(rows, &page); err != nil {
			return nil, err
		}
		out = append(out, page)
	}
	return out, rows.Err()
}

func (s *PostgresStore) ApplyPageQualityResult(ctx context.Context, tenantID string, input ApplyPageQualityInput) (SubmissionPage, error) {
	if input.QualityStatus != "passed" && input.QualityStatus != "review" && input.QualityStatus != "failed" {
		return SubmissionPage{}, ErrInvalidInput
	}
	issuesJSON, _ := json.Marshal(input.QualityIssues)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return SubmissionPage{}, err
	}
	defer tx.Rollback()
	row := tx.QueryRowContext(ctx, `
UPDATE submission_page
SET latest_quality_run_id = NULLIF($4, '')::uuid,
  normalized_file_asset_id = NULLIF($5, '')::uuid,
  quality_status = $6,
  quality_issues = $7,
  updated_at = now()
WHERE tenant_id = $1 AND submission_id = $2 AND id::text = $3 AND deleted_at IS NULL
RETURNING id::text, tenant_id::text, submission_id::text, file_asset_id::text, page_no, status,
  COALESCE(latest_quality_run_id::text, ''), COALESCE(normalized_file_asset_id::text, ''),
  quality_status, quality_override, quality_issues, created_at
`, tenantID, input.SubmissionID, input.PageID, input.LatestQualityRunID, input.NormalizedFileAssetID, input.QualityStatus, issuesJSON)
	var page SubmissionPage
	if err := scanPage(row, &page); err != nil {
		return SubmissionPage{}, err
	}
	if err := s.aggregateQualityTx(ctx, tx, tenantID, input.SubmissionID); err != nil {
		return SubmissionPage{}, err
	}
	if err := tx.Commit(); err != nil {
		return SubmissionPage{}, err
	}
	return page, nil
}

func (s *PostgresStore) RunQualityCheck(ctx context.Context, tenantID string, submissionID string, _ string) (QualityResult, error) {
	item, err := s.getSubmission(ctx, tenantID, submissionID)
	if err != nil {
		return QualityResult{}, err
	}
	if item.Status == "ready_for_ocr" || item.Status == "rejected" {
		return QualityResult{}, ErrSubmissionLocked
	}
	pages, err := s.ListPages(ctx, tenantID, submissionID)
	if err != nil {
		return QualityResult{}, err
	}
	issues := qualityIssues(item, pages)
	status := "pages_uploaded"
	qualityStatus := "failed"
	if len(issues) == 0 {
		status = "quality_checked"
		qualityStatus = "passed"
	}
	issuesJSON, _ := json.Marshal(issues)
	if _, err := s.db.ExecContext(ctx, `
UPDATE submission
SET status = $3, quality_status = $4, quality_issues = $5, actual_page_count = $6, updated_at = now()
WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL
`, tenantID, submissionID, status, qualityStatus, issuesJSON, len(pages)); err != nil {
		return QualityResult{}, err
	}
	return QualityResult{Valid: len(issues) == 0, Issues: issues}, nil
}

func (s *PostgresStore) UpdateStatus(ctx context.Context, tenantID string, submissionID string, _ string, status string) (Submission, error) {
	item, err := s.getSubmission(ctx, tenantID, submissionID)
	if err != nil {
		return Submission{}, err
	}
	if !CanTransition(item.Status, status, item.QualityStatus) {
		return Submission{}, ErrInvalidTransition
	}
	row := s.db.QueryRowContext(ctx, `
UPDATE submission
SET status = $3, updated_at = now()
WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL
RETURNING id::text, tenant_id::text, exam_id::text, COALESCE(student_id::text, ''),
  COALESCE(candidate_no, ''), source_type, status, expected_page_count, actual_page_count,
  quality_status, quality_issues, collected_by::text, created_at
`, tenantID, submissionID, status)
	var out Submission
	if err := scanSubmission(row, &out); err != nil {
		return Submission{}, err
	}
	return out, nil
}

func (s *PostgresStore) getSubmission(ctx context.Context, tenantID string, id string) (Submission, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id::text, tenant_id::text, exam_id::text, COALESCE(student_id::text, ''),
  COALESCE(candidate_no, ''), source_type, status, expected_page_count, actual_page_count,
  quality_status, quality_issues, collected_by::text, created_at
FROM submission
WHERE tenant_id = $1 AND id::text = $2 AND deleted_at IS NULL
`, tenantID, id)
	var out Submission
	if err := scanSubmission(row, &out); err != nil {
		return Submission{}, err
	}
	return out, nil
}

func (s *PostgresStore) pageByNo(ctx context.Context, tenantID string, submissionID string, pageNo int) (SubmissionPage, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id::text, tenant_id::text, submission_id::text, file_asset_id::text, page_no, status,
  COALESCE(latest_quality_run_id::text, ''), COALESCE(normalized_file_asset_id::text, ''),
  quality_status, quality_override, quality_issues, created_at
FROM submission_page
WHERE tenant_id = $1 AND submission_id = $2 AND page_no = $3 AND deleted_at IS NULL
`, tenantID, submissionID, pageNo)
	var page SubmissionPage
	if err := scanPage(row, &page); err != nil {
		return SubmissionPage{}, err
	}
	return page, nil
}

func (s *PostgresStore) aggregateQualityTx(ctx context.Context, tx *sql.Tx, tenantID string, submissionID string) error {
	rows, err := tx.QueryContext(ctx, `
SELECT quality_status
FROM submission_page
WHERE tenant_id = $1 AND submission_id = $2 AND deleted_at IS NULL
`, tenantID, submissionID)
	if err != nil {
		return err
	}
	defer rows.Close()
	pageCount := 0
	hasUnchecked := false
	hasReview := false
	hasFailed := false
	for rows.Next() {
		pageCount++
		var status string
		if err := rows.Scan(&status); err != nil {
			return err
		}
		switch status {
		case "failed":
			hasFailed = true
		case "review":
			hasReview = true
		case "passed":
		default:
			hasUnchecked = true
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	var expected int
	if err := tx.QueryRowContext(ctx, `
SELECT expected_page_count
FROM submission
WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL
`, tenantID, submissionID).Scan(&expected); err != nil {
		return err
	}
	qualityStatus := "unchecked"
	submissionStatus := "pages_uploaded"
	if expected > 0 && pageCount != expected {
		qualityStatus = "failed"
	} else if hasFailed {
		qualityStatus = "failed"
	} else if hasReview {
		qualityStatus = "review"
	} else if hasUnchecked || pageCount == 0 {
		qualityStatus = "unchecked"
	} else {
		qualityStatus = "passed"
		submissionStatus = "quality_checked"
	}
	_, err = tx.ExecContext(ctx, `
UPDATE submission
SET actual_page_count = $3,
  quality_status = $4,
  status = $5,
  updated_at = now()
WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL
`, tenantID, submissionID, pageCount, qualityStatus, submissionStatus)
	return err
}

type submissionScanner interface {
	Scan(dest ...any) error
}

func scanSubmission(row submissionScanner, out *Submission) error {
	var issues []byte
	if err := row.Scan(
		&out.ID,
		&out.TenantID,
		&out.ExamID,
		&out.StudentID,
		&out.CandidateNo,
		&out.SourceType,
		&out.Status,
		&out.ExpectedPageCount,
		&out.ActualPageCount,
		&out.QualityStatus,
		&issues,
		&out.CollectedBy,
		&out.CreatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	_ = json.Unmarshal(issues, &out.QualityIssues)
	return nil
}

func scanPage(row submissionScanner, out *SubmissionPage) error {
	var issues []byte
	var override []byte
	if err := row.Scan(
		&out.ID,
		&out.TenantID,
		&out.SubmissionID,
		&out.FileAssetID,
		&out.PageNo,
		&out.Status,
		&out.LatestQualityRunID,
		&out.NormalizedFileAssetID,
		&out.QualityStatus,
		&override,
		&issues,
		&out.CreatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	_ = json.Unmarshal(override, &out.QualityOverride)
	if out.QualityOverride == nil {
		out.QualityOverride = map[string]any{}
	}
	_ = json.Unmarshal(issues, &out.QualityIssues)
	return nil
}
