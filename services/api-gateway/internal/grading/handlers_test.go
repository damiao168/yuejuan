package grading_test

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
	"edugrade-enterprise/services/api-gateway/internal/files"
	"edugrade-enterprise/services/api-gateway/internal/grading"
	"edugrade-enterprise/services/api-gateway/internal/logger"
	ocrpkg "edugrade-enterprise/services/api-gateway/internal/ocr"
	"edugrade-enterprise/services/api-gateway/internal/org"
	"edugrade-enterprise/services/api-gateway/internal/paper"
	"edugrade-enterprise/services/api-gateway/internal/segment"
	"edugrade-enterprise/services/api-gateway/internal/server"
	"edugrade-enterprise/services/api-gateway/internal/submission"
)

const tenantID = "00000000-0000-0000-0000-000000000002"
const userID = "00000000-0000-0000-0000-000000000801"

func TestRecordAnswerRuleGradeListAndAudit(t *testing.T) {
	authStore := authStoreWithPermissions(t, []string{"grading:manage"})
	gradingStore := grading.NewMemoryStore()
	gradingStore.AddContext(tenantID, "segment-1", questionWithAnswerKey("single_choice", "B", nil, nil))
	router := testRouter(authStore, gradingStore)
	token := login(t, router)

	req := authedRequest(http.MethodPut, "/api/v1/answer-segments/segment-1/answer", bytes.NewBufferString(`{"answer_text":"B","source":"manual_entry","confidence":0.99}`), token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"answer_text":"B"`) {
		t.Fatalf("record answer expected 200, got %d %s", rec.Code, rec.Body.String())
	}

	req = authedRequest(http.MethodPost, "/api/v1/answer-segments/segment-1/rule-grade", nil, token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated || !strings.Contains(rec.Body.String(), `"suggested_score":5`) || !strings.Contains(rec.Body.String(), `"grader_type":"rule_based_objective"`) || !strings.Contains(rec.Body.String(), `"mock":false`) {
		t.Fatalf("rule grade expected created full score, got %d %s", rec.Code, rec.Body.String())
	}

	req = authedRequest(http.MethodGet, "/api/v1/answer-segments/segment-1/ai-grades", nil, token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || strings.Count(rec.Body.String(), `"suggested_score"`) != 1 {
		t.Fatalf("list grades expected one grade, got %d %s", rec.Code, rec.Body.String())
	}

	assertAuditAction(t, authStore, "grading.answer_recorded")
	assertAuditAction(t, authStore, "grading.rule_grade_created")
}

func TestRuleGradeMissingAnswerAndAnswerKey(t *testing.T) {
	authStore := authStoreWithPermissions(t, []string{"grading:manage"})
	gradingStore := grading.NewMemoryStore()
	gradingStore.AddContext(tenantID, "segment-no-answer", questionWithAnswerKey("single_choice", "A", nil, nil))
	gradingStore.AddContext(tenantID, "segment-no-key", paper.Question{ID: "question-no-key", TenantID: tenantID, ExamID: "exam-1", QuestionNo: "Q2", QuestionType: "single_choice", Score: 5})
	router := testRouter(authStore, gradingStore)
	token := login(t, router)

	req := authedRequest(http.MethodPost, "/api/v1/answer-segments/segment-no-answer/rule-grade", nil, token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "answer_segment_answer_missing") {
		t.Fatalf("missing answer expected conflict, got %d %s", rec.Code, rec.Body.String())
	}

	req = authedRequest(http.MethodPut, "/api/v1/answer-segments/segment-no-key/answer", bytes.NewBufferString(`{"answer_text":"A","source":"manual_entry"}`), token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("record answer expected 200, got %d %s", rec.Code, rec.Body.String())
	}
	req = authedRequest(http.MethodPost, "/api/v1/answer-segments/segment-no-key/rule-grade", nil, token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "question_answer_key_missing") {
		t.Fatalf("missing answer key expected conflict, got %d %s", rec.Code, rec.Body.String())
	}
}

func TestGradingPermissionDeniedAndUnauthenticated(t *testing.T) {
	gradingStore := grading.NewMemoryStore()
	gradingStore.AddContext(tenantID, "segment-1", questionWithAnswerKey("single_choice", "A", nil, nil))
	router := testRouter(authStoreWithPermissions(t, []string{"system:read"}), gradingStore)
	token := login(t, router)

	req := authedRequest(http.MethodPut, "/api/v1/answer-segments/segment-1/answer", bytes.NewBufferString(`{"answer_text":"A"}`), token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/v1/answer-segments/segment-1/rule-grade", nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d %s", rec.Code, rec.Body.String())
	}
}

func testRouter(authStore *auth.MemoryStore, gradingStore grading.Store) http.Handler {
	cfg := config.Config{
		Service: config.ServiceConfig{Name: "test", Environment: "test", ReadinessTimeout: time.Millisecond},
		Auth:    config.AuthConfig{SessionTTL: time.Hour},
	}
	return server.NewRouterComplete(cfg, logger.New(io.Discard, "error"), nil, authStore, org.NewMemoryStore(), exam.NewMemoryStore(), paper.NewMemoryStore(), files.NewMemoryStore(), files.NewMemoryObjectStorage(), submission.NewMemoryStore(), ocrpkg.NewMemoryStore(), ocrpkg.NewMemoryQueue(), segment.NewMemoryStore(), gradingStore)
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
			ID:          userID,
			TenantID:    tenantID,
			TenantCode:  "demo",
			Username:    "grading_admin",
			DisplayName: "Grading Admin",
			Status:      "active",
			Roles:       []string{"teacher"},
			Permissions: permissions,
			DataScope:   map[string]any{"scope": "school"},
		},
		PasswordHash: hash,
	})
	return store
}

func login(t *testing.T, router http.Handler) string {
	t.Helper()
	raw, _ := json.Marshal(map[string]string{"tenant_code": "demo", "username": "grading_admin", "password": "ChangeMe123!"})
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

func authedRequest(method string, path string, body *bytes.Buffer, token string) *http.Request {
	var reader io.Reader
	if body != nil {
		reader = body
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Authorization", "Bearer "+token)
	return req
}

func questionWithAnswerKey(kind string, standard any, equiv []any, tolerance any) paper.Question {
	return paper.Question{
		ID:           "question-" + kind,
		TenantID:     tenantID,
		ExamID:       "exam-1",
		QuestionNo:   "Q1",
		QuestionType: kind,
		Score:        5,
		AnswerKey: &paper.AnswerKey{
			ID:                "answer-key-" + kind,
			QuestionID:        "question-" + kind,
			AnswerVersion:     "v1",
			StandardAnswer:    standard,
			EquivalentAnswers: equiv,
			Tolerance:         tolerance,
		},
	}
}

func assertAuditAction(t *testing.T, store *auth.MemoryStore, action string) {
	t.Helper()
	for _, audit := range store.Audits() {
		if audit.Action == action {
			return
		}
	}
	t.Fatalf("missing audit action %s in %#v", action, store.Audits())
}
