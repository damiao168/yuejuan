package studentportal

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"edugrade-enterprise/services/api-gateway/internal/auth"
)

func TestListPublishedExamsUsesAuthenticatedStudentScopeOnly(t *testing.T) {
	store := NewMemoryStore()
	store.SeedPublishedExams("tenant-1", "student-1", []PublishedExam{{ExamID: "exam-1", Name: "Mathematics", Subject: "math", ReleaseVersion: 2, PublishedAt: time.Date(2026, 8, 10, 8, 0, 0, 0, time.UTC)}})
	store.SeedPublishedExams("tenant-1", "student-2", []PublishedExam{{ExamID: "exam-hidden", Name: "Hidden", ReleaseVersion: 1, PublishedAt: time.Now().UTC()}})
	h := NewHandler(NewService(store))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/student/exams?student_id=student-2", nil)
	req = req.WithContext(auth.WithUser(context.Background(), auth.User{TenantID: "tenant-1", ID: "user-1", Permissions: []string{"student:grade:read"}, DataScope: map[string]any{"scope": "self", "student_id": "student-1"}}))
	res := httptest.NewRecorder()
	h.ListPublishedExams(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	body := res.Body.String()
	if !strings.Contains(body, "exam-1") || strings.Contains(body, "exam-hidden") {
		t.Fatalf("list must use the authenticated student identity only: %s", body)
	}
	for _, forbidden := range []string{"student_id", "release_id", "quality", "prompt", "reviewer", "private_note"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("student discovery response leaked %q: %s", forbidden, body)
		}
	}
}

func TestListPublishedExamsRequiresStudentPermissionAndScope(t *testing.T) {
	h := NewHandler(NewService(NewMemoryStore()))
	cases := []auth.User{
		{TenantID: "tenant-1", ID: "admin-1", Permissions: []string{"score:manage"}, DataScope: map[string]any{"scope": "tenant"}},
		{TenantID: "tenant-1", ID: "student-1", Permissions: []string{"student:grade:read"}, DataScope: map[string]any{"scope": "self"}},
	}
	for _, user := range cases {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/student/exams", nil).WithContext(auth.WithUser(context.Background(), user))
		res := httptest.NewRecorder()
		h.ListPublishedExams(res, req)
		if res.Code != http.StatusForbidden {
			t.Fatalf("expected 403 for %#v, got %d: %s", user, res.Code, res.Body.String())
		}
	}
}
