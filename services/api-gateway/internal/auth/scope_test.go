package auth

import (
	"net/http"
	"net/http/httptest"
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

func TestPlatformWorkerTenantScope(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, _ := UserFromContext(r.Context())
		if user.TenantID != "00000000-0000-0000-0000-000000000002" || user.ID != "" {
			t.Fatalf("unexpected scoped worker: %#v", user)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(WorkerTenantHeader, "00000000-0000-0000-0000-000000000002")
	req = req.WithContext(WithUser(req.Context(), User{
		ID: PlatformTenantID, TenantID: PlatformTenantID, Roles: []string{"page_processing_worker"},
	}))
	rec := httptest.NewRecorder()

	PlatformWorkerTenantScope(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected scoped request, got %d %s", rec.Code, rec.Body.String())
	}
}

func TestPlatformWorkerTenantScopeRejectsProductUser(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(WorkerTenantHeader, "00000000-0000-0000-0000-000000000002")
	req = req.WithContext(WithUser(req.Context(), User{
		ID: PlatformTenantID, TenantID: PlatformTenantID, Roles: []string{"platform_admin"},
	}))
	rec := httptest.NewRecorder()

	PlatformWorkerTenantScope(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("forbidden scope reached handler")
	})).ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d %s", rec.Code, rec.Body.String())
	}
}
