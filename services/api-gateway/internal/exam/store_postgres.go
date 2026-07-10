package exam

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

type PostgresStore struct {
	db *sql.DB
}

func NewPostgresStore(db *sql.DB) *PostgresStore {
	return &PostgresStore{db: db}
}

func (s *PostgresStore) CreateExam(ctx context.Context, tenantID string, createdBy string, input CreateInput) (Exam, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Exam{}, err
	}
	defer tx.Rollback()

	appealEnabled := true
	if input.AppealEnabled != nil {
		appealEnabled = *input.AppealEnabled
	}
	row := tx.QueryRowContext(ctx, `
INSERT INTO exam (tenant_id, school_id, name, subject, exam_type, total_score, status, grading_mode, appeal_enabled, publish_policy, created_by)
SELECT $1, school.id, $3, $4, $5, $6, 'draft', $7, $8, $9, $10
FROM school
WHERE tenant_id = $1 AND id::text = $2 AND deleted_at IS NULL
RETURNING id::text, tenant_id::text, school_id::text, name, subject, exam_type, total_score::float8, status, grading_mode, appeal_enabled, publish_policy, created_by::text
`, tenantID, input.SchoolID, input.Name, input.Subject, input.ExamType, input.TotalScore, input.GradingMode, appealEnabled, input.PublishPolicy, createdBy)
	var out Exam
	if err := scanExam(row, &out); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Exam{}, ErrInvalidInput
		}
		return Exam{}, err
	}
	if err := s.replaceClasses(ctx, tx, tenantID, out.ID, input.ClassIDs); err != nil {
		return Exam{}, err
	}
	out.ClassIDs = cloneStrings(input.ClassIDs)
	if err := tx.Commit(); err != nil {
		return Exam{}, err
	}
	return out, nil
}

func (s *PostgresStore) ListExams(ctx context.Context, tenantID string, filter ListFilter) ([]Exam, error) {
	query := `
SELECT id::text, tenant_id::text, school_id::text, name, subject, exam_type, total_score::float8, status, grading_mode, appeal_enabled, publish_policy, created_by::text
FROM exam
WHERE tenant_id = $1 AND deleted_at IS NULL`
	args := []any{tenantID}
	if filter.Status != "" {
		query += ` AND status = $2`
		args = append(args, filter.Status)
	}
	if filter.SchoolID != "" {
		query += fmt.Sprintf(` AND school_id = $%d`, len(args)+1)
		args = append(args, filter.SchoolID)
	}
	query += ` ORDER BY created_at DESC`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Exam{}
	for rows.Next() {
		var item Exam
		if err := scanExam(rows, &item); err != nil {
			return nil, err
		}
		item.ClassIDs, _ = s.classIDs(ctx, tenantID, item.ID)
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *PostgresStore) GetExam(ctx context.Context, tenantID string, id string) (Exam, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id::text, tenant_id::text, school_id::text, name, subject, exam_type, total_score::float8, status, grading_mode, appeal_enabled, publish_policy, created_by::text
FROM exam
WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL
`, tenantID, id)
	var out Exam
	if err := scanExam(row, &out); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Exam{}, ErrNotFound
		}
		return Exam{}, err
	}
	out.ClassIDs, _ = s.classIDs(ctx, tenantID, id)
	return out, nil
}

func (s *PostgresStore) UpdateExam(ctx context.Context, tenantID string, id string, input UpdateInput) (Exam, error) {
	current, err := s.GetExam(ctx, tenantID, id)
	if err != nil {
		return Exam{}, err
	}
	if IsCoreLocked(current.Status) {
		return Exam{}, ErrLocked
	}
	merged := current
	if input.SchoolID != nil {
		merged.SchoolID = *input.SchoolID
	}
	if input.Name != nil {
		merged.Name = *input.Name
	}
	if input.Subject != nil {
		merged.Subject = *input.Subject
	}
	if input.ExamType != nil {
		merged.ExamType = *input.ExamType
	}
	if input.TotalScore != nil {
		merged.TotalScore = *input.TotalScore
	}
	if input.GradingMode != nil {
		merged.GradingMode = *input.GradingMode
	}
	if input.AppealEnabled != nil {
		merged.AppealEnabled = *input.AppealEnabled
	}
	if input.PublishPolicy != nil {
		merged.PublishPolicy = *input.PublishPolicy
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Exam{}, err
	}
	defer tx.Rollback()
	if err := s.requireSchoolInTenant(ctx, tx, tenantID, merged.SchoolID); err != nil {
		return Exam{}, err
	}
	row := tx.QueryRowContext(ctx, `
UPDATE exam
SET school_id = $3, name = $4, subject = $5, exam_type = $6, total_score = $7, grading_mode = $8, appeal_enabled = $9, publish_policy = $10, updated_at = now()
WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL
RETURNING id::text, tenant_id::text, school_id::text, name, subject, exam_type, total_score::float8, status, grading_mode, appeal_enabled, publish_policy, created_by::text
`, tenantID, id, merged.SchoolID, merged.Name, merged.Subject, merged.ExamType, merged.TotalScore, merged.GradingMode, merged.AppealEnabled, merged.PublishPolicy)
	var out Exam
	if err := scanExam(row, &out); err != nil {
		return Exam{}, err
	}
	if input.ClassIDs != nil {
		if err := s.replaceClasses(ctx, tx, tenantID, id, *input.ClassIDs); err != nil {
			return Exam{}, err
		}
		out.ClassIDs = cloneStrings(*input.ClassIDs)
	} else {
		out.ClassIDs = current.ClassIDs
	}
	if err := tx.Commit(); err != nil {
		return Exam{}, err
	}
	return out, nil
}

func (s *PostgresStore) UpdateStatus(ctx context.Context, tenantID string, id string, status string) (Exam, error) {
	current, err := s.GetExam(ctx, tenantID, id)
	if err != nil {
		return Exam{}, err
	}
	if !CanTransition(current.Status, status) {
		return Exam{}, ErrInvalidTransition
	}
	row := s.db.QueryRowContext(ctx, `
UPDATE exam
SET status = $3, updated_at = now()
WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL
RETURNING id::text, tenant_id::text, school_id::text, name, subject, exam_type, total_score::float8, status, grading_mode, appeal_enabled, publish_policy, created_by::text
`, tenantID, id, status)
	var out Exam
	if err := scanExam(row, &out); err != nil {
		return Exam{}, err
	}
	out.ClassIDs = current.ClassIDs
	return out, nil
}

func (s *PostgresStore) replaceClasses(ctx context.Context, tx *sql.Tx, tenantID string, examID string, classIDs []string) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM exam_class WHERE tenant_id = $1 AND exam_id = $2`, tenantID, examID); err != nil {
		return err
	}
	for _, classID := range classIDs {
		if err := s.requireClassInTenant(ctx, tx, tenantID, classID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO exam_class (tenant_id, exam_id, class_id) VALUES ($1, $2, $3) ON CONFLICT (tenant_id, exam_id, class_id) DO NOTHING`, tenantID, examID, classID); err != nil {
			return err
		}
	}
	return nil
}

func (s *PostgresStore) requireSchoolInTenant(ctx context.Context, tx *sql.Tx, tenantID string, schoolID string) error {
	var id string
	err := tx.QueryRowContext(ctx, `
SELECT id::text
FROM school
WHERE tenant_id = $1 AND id::text = $2 AND deleted_at IS NULL
`, tenantID, schoolID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrInvalidInput
	}
	return err
}

func (s *PostgresStore) requireClassInTenant(ctx context.Context, tx *sql.Tx, tenantID string, classID string) error {
	var id string
	err := tx.QueryRowContext(ctx, `
SELECT id::text
FROM school_class
WHERE tenant_id = $1 AND id::text = $2 AND deleted_at IS NULL
`, tenantID, classID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrInvalidInput
	}
	return err
}

func (s *PostgresStore) classIDs(ctx context.Context, tenantID string, examID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT class_id::text FROM exam_class WHERE tenant_id = $1 AND exam_id = $2 ORDER BY created_at`, tenantID, examID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

type scanner interface {
	Scan(dest ...any) error
}

func scanExam(row scanner, out *Exam) error {
	return row.Scan(&out.ID, &out.TenantID, &out.SchoolID, &out.Name, &out.Subject, &out.ExamType, &out.TotalScore, &out.Status, &out.GradingMode, &out.AppealEnabled, &out.PublishPolicy, &out.CreatedBy)
}
