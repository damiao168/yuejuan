package auth

import "testing"

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
