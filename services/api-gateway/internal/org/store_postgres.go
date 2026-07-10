package org

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

func (s *PostgresStore) CreateTenant(ctx context.Context, input Tenant) (Tenant, error) {
	row := s.db.QueryRowContext(ctx, `
WITH new_id AS (SELECT gen_random_uuid() AS id)
INSERT INTO tenant (id, tenant_id, name, code, status)
SELECT id, id, $1, $2, COALESCE(NULLIF($3, ''), 'active') FROM new_id
RETURNING id::text, name, code, status
`, input.Name, input.Code, input.Status)
	var out Tenant
	if err := row.Scan(&out.ID, &out.Name, &out.Code, &out.Status); err != nil {
		return Tenant{}, err
	}
	return out, nil
}

func (s *PostgresStore) ListTenants(ctx context.Context, tenantID string, canListAll bool) ([]Tenant, error) {
	query := `SELECT id::text, name, code, status FROM tenant WHERE deleted_at IS NULL`
	args := []any{}
	if !canListAll {
		query += ` AND id = $1`
		args = append(args, tenantID)
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Tenant{}
	for rows.Next() {
		var item Tenant
		if err := rows.Scan(&item.ID, &item.Name, &item.Code, &item.Status); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *PostgresStore) UpdateTenantStatus(ctx context.Context, id string, status string) (Tenant, error) {
	row := s.db.QueryRowContext(ctx, `UPDATE tenant SET status = $2, updated_at = now() WHERE id = $1 AND deleted_at IS NULL RETURNING id::text, name, code, status`, id, status)
	var out Tenant
	if err := row.Scan(&out.ID, &out.Name, &out.Code, &out.Status); err != nil {
		return Tenant{}, err
	}
	return out, nil
}

func (s *PostgresStore) CreateSchool(ctx context.Context, tenantID string, input School) (School, error) {
	row := s.db.QueryRowContext(ctx, `INSERT INTO school (tenant_id, name, code, status) VALUES ($1, $2, $3, COALESCE(NULLIF($4, ''), 'active')) RETURNING id::text, tenant_id::text, name, code, status`, tenantID, input.Name, input.Code, input.Status)
	var out School
	if err := row.Scan(&out.ID, &out.TenantID, &out.Name, &out.Code, &out.Status); err != nil {
		return School{}, err
	}
	return out, nil
}

func (s *PostgresStore) ListSchools(ctx context.Context, tenantID string) ([]School, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id::text, tenant_id::text, name, code, status FROM school WHERE tenant_id = $1 AND deleted_at IS NULL ORDER BY created_at DESC`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []School{}
	for rows.Next() {
		var item School
		if err := rows.Scan(&item.ID, &item.TenantID, &item.Name, &item.Code, &item.Status); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *PostgresStore) CreateGrade(ctx context.Context, tenantID string, input Grade) (Grade, error) {
	row := s.db.QueryRowContext(ctx, `
INSERT INTO grade (tenant_id, school_id, name, level_no, academic_year, status)
SELECT $1, school.id, $3, $4, $5, COALESCE(NULLIF($6, ''), 'active')
FROM school
WHERE tenant_id = $1 AND id::text = $2 AND deleted_at IS NULL
RETURNING id::text, tenant_id::text, school_id::text, name, COALESCE(level_no, 0), COALESCE(academic_year, ''), status
`, tenantID, input.SchoolID, input.Name, input.LevelNo, input.AcademicYear, input.Status)
	var out Grade
	if err := row.Scan(&out.ID, &out.TenantID, &out.SchoolID, &out.Name, &out.LevelNo, &out.AcademicYear, &out.Status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Grade{}, ErrInvalidParent
		}
		return Grade{}, err
	}
	return out, nil
}

func (s *PostgresStore) ListGrades(ctx context.Context, tenantID string, schoolID string) ([]Grade, error) {
	query := `SELECT id::text, tenant_id::text, school_id::text, name, COALESCE(level_no, 0), COALESCE(academic_year, ''), status FROM grade WHERE tenant_id = $1 AND deleted_at IS NULL`
	args := []any{tenantID}
	if schoolID != "" {
		query += ` AND school_id = $2`
		args = append(args, schoolID)
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Grade{}
	for rows.Next() {
		var item Grade
		if err := rows.Scan(&item.ID, &item.TenantID, &item.SchoolID, &item.Name, &item.LevelNo, &item.AcademicYear, &item.Status); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *PostgresStore) CreateClass(ctx context.Context, tenantID string, input Class) (Class, error) {
	row := s.db.QueryRowContext(ctx, `
WITH parent_school AS (
  SELECT id FROM school WHERE tenant_id = $1 AND id::text = $2 AND deleted_at IS NULL
),
parent_grade AS (
  SELECT id FROM grade WHERE tenant_id = $1 AND id::text = $3 AND school_id::text = $2 AND deleted_at IS NULL
)
INSERT INTO school_class (tenant_id, school_id, grade_id, name, code, status)
SELECT $1, parent_school.id, parent_grade.id, $4, $5, COALESCE(NULLIF($6, ''), 'active')
FROM parent_school
CROSS JOIN parent_grade
RETURNING id::text, tenant_id::text, school_id::text, grade_id::text, name, code, status
`, tenantID, input.SchoolID, input.GradeID, input.Name, input.Code, input.Status)
	var out Class
	if err := row.Scan(&out.ID, &out.TenantID, &out.SchoolID, &out.GradeID, &out.Name, &out.Code, &out.Status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Class{}, ErrInvalidParent
		}
		return Class{}, err
	}
	return out, nil
}

func (s *PostgresStore) ListClasses(ctx context.Context, tenantID string, gradeID string) ([]Class, error) {
	query := `SELECT id::text, tenant_id::text, school_id::text, grade_id::text, name, code, status FROM school_class WHERE tenant_id = $1 AND deleted_at IS NULL`
	args := []any{tenantID}
	if gradeID != "" {
		query += ` AND grade_id = $2`
		args = append(args, gradeID)
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Class{}
	for rows.Next() {
		var item Class
		if err := rows.Scan(&item.ID, &item.TenantID, &item.SchoolID, &item.GradeID, &item.Name, &item.Code, &item.Status); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *PostgresStore) CreateStudent(ctx context.Context, tenantID string, input Student) (Student, error) {
	row := s.db.QueryRowContext(ctx, `
WITH parent_school AS (
  SELECT id FROM school WHERE tenant_id = $1 AND id::text = $2 AND deleted_at IS NULL
),
parent_class AS (
  SELECT id FROM school_class WHERE tenant_id = $1 AND id::text = $3 AND school_id::text = $2 AND deleted_at IS NULL
)
INSERT INTO student (tenant_id, school_id, class_id, student_no, name, gender, status)
SELECT $1, parent_school.id, parent_class.id, $4, $5, $6, COALESCE(NULLIF($7, ''), 'active')
FROM parent_school
CROSS JOIN parent_class
RETURNING id::text, tenant_id::text, school_id::text, class_id::text, student_no, name, COALESCE(gender, ''), status
`, tenantID, input.SchoolID, input.ClassID, input.StudentNo, input.Name, input.Gender, input.Status)
	var out Student
	if err := row.Scan(&out.ID, &out.TenantID, &out.SchoolID, &out.ClassID, &out.StudentNo, &out.Name, &out.Gender, &out.Status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Student{}, ErrInvalidParent
		}
		return Student{}, err
	}
	return out, nil
}

func (s *PostgresStore) ListStudents(ctx context.Context, tenantID string, classID string) ([]Student, error) {
	query := `SELECT id::text, tenant_id::text, school_id::text, class_id::text, student_no, name, COALESCE(gender, ''), status FROM student WHERE tenant_id = $1 AND deleted_at IS NULL`
	args := []any{tenantID}
	if classID != "" {
		query += ` AND class_id = $2`
		args = append(args, classID)
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Student{}
	for rows.Next() {
		var item Student
		if err := rows.Scan(&item.ID, &item.TenantID, &item.SchoolID, &item.ClassID, &item.StudentNo, &item.Name, &item.Gender, &item.Status); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *PostgresStore) UpdateStudentStatus(ctx context.Context, tenantID string, id string, status string) (Student, error) {
	row := s.db.QueryRowContext(ctx, `UPDATE student SET status = $3, updated_at = now() WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL RETURNING id::text, tenant_id::text, school_id::text, class_id::text, student_no, name, COALESCE(gender, ''), status`, tenantID, id, status)
	var out Student
	if err := row.Scan(&out.ID, &out.TenantID, &out.SchoolID, &out.ClassID, &out.StudentNo, &out.Name, &out.Gender, &out.Status); err != nil {
		return Student{}, err
	}
	return out, nil
}

func (s *PostgresStore) BindTeacherClass(ctx context.Context, tenantID string, teacherID string, classID string) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO teacher_class (tenant_id, teacher_id, class_id) VALUES ($1, $2, $3) ON CONFLICT (tenant_id, teacher_id, class_id) DO NOTHING`, tenantID, teacherID, classID)
	return err
}
