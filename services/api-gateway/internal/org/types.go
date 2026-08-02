package org

import (
	"context"
	"errors"
)

var ErrInvalidParent = errors.New("organization parent does not belong to tenant")

type Tenant struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Code   string `json:"code"`
	Status string `json:"status"`
}

type TenantProvision struct {
	Name             string
	Code             string
	Status           string
	AdminUsername    string
	AdminDisplayName string
	PasswordHash     string
}

type School struct {
	ID       string `json:"id"`
	TenantID string `json:"tenant_id"`
	Name     string `json:"name"`
	Code     string `json:"code"`
	Status   string `json:"status"`
}

type Grade struct {
	ID           string `json:"id"`
	TenantID     string `json:"tenant_id"`
	SchoolID     string `json:"school_id"`
	Name         string `json:"name"`
	LevelNo      int    `json:"level_no"`
	AcademicYear string `json:"academic_year"`
	Status       string `json:"status"`
}

type Class struct {
	ID       string `json:"id"`
	TenantID string `json:"tenant_id"`
	SchoolID string `json:"school_id"`
	GradeID  string `json:"grade_id"`
	Name     string `json:"name"`
	Code     string `json:"code"`
	Status   string `json:"status"`
}

type Student struct {
	ID        string `json:"id"`
	TenantID  string `json:"tenant_id"`
	SchoolID  string `json:"school_id"`
	ClassID   string `json:"class_id"`
	StudentNo string `json:"student_no"`
	Name      string `json:"name"`
	Gender    string `json:"gender,omitempty"`
	Status    string `json:"status"`
}

type CSVImportError struct {
	Row     int    `json:"row"`
	Message string `json:"message"`
}

type CSVImportResult struct {
	Created int              `json:"created"`
	Errors  []CSVImportError `json:"errors"`
}

type Store interface {
	CreateTenant(ctx context.Context, input TenantProvision) (Tenant, error)
	ListTenants(ctx context.Context, tenantID string, canListAll bool) ([]Tenant, error)
	UpdateTenantStatus(ctx context.Context, id string, status string) (Tenant, error)
	CreateSchool(ctx context.Context, tenantID string, input School) (School, error)
	ListSchools(ctx context.Context, tenantID string) ([]School, error)
	CreateGrade(ctx context.Context, tenantID string, input Grade) (Grade, error)
	ListGrades(ctx context.Context, tenantID string, schoolID string) ([]Grade, error)
	CreateClass(ctx context.Context, tenantID string, input Class) (Class, error)
	ListClasses(ctx context.Context, tenantID string, gradeID string) ([]Class, error)
	CreateStudent(ctx context.Context, tenantID string, input Student) (Student, error)
	ListStudents(ctx context.Context, tenantID string, classID string) ([]Student, error)
	UpdateStudentStatus(ctx context.Context, tenantID string, id string, status string) (Student, error)
	BindTeacherClass(ctx context.Context, tenantID string, teacherID string, classID string) error
}
