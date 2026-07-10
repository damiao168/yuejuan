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

	appealpkg "edugrade-enterprise/services/api-gateway/internal/appeal"
	"edugrade-enterprise/services/api-gateway/internal/auth"
	"edugrade-enterprise/services/api-gateway/internal/config"
	"edugrade-enterprise/services/api-gateway/internal/exam"
	"edugrade-enterprise/services/api-gateway/internal/files"
	"edugrade-enterprise/services/api-gateway/internal/logger"
	ocrpkg "edugrade-enterprise/services/api-gateway/internal/ocr"
	"edugrade-enterprise/services/api-gateway/internal/org"
	"edugrade-enterprise/services/api-gateway/internal/paper"
	"edugrade-enterprise/services/api-gateway/internal/segment"
	"edugrade-enterprise/services/api-gateway/internal/submission"
)

const appealTenantID = "00000000-0000-0000-0000-000000000002"
const appealStudentUserID = "00000000-0000-0000-0000-000000000801"
const appealTeacherUserID = "00000000-0000-0000-0000-000000000802"

func TestAppealRoutesCreateReviewAdjustCloseAndStatistics(t *testing.T) {
	authStore := appealAuthStore(t)
	appealStore := seededRouteAppealStore()
	router := appealRouter(authStore, appealStore)
	studentToken := appealLogin(t, router, "appeal_student")
	teacherToken := appealLogin(t, router, "appeal_teacher")

	req := appealAuthedRequest(http.MethodPost, "/api/v1/appeals", `{"exam_id":"exam-1","student_id":"student-2","target_type":"question","final_grade_id":"final-1","reason":"not mine"}`, studentToken)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("student should not appeal another student grade, got %d %s", rec.Code, rec.Body.String())
	}

	req = appealAuthedRequest(http.MethodPost, "/api/v1/appeals", `{"exam_id":"exam-1","student_id":"student-1","target_type":"question","final_grade_id":"final-1","reason":"Q1 should receive full credit","attachment":{"file_id":"file-1"}}`, studentToken)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated || !strings.Contains(rec.Body.String(), `"status":"submitted"`) || !strings.Contains(rec.Body.String(), `"raw_answer":"student wrote a correct explanation"`) {
		t.Fatalf("create appeal expected submitted appeal with evidence, got %d %s", rec.Code, rec.Body.String())
	}
	appealID := decodeAppealID(t, rec.Body.Bytes())

	req = appealAuthedRequest(http.MethodPost, "/api/v1/appeals/"+appealID+"/review", `{"status":"score_adjusted","reason":"rubric evidence supports full credit","adjusted_score":5}`, teacherToken)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"score_adjustment"`) || !strings.Contains(rec.Body.String(), `"adjusted_score":5`) {
		t.Fatalf("review appeal expected score adjustment, got %d %s", rec.Code, rec.Body.String())
	}

	req = appealAuthedRequest(http.MethodGet, "/api/v1/appeals/"+appealID, "", studentToken)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"status":"score_adjusted"`) || !strings.Contains(rec.Body.String(), `"adjustments"`) {
		t.Fatalf("student should see appeal result, got %d %s", rec.Code, rec.Body.String())
	}

	req = appealAuthedRequest(http.MethodGet, "/api/v1/appeals/statistics?exam_id=exam-1", "", teacherToken)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"score_adjusted_count":1`) {
		t.Fatalf("statistics expected score adjusted count, got %d %s", rec.Code, rec.Body.String())
	}

	req = appealAuthedRequest(http.MethodPost, "/api/v1/appeals/"+appealID+"/close", `{"reason":"resolved"}`, teacherToken)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"status":"closed"`) {
		t.Fatalf("close appeal expected closed, got %d %s", rec.Code, rec.Body.String())
	}

	reviewAssertAuditAction(t, authStore, "appeal.created")
	reviewAssertAuditAction(t, authStore, "appeal.reviewed")
	reviewAssertAuditAction(t, authStore, "appeal.score_adjusted")
	reviewAssertAuditAction(t, authStore, "appeal.closed")
}

func TestAppealRoutesRequirePermission(t *testing.T) {
	authStore := appealAuthStoreWithPermissions(t, []string{"system:read"}, map[string]any{"student_id": "student-1"})
	router := appealRouter(authStore, seededRouteAppealStore())
	token := appealLogin(t, router, "appeal_student")

	req := appealAuthedRequest(http.MethodPost, "/api/v1/appeals", `{"exam_id":"exam-1","student_id":"student-1","target_type":"exam","reason":"score issue"}`, token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("create appeal expected 403, got %d %s", rec.Code, rec.Body.String())
	}
}

func appealRouter(authStore *auth.MemoryStore, appealStore appealpkg.Store) http.Handler {
	cfg := config.Config{
		Service: config.ServiceConfig{Name: "test", Environment: "test", ReadinessTimeout: time.Millisecond},
		Auth:    config.AuthConfig{SessionTTL: time.Hour},
	}
	return NewRouterComplete(cfg, logger.New(io.Discard, "error"), nil, authStore, org.NewMemoryStore(), exam.NewMemoryStore(), paper.NewMemoryStore(), files.NewMemoryStore(), files.NewMemoryObjectStorage(), submission.NewMemoryStore(), ocrpkg.NewMemoryStore(), ocrpkg.NewMemoryQueue(), segment.NewMemoryStore(), appealStore)
}

func appealAuthStore(t *testing.T) *auth.MemoryStore {
	store := auth.NewMemoryStore()
	hash, err := auth.HashPassword("ChangeMe123!")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	users := []auth.User{
		{
			ID:          appealStudentUserID,
			TenantID:    appealTenantID,
			TenantCode:  "demo",
			Username:    "appeal_student",
			DisplayName: "Appeal Student",
			Status:      "active",
			Roles:       []string{"student"},
			Permissions: []string{"appeal:create", "appeal:read"},
			DataScope:   map[string]any{"scope": "self", "student_id": "student-1"},
		},
		{
			ID:          appealTeacherUserID,
			TenantID:    appealTenantID,
			TenantCode:  "demo",
			Username:    "appeal_teacher",
			DisplayName: "Appeal Teacher",
			Status:      "active",
			Roles:       []string{"teacher"},
			Permissions: []string{"appeal:read", "appeal:manage"},
			DataScope:   map[string]any{"scope": "school"},
		},
	}
	for _, user := range users {
		store.AddUser(auth.UserWithPassword{User: user, PasswordHash: hash})
	}
	return store
}

func appealAuthStoreWithPermissions(t *testing.T, permissions []string, dataScope map[string]any) *auth.MemoryStore {
	store := auth.NewMemoryStore()
	hash, err := auth.HashPassword("ChangeMe123!")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	store.AddUser(auth.UserWithPassword{
		User: auth.User{
			ID:          appealStudentUserID,
			TenantID:    appealTenantID,
			TenantCode:  "demo",
			Username:    "appeal_student",
			DisplayName: "Appeal Student",
			Status:      "active",
			Roles:       []string{"student"},
			Permissions: permissions,
			DataScope:   dataScope,
		},
		PasswordHash: hash,
	})
	return store
}

func appealLogin(t *testing.T, router http.Handler, username string) string {
	t.Helper()
	raw, _ := json.Marshal(map[string]string{"tenant_code": "demo", "username": username, "password": "ChangeMe123!"})
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
		t.Fatalf("decode login: %v", err)
	}
	return response.AccessToken
}

func appealAuthedRequest(method string, path string, body string, token string) *http.Request {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	return req
}

func decodeAppealID(t *testing.T, raw []byte) string {
	t.Helper()
	var response struct {
		Appeal struct {
			ID string `json:"id"`
		} `json:"appeal"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		t.Fatalf("decode appeal: %v", err)
	}
	if response.Appeal.ID == "" {
		t.Fatalf("missing appeal id in %s", string(raw))
	}
	return response.Appeal.ID
}

func seededRouteAppealStore() *appealpkg.MemoryStore {
	store := appealpkg.NewMemoryStore()
	store.AddSubmissionGrade(appealpkg.SubmissionGradeSeed{
		ID:            "submission-grade-1",
		ExamID:        "exam-1",
		SubmissionID:  "submission-1",
		StudentID:     "student-1",
		AnonymousCode: "ANON-001",
		TotalScore:    8,
		MaxScore:      10,
		Status:        "published",
		Locked:        true,
	})
	store.AddFinalGrade(appealpkg.FinalGradeSeed{
		ID:              "final-1",
		ExamID:          "exam-1",
		SubmissionID:    "submission-1",
		AnswerSegmentID: "segment-1",
		QuestionID:      "question-1",
		QuestionNo:      "Q1",
		Score:           4,
		MaxScore:        5,
		Status:          "locked",
		Locked:          true,
		RawAnswer:       "student wrote a correct explanation",
		OCRText:         "student wrote a correct explanation",
		AIGrades:        []map[string]any{{"suggested_score": 4.0, "mock": false}},
		HumanGrades:     []map[string]any{{"score": 4.0, "reviewer_id": "reviewer-1"}},
		Rubric:          map[string]any{"points": []any{"main idea"}},
	})
	return store
}
