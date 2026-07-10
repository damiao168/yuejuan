package segment

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

func (s *PostgresStore) CreateSegments(ctx context.Context, inputs []CreateSegmentInput) ([]Segment, error) {
	out := make([]Segment, 0, len(inputs))
	for _, input := range inputs {
		if err := ValidateBBox(input.BBox); err != nil {
			return nil, err
		}
		bbox, _ := json.Marshal(input.BBox)
		row := s.db.QueryRowContext(ctx, `
INSERT INTO answer_segment (
  tenant_id, submission_id, submission_page_id, question_id, question_no, bbox, source, status
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (tenant_id, submission_id, question_id)
DO UPDATE SET updated_at = answer_segment.updated_at
RETURNING id::text, tenant_id::text, submission_id::text, submission_page_id::text,
  question_id::text, question_no, bbox, source, status, COALESCE(review_notes, ''),
  COALESCE(reviewed_by::text, ''), reviewed_at, created_at
`, input.TenantID, input.SubmissionID, input.SubmissionPageID, input.QuestionID, input.QuestionNo, bbox, input.Source, input.Status)
		var item Segment
		if err := scanSegment(row, &item); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	sortSegments(out)
	return out, nil
}

func (s *PostgresStore) ListBySubmission(ctx context.Context, tenantID string, submissionID string) ([]Segment, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id::text, tenant_id::text, submission_id::text, submission_page_id::text,
  question_id::text, question_no, bbox, source, status, COALESCE(review_notes, ''),
  COALESCE(reviewed_by::text, ''), reviewed_at, created_at
FROM answer_segment
WHERE tenant_id = $1 AND submission_id = $2 AND deleted_at IS NULL
ORDER BY question_no
`, tenantID, submissionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Segment{}
	for rows.Next() {
		var item Segment
		if err := scanSegment(rows, &item); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *PostgresStore) Update(ctx context.Context, tenantID string, id string, actorID string, input UpdateSegmentInput) (Segment, error) {
	current, err := s.get(ctx, tenantID, id)
	if err != nil {
		return Segment{}, err
	}
	merged := current
	if input.BBox != nil {
		if err := ValidateBBox(*input.BBox); err != nil {
			return Segment{}, err
		}
		merged.BBox = append([]float64{}, (*input.BBox)...)
		merged.Source = "manual"
	}
	if input.Status != nil {
		if !IsValidStatus(*input.Status) {
			return Segment{}, ErrInvalidInput
		}
		merged.Status = *input.Status
	}
	if input.ReviewNotes != nil {
		merged.ReviewNotes = *input.ReviewNotes
	}
	bbox, _ := json.Marshal(merged.BBox)
	row := s.db.QueryRowContext(ctx, `
UPDATE answer_segment
SET bbox = $3,
  source = $4,
  status = $5,
  review_notes = $6,
  reviewed_by = NULLIF($7, '')::uuid,
  reviewed_at = now(),
  updated_at = now()
WHERE tenant_id = $1 AND id::text = $2 AND deleted_at IS NULL
RETURNING id::text, tenant_id::text, submission_id::text, submission_page_id::text,
  question_id::text, question_no, bbox, source, status, COALESCE(review_notes, ''),
  COALESCE(reviewed_by::text, ''), reviewed_at, created_at
`, tenantID, id, bbox, merged.Source, merged.Status, merged.ReviewNotes, actorID)
	var out Segment
	if err := scanSegment(row, &out); err != nil {
		return Segment{}, err
	}
	return out, nil
}

func (s *PostgresStore) get(ctx context.Context, tenantID string, id string) (Segment, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id::text, tenant_id::text, submission_id::text, submission_page_id::text,
  question_id::text, question_no, bbox, source, status, COALESCE(review_notes, ''),
  COALESCE(reviewed_by::text, ''), reviewed_at, created_at
FROM answer_segment
WHERE tenant_id = $1 AND id::text = $2 AND deleted_at IS NULL
`, tenantID, id)
	var item Segment
	if err := scanSegment(row, &item); err != nil {
		return Segment{}, err
	}
	return item, nil
}

type segmentScanner interface {
	Scan(dest ...any) error
}

func scanSegment(row segmentScanner, out *Segment) error {
	var bbox []byte
	var reviewedAt sql.NullTime
	if err := row.Scan(
		&out.ID,
		&out.TenantID,
		&out.SubmissionID,
		&out.SubmissionPageID,
		&out.QuestionID,
		&out.QuestionNo,
		&bbox,
		&out.Source,
		&out.Status,
		&out.ReviewNotes,
		&out.ReviewedBy,
		&reviewedAt,
		&out.CreatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	_ = json.Unmarshal(bbox, &out.BBox)
	if reviewedAt.Valid {
		value := reviewedAt.Time.UTC()
		out.ReviewedAt = &value
	}
	return nil
}
