package server

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"edugrade-enterprise/services/api-gateway/internal/auth"
	"edugrade-enterprise/services/api-gateway/internal/config"
	"edugrade-enterprise/services/api-gateway/internal/evidence"
	"edugrade-enterprise/services/api-gateway/internal/exam"
	"edugrade-enterprise/services/api-gateway/internal/files"
	"edugrade-enterprise/services/api-gateway/internal/grading"
	"edugrade-enterprise/services/api-gateway/internal/logger"
	ocrpkg "edugrade-enterprise/services/api-gateway/internal/ocr"
	"edugrade-enterprise/services/api-gateway/internal/org"
	"edugrade-enterprise/services/api-gateway/internal/paper"
	"edugrade-enterprise/services/api-gateway/internal/segment"
	"edugrade-enterprise/services/api-gateway/internal/submission"
)

const evidenceTenantID = "00000000-0000-0000-0000-000000000002"
const evidenceUserID = "00000000-0000-0000-0000-000000000701"

func TestVerifyEvidenceRouteRecordsJobAndAudit(t *testing.T) {
	authStore := evidenceAuthStoreWithPermissions(t, []string{"evidence:manage"})
	evidenceStore := evidence.NewMemoryStore()
	evidenceStore.AddContext(evidenceTenantID, "grade-1", validEvidenceRouteContext())
	router := evidenceRouter(authStore, evidenceStore)
	token := evidenceLogin(t, router)

	req := evidenceAuthedRequest(http.MethodPost, "/api/v1/ai-grades/grade-1/verify-evidence", token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("verify evidence expected 201, got %d %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, expected := range []string{`"job_type":"evidence_check"`, `"target_type":"ai_grade"`, `"target_id":"grade-1"`, `"status":"passed"`, `"passed":true`} {
		if !strings.Contains(body, expected) {
			t.Fatalf("verify evidence response missing %s: %s", expected, body)
		}
	}
	evidenceAssertAuditAction(t, authStore, "evidence.checked")
}

func TestVerifyEvidenceRouteRequiresEvidenceManage(t *testing.T) {
	evidenceStore := evidence.NewMemoryStore()
	evidenceStore.AddContext(evidenceTenantID, "grade-1", validEvidenceRouteContext())
	router := evidenceRouter(evidenceAuthStoreWithPermissions(t, []string{"system:read"}), evidenceStore)
	token := evidenceLogin(t, router)

	req := evidenceAuthedRequest(http.MethodPost, "/api/v1/ai-grades/grade-1/verify-evidence", token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/v1/ai-grades/grade-1/verify-evidence", nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d %s", rec.Code, rec.Body.String())
	}
}

func evidenceRouter(authStore *auth.MemoryStore, evidenceStore evidence.Store) http.Handler {
	cfg := config.Config{
		Service: config.ServiceConfig{Name: "test", Environment: "test", ReadinessTimeout: time.Millisecond},
		Auth:    config.AuthConfig{SessionTTL: time.Hour},
	}
	return NewRouterComplete(cfg, logger.New(io.Discard, "error"), nil, authStore, org.NewMemoryStore(), exam.NewMemoryStore(), paper.NewMemoryStore(), files.NewMemoryStore(), files.NewMemoryObjectStorage(), submission.NewMemoryStore(), ocrpkg.NewMemoryStore(), ocrpkg.NewMemoryQueue(), segment.NewMemoryStore(), evidenceStore)
}

func evidenceAuthStoreWithPermissions(t *testing.T, permissions []string) *auth.MemoryStore {
	t.Helper()
	hash, err := auth.HashPassword("ChangeMe123!")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	store := auth.NewMemoryStore()
	store.AddUser(auth.UserWithPassword{
		User: auth.User{
			ID:          evidenceUserID,
			TenantID:    evidenceTenantID,
			TenantCode:  "demo",
			Username:    "evidence_admin",
			DisplayName: "Evidence Admin",
			Status:      "active",
			Roles:       []string{"teacher"},
			Permissions: permissions,
			DataScope:   map[string]any{"scope": "school"},
		},
		PasswordHash: hash,
	})
	return store
}

func evidenceLogin(t *testing.T, router http.Handler) string {
	t.Helper()
	raw, _ := json.Marshal(map[string]string{"tenant_code": "demo", "username": "evidence_admin", "password": "ChangeMe123!"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(raw))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("login expected 200, got %d %s", rec.Code, rec.Body.String())
	}
	var response struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return response.AccessToken
}

func evidenceAuthedRequest(method string, path string, token string) *http.Request {
	req := httptest.NewRequest(method, path, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	return req
}

func validEvidenceRouteContext() evidence.Context {
	ocrConfidence := 0.95
	return evidence.Context{
		Grade: evidence.Grade{
			ID:              "grade-1",
			TenantID:        evidenceTenantID,
			AnswerSegmentID: "segment-1",
			QuestionID:      "question-1",
			QuestionNo:      "Q1",
			QuestionType:    "short_answer",
			SuggestedScore:  2,
			MaxScore:        2,
			MatchedPoints: []grading.PointResult{
				{Code: "p1", Label: "correct concept", Score: 2},
			},
			Evidence: []grading.Evidence{
				{Type: "text", AnswerSegment: "segment-1", AnswerText: "photosynthesis uses sunlight", BBox: []float64{10, 10, 50, 20}},
			},
			Status: "succeeded",
		},
		AnswerText:        "Photosynthesis uses sunlight.",
		OCRConfidence:     &ocrConfidence,
		AnswerSegmentBBox: []float64{0, 0, 100, 100},
		Rubric: paper.Rubric{
			ID:         "rubric-1",
			QuestionID: "question-1",
			Version:    "v1",
			Status:     "approved",
			MaxScore:   2,
			Points: []paper.RubricPoint{
				{ID: "p1", Description: "correct concept", Score: 2, Required: true},
			},
		},
	}
}

func evidenceAssertAuditAction(t *testing.T, store *auth.MemoryStore, action string) {
	t.Helper()
	for _, audit := range store.Audits() {
		if audit.Action == action {
			return
		}
	}
	t.Fatalf("missing audit action %s in %#v", action, store.Audits())
}
