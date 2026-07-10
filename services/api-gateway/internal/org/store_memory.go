package org

import (
	"context"
	"fmt"
	"sync"
)

type MemoryStore struct {
	mu       sync.RWMutex
	next     int
	tenants  map[string]Tenant
	schools  map[string]School
	grades   map[string]Grade
	classes  map[string]Class
	students map[string]Student
	bindings map[string]bool
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		next:     1,
		tenants:  map[string]Tenant{},
		schools:  map[string]School{},
		grades:   map[string]Grade{},
		classes:  map[string]Class{},
		students: map[string]Student{},
		bindings: map[string]bool{},
	}
}

func (s *MemoryStore) CreateTenant(_ context.Context, input Tenant) (Tenant, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	input.ID = s.id("tenant")
	if input.Status == "" {
		input.Status = "active"
	}
	s.tenants[input.ID] = input
	return input, nil
}

func (s *MemoryStore) ListTenants(_ context.Context, tenantID string, canListAll bool) ([]Tenant, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []Tenant{}
	for _, item := range s.tenants {
		if canListAll || item.ID == tenantID {
			out = append(out, item)
		}
	}
	return out, nil
}

func (s *MemoryStore) UpdateTenantStatus(_ context.Context, id string, status string) (Tenant, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.tenants[id]
	if !ok {
		return Tenant{}, fmt.Errorf("tenant not found")
	}
	item.Status = status
	s.tenants[id] = item
	return item, nil
}

func (s *MemoryStore) CreateSchool(_ context.Context, tenantID string, input School) (School, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	input.ID = s.id("school")
	input.TenantID = tenantID
	if input.Status == "" {
		input.Status = "active"
	}
	s.schools[input.ID] = input
	return input, nil
}

func (s *MemoryStore) ListSchools(_ context.Context, tenantID string) ([]School, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []School{}
	for _, item := range s.schools {
		if item.TenantID == tenantID {
			out = append(out, item)
		}
	}
	return out, nil
}

func (s *MemoryStore) CreateGrade(_ context.Context, tenantID string, input Grade) (Grade, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	input.ID = s.id("grade")
	input.TenantID = tenantID
	if input.Status == "" {
		input.Status = "active"
	}
	s.grades[input.ID] = input
	return input, nil
}

func (s *MemoryStore) ListGrades(_ context.Context, tenantID string, schoolID string) ([]Grade, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []Grade{}
	for _, item := range s.grades {
		if item.TenantID == tenantID && (schoolID == "" || item.SchoolID == schoolID) {
			out = append(out, item)
		}
	}
	return out, nil
}

func (s *MemoryStore) CreateClass(_ context.Context, tenantID string, input Class) (Class, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	input.ID = s.id("class")
	input.TenantID = tenantID
	if input.Status == "" {
		input.Status = "active"
	}
	s.classes[input.ID] = input
	return input, nil
}

func (s *MemoryStore) ListClasses(_ context.Context, tenantID string, gradeID string) ([]Class, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []Class{}
	for _, item := range s.classes {
		if item.TenantID == tenantID && (gradeID == "" || item.GradeID == gradeID) {
			out = append(out, item)
		}
	}
	return out, nil
}

func (s *MemoryStore) CreateStudent(_ context.Context, tenantID string, input Student) (Student, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if input.ID == "" {
		input.ID = s.id("student")
	}
	input.TenantID = tenantID
	if input.Status == "" {
		input.Status = "active"
	}
	s.students[input.ID] = input
	return input, nil
}

func (s *MemoryStore) ListStudents(_ context.Context, tenantID string, classID string) ([]Student, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []Student{}
	for _, item := range s.students {
		if item.TenantID == tenantID && (classID == "" || item.ClassID == classID) {
			out = append(out, item)
		}
	}
	return out, nil
}

func (s *MemoryStore) UpdateStudentStatus(_ context.Context, tenantID string, id string, status string) (Student, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.students[id]
	if !ok || item.TenantID != tenantID {
		return Student{}, fmt.Errorf("student not found")
	}
	item.Status = status
	s.students[id] = item
	return item, nil
}

func (s *MemoryStore) BindTeacherClass(_ context.Context, tenantID string, teacherID string, classID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.bindings[tenantID+"|"+teacherID+"|"+classID] = true
	return nil
}

func (s *MemoryStore) id(prefix string) string {
	id := fmt.Sprintf("%s-%d", prefix, s.next)
	s.next++
	return id
}
