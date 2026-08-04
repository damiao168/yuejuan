package auth

import (
	"errors"
	"testing"
)

func TestScopedStudentIDSupportsFlatAndRoleKeyedScopes(t *testing.T) {
	flat := User{DataScope: map[string]any{"scope": "self", "student_id": "student-flat"}}
	if got, ok := ScopedStudentID(flat); !ok || got != "student-flat" {
		t.Fatalf("flat student scope expected student-flat, got %q ok=%t", got, ok)
	}

	roleKeyed := User{DataScope: map[string]any{
		"student": map[string]any{"scope": "self", "student_id": "student-role"},
	}}
	if got, ok := ScopedStudentID(roleKeyed); !ok || got != "student-role" {
		t.Fatalf("role-keyed student scope expected student-role, got %q ok=%t", got, ok)
	}
}

func TestResolveDeclaredAccessScopeCombinesRoleKeyedScopes(t *testing.T) {
	user := User{
		ID: "teacher-1", TenantID: "tenant-1", Roles: []string{"teacher", "grader"},
		DataScope: map[string]any{
			"teacher": map[string]any{"scope": "school", "school_id": "school-1", "class_ids": []any{"class-2", "class-1"}},
			"grader":  map[string]any{"scope": "exam_task", "review_task_id": "task-1"},
		},
	}
	scope, err := ResolveDeclaredAccessScope(user)
	if err != nil {
		t.Fatal(err)
	}
	if scope.TenantWide || !scope.AssignedOnly || !scope.AllowsSchool("school-1") ||
		!scope.AllowsClass("class-1") || !scope.AllowsReviewTask("task-1") {
		t.Fatalf("unexpected combined access scope: %#v", scope)
	}
}

func TestResolveDeclaredAccessScopeDefaultsToDeny(t *testing.T) {
	tests := []User{
		{ID: "u-1", TenantID: "t-1", Roles: []string{"teacher"}, DataScope: map[string]any{}},
		{ID: "u-1", TenantID: "t-1", Roles: []string{"teacher"}, DataScope: map[string]any{"teacher": map[string]any{"scope": "school", "unexpected": true}}},
		{ID: "u-1", TenantID: "t-1", Roles: []string{"teacher"}, DataScope: map[string]any{"other_role": map[string]any{"scope": "tenant"}}},
		{ID: "u-1", TenantID: "t-1", Roles: []string{"student"}, DataScope: map[string]any{"scope": "self"}},
	}
	for _, user := range tests {
		if _, err := ResolveDeclaredAccessScope(user); !errors.Is(err, ErrAccessScopeMissing) && !errors.Is(err, ErrAccessScopeInvalid) {
			t.Fatalf("expected missing/invalid scope for %#v, got %v", user.DataScope, err)
		}
	}
}

func TestResolveDeclaredAccessScopeRejectsForgedPlatformScope(t *testing.T) {
	_, err := ResolveDeclaredAccessScope(User{
		ID: "tenant-admin", TenantID: "tenant-1", Roles: []string{"tenant_admin"},
		DataScope: map[string]any{"scope": "platform"},
	})
	if !errors.Is(err, ErrAccessScopeInvalid) {
		t.Fatalf("expected invalid platform scope, got %v", err)
	}
}
