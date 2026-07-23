package exam_test

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
	"edugrade-enterprise/services/api-gateway/internal/exam"
	"edugrade-enterprise/services/api-gateway/internal/logger"
	"edugrade-enterprise/services/api-gateway/internal/org"
	"edugrade-enterprise/services/api-gateway/internal/paper"
	"edugrade-enterprise/services/api-gateway/internal/server"
)

func TestCreateListDetailExam(t *testing.T) {
	authStore := authStoreWithPermissions(t, []string{"exam:manage"})
	examStore := exam.NewMemoryStore()
	router := testRouter(authStore, examStore)
	token := login(t, router)

	created := createExam(t, router, token)
	if created.Status != "draft" {
		t.Fatalf("expected draft status, got %s", created.Status)
	}
	if len(created.ClassIDs) != 2 {
		t.Fatalf("expected class ids to be persisted, got %#v", created.ClassIDs)
	}

	listReq := authedRequest(http.MethodGet, "/api/v1/exams", nil, token)
	listRec := httptest.NewRecorder()
	router.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK || !strings.Contains(listRec.Body.String(), created.ID) {
		t.Fatalf("expected created exam in list, got %d %s", listRec.Code, listRec.Body.String())
	}

	detailReq := authedRequest(http.MethodGet, "/api/v1/exams/"+created.ID, nil, token)
	detailRec := httptest.NewRecorder()
	router.ServeHTTP(detailRec, detailReq)
	if detailRec.Code != http.StatusOK || !strings.Contains(detailRec.Body.String(), `"name":"高二物理期末考试"`) {
		t.Fatalf("expected exam detail, got %d %s", detailRec.Code, detailRec.Body.String())
	}
}

func TestInvalidStatusTransitionRejected(t *testing.T) {
	authStore := authStoreWithPermissions(t, []string{"exam:manage"})
	router := testRouter(authStore, exam.NewMemoryStore())
	token := login(t, router)
	created := createExam(t, router, token)

	req := authedRequest(http.MethodPost, "/api/v1/exams/"+created.ID+"/status", bytes.NewBufferString(`{"status":"published"}`), token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409 for invalid transition, got %d %s", rec.Code, rec.Body.String())
	}
}

func TestFinalizedExamCannotPublishThroughStatusRoute(t *testing.T) {
	authStore := authStoreWithPermissions(t, []string{"exam:manage"})
	store := exam.NewMemoryStore()
	router := testRouter(authStore, store)
	token := login(t, router)
	created := createExam(t, router, token)
	if err := store.SetStatusForTest("tenant-exam", created.ID, "finalized"); err != nil {
		t.Fatalf("seed finalized exam: %v", err)
	}

	req := authedRequest(http.MethodPost, "/api/v1/exams/"+created.ID+"/status", bytes.NewBufferString(`{"status":"published"}`), token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "invalid_status_transition") {
		t.Fatalf("generic status route must not bypass score publication workflow, got %d %s", rec.Code, rec.Body.String())
	}
	unchanged, err := store.GetExam(t.Context(), "tenant-exam", created.ID)
	if err != nil || unchanged.Status != "finalized" {
		t.Fatalf("rejected publication must leave exam finalized, exam=%#v err=%v", unchanged, err)
	}
}

func TestPublishedExamCannotBeModified(t *testing.T) {
	authStore := authStoreWithPermissions(t, []string{"exam:manage"})
	store := exam.NewMemoryStore()
	router := testRouter(authStore, store)
	token := login(t, router)
	created := createExam(t, router, token)

	if err := store.SetStatusForTest("tenant-exam", created.ID, "published"); err != nil {
		t.Fatalf("seed published exam: %v", err)
	}

	req := authedRequest(http.MethodPatch, "/api/v1/exams/"+created.ID, bytes.NewBufferString(`{"name":"不应修改"}`), token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected published exam update to be rejected, got %d %s", rec.Code, rec.Body.String())
	}
}

func TestCollectionCannotBypassReadinessGate(t *testing.T) {
	authStore := authStoreWithPermissions(t, []string{"exam:manage"})
	router := testRouter(authStore, exam.NewMemoryStore())
	token := login(t, router)
	created := createExam(t, router, token)

	for _, status := range []string{"configured", "collecting"} {
		req := authedRequest(http.MethodPost, "/api/v1/exams/"+created.ID+"/status", bytes.NewBufferString(`{"status":"`+status+`"}`), token)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if status == "configured" && rec.Code != http.StatusOK {
			t.Fatalf("draft to configured expected 200, got %d %s", rec.Code, rec.Body.String())
		}
		if status == "collecting" && rec.Code != http.StatusConflict {
			t.Fatalf("configured to collecting must require readiness gate, got %d %s", rec.Code, rec.Body.String())
		}
	}
}

func TestArchiveExam(t *testing.T) {
	authStore := authStoreWithPermissions(t, []string{"exam:manage"})
	router := testRouter(authStore, exam.NewMemoryStore())
	token := login(t, router)
	created := createExam(t, router, token)

	req := authedRequest(http.MethodPost, "/api/v1/exams/"+created.ID+"/archive", nil, token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"status":"archived"`) {
		t.Fatalf("expected archive success, got %d %s", rec.Code, rec.Body.String())
	}
}

func TestExamPermissionDenied(t *testing.T) {
	authStore := authStoreWithPermissions(t, []string{"system:read"})
	router := testRouter(authStore, exam.NewMemoryStore())
	token := login(t, router)

	req := authedRequest(http.MethodPost, "/api/v1/exams", bytes.NewBufferString(validCreateExamJSON()), token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d %s", rec.Code, rec.Body.String())
	}
}

func TestExamTenantIsolation(t *testing.T) {
	store := exam.NewMemoryStore()
	if _, err := store.CreateExam(nil, "tenant-a", "user-a", exam.CreateInput{SchoolID: "school-a", Name: "A", Subject: "physics", ExamType: "formal_exam", TotalScore: 100, GradingMode: "ai_assisted", PublishPolicy: "after_admin_approval"}); err != nil {
		t.Fatalf("create a: %v", err)
	}
	if _, err := store.CreateExam(nil, "tenant-b", "user-b", exam.CreateInput{SchoolID: "school-b", Name: "B", Subject: "math", ExamType: "formal_exam", TotalScore: 100, GradingMode: "ai_assisted", PublishPolicy: "after_admin_approval"}); err != nil {
		t.Fatalf("create b: %v", err)
	}
	items, err := store.ListExams(nil, "tenant-a", exam.ListFilter{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(items) != 1 || items[0].Name != "A" {
		t.Fatalf("tenant isolation failed: %#v", items)
	}
}

func testRouter(authStore *auth.MemoryStore, examStore *exam.MemoryStore) http.Handler {
	cfg := config.Config{
		Service: config.ServiceConfig{Name: "test", Environment: "test", ReadinessTimeout: time.Millisecond},
		Auth:    config.AuthConfig{SessionTTL: time.Hour},
	}
	return server.NewRouter(cfg, logger.New(io.Discard, "error"), nil, authStore, org.NewMemoryStore(), examStore, paper.NewMemoryStore())
}

func authStoreWithPermissions(t *testing.T, permissions []string) *auth.MemoryStore {
	t.Helper()
	hash, err := auth.HashPassword("ChangeMe123!")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	store := auth.NewMemoryStore()
	store.AddUser(auth.UserWithPassword{
		User: auth.User{
			ID:          "u-exam",
			TenantID:    "tenant-exam",
			TenantCode:  "demo",
			Username:    "school_admin",
			DisplayName: "School Admin",
			Status:      "active",
			Roles:       []string{"school_admin"},
			Permissions: permissions,
			DataScope:   map[string]any{"scope": "school"},
		},
		PasswordHash: hash,
	})
	return store
}

func login(t *testing.T, router http.Handler) string {
	t.Helper()
	raw, _ := json.Marshal(map[string]string{"tenant_code": "demo", "username": "school_admin", "password": "ChangeMe123!"})
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

func createExam(t *testing.T, router http.Handler, token string) exam.Exam {
	t.Helper()
	req := authedRequest(http.MethodPost, "/api/v1/exams", bytes.NewBufferString(validCreateExamJSON()), token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create exam expected 201, got %d %s", rec.Code, rec.Body.String())
	}
	var response struct {
		Exam exam.Exam `json:"exam"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return response.Exam
}

func authedRequest(method string, path string, body *bytes.Buffer, token string) *http.Request {
	var reader io.Reader
	if body != nil {
		reader = body
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Authorization", "Bearer "+token)
	return req
}

func validCreateExamJSON() string {
	return `{"school_id":"school-1","name":"高二物理期末考试","subject":"physics","exam_type":"formal_exam","total_score":100,"grading_mode":"ai_assisted","appeal_enabled":true,"publish_policy":"after_admin_approval","class_ids":["class-1","class-2"]}`
}
