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
	"edugrade-enterprise/services/api-gateway/internal/files"
	"edugrade-enterprise/services/api-gateway/internal/logger"
	reportpkg "edugrade-enterprise/services/api-gateway/internal/report"
)

const reportTenantID = "00000000-0000-0000-0000-000000000003"
const reportTeacherID = "00000000-0000-0000-0000-000000000901"
const reportStudentID = "00000000-0000-0000-0000-000000000902"

func TestReportRoutesGenerateStudentClassQuestionQualityAndExport(t *testing.T) {
	authStore := reportAuthStore(t)
	reportStore := seededRouteReportStore()
	router := reportRouter(authStore, reportStore)
	teacherToken := reportLogin(t, router, "report_teacher")
	studentToken := reportLogin(t, router, "report_student")

	req := reportAuthedRequest(http.MethodGet, "/api/v1/students/student-2/reports/exam-1", "", studentToken)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("student should not view another report, got %d %s", rec.Code, rec.Body.String())
	}

	req = reportAuthedRequest(http.MethodGet, "/api/v1/students/student-1/reports/exam-1", "", studentToken)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"total_score":85`) || !strings.Contains(rec.Body.String(), `"knowledge_mastery"`) {
		t.Fatalf("student report expected score and mastery, got %d %s", rec.Code, rec.Body.String())
	}

	for _, path := range []string{
		"/api/v1/exams/exam-1/reports/overview",
		"/api/v1/exams/exam-1/reports/classes",
		"/api/v1/exams/exam-1/reports/questions",
		"/api/v1/exams/exam-1/reports/grading-quality",
	} {
		req = reportAuthedRequest(http.MethodGet, path, "", teacherToken)
		rec = httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s expected 200, got %d %s", path, rec.Code, rec.Body.String())
		}
	}

	req = reportAuthedRequest(http.MethodPost, "/api/v1/exams/exam-1/reports/export", "", teacherToken)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "EduGrade report export") || rec.Header().Get("X-EduGrade-Watermark") == "" {
		t.Fatalf("report export expected watermarked CSV, got %d headers=%v body=%s", rec.Code, rec.Header(), rec.Body.String())
	}

	reviewAssertAuditAction(t, authStore, "report.generated")
	reviewAssertAuditAction(t, authStore, "report.exported")
	if reportStore.ExportCount() != 1 {
		t.Fatalf("export should record report snapshot, got %d", reportStore.ExportCount())
	}
}

func TestReportRoutesRequirePermission(t *testing.T) {
	authStore := reportAuthStoreWithPermissions(t, []string{"system:read"}, map[string]any{"student_id": "student-1"})
	router := reportRouter(authStore, seededRouteReportStore())
	token := reportLogin(t, router, "report_student")

	req := reportAuthedRequest(http.MethodGet, "/api/v1/exams/exam-1/reports/overview", "", token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("overview expected 403, got %d %s", rec.Code, rec.Body.String())
	}

	req = reportAuthedRequest(http.MethodGet, "/api/v1/students/student-1/reports/exam-1", "", token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("student report without report permission expected 403, got %d %s", rec.Code, rec.Body.String())
	}
}

func reportRouter(authStore *auth.MemoryStore, reportStore reportpkg.Store) http.Handler {
	cfg := config.Config{
		Service: config.ServiceConfig{Name: "test", Environment: "test", ReadinessTimeout: time.Millisecond},
		Auth:    config.AuthConfig{SessionTTL: time.Hour},
	}
	return NewMemoryRouter(cfg, logger.New(io.Discard, "error"), nil, files.NewMemoryObjectStorage(), func(stores *ApplicationStores) {
		stores.Identity.Auth = authStore
		stores.Release.Report = reportStore
	})
}

func reportAuthStore(t *testing.T) *auth.MemoryStore {
	store := auth.NewMemoryStore()
	hash, err := auth.HashPassword("ChangeMe123!")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	users := []auth.User{
		{
			ID:          reportTeacherID,
			TenantID:    reportTenantID,
			TenantCode:  "demo",
			Username:    "report_teacher",
			DisplayName: "Report Teacher",
			Status:      "active",
			Roles:       []string{"teacher"},
			Permissions: []string{"report:read", "report:export"},
			DataScope:   map[string]any{"scope": "school", "synthetic": true},
		},
		{
			ID:          reportStudentID,
			TenantID:    reportTenantID,
			TenantCode:  "demo",
			Username:    "report_student",
			DisplayName: "Report Student",
			Status:      "active",
			Roles:       []string{"student"},
			Permissions: []string{"student:report:read"},
			DataScope:   map[string]any{"scope": "self", "student_id": "student-1", "synthetic": true},
		},
	}
	for _, user := range users {
		store.AddUser(auth.UserWithPassword{User: user, PasswordHash: hash})
	}
	return store
}

func reportAuthStoreWithPermissions(t *testing.T, permissions []string, dataScope map[string]any) *auth.MemoryStore {
	store := auth.NewMemoryStore()
	hash, err := auth.HashPassword("ChangeMe123!")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	store.AddUser(auth.UserWithPassword{
		User: auth.User{
			ID:          reportStudentID,
			TenantID:    reportTenantID,
			TenantCode:  "demo",
			Username:    "report_student",
			DisplayName: "Report Student",
			Status:      "active",
			Roles:       []string{"student"},
			Permissions: permissions,
			DataScope:   dataScope,
		},
		PasswordHash: hash,
	})
	return store
}

func reportLogin(t *testing.T, router http.Handler, username string) string {
	t.Helper()
	raw, _ := json.Marshal(map[string]string{"tenant_code": "demo", "username": username, "password": "ChangeMe123!"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/token", bytes.NewReader(raw))
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

func reportAuthedRequest(method string, path string, body string, token string) *http.Request {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	return req
}

func seededRouteReportStore() *reportpkg.MemoryStore {
	store := reportpkg.NewMemoryStore()
	store.AddSubmission(reportpkg.SubmissionSeed{ID: "sub-1", ExamID: "exam-1", StudentID: "student-1", ClassID: "class-a", ClassName: "Class A", AnonymousCode: "A001", TotalScore: 85, MaxScore: 100})
	store.AddSubmission(reportpkg.SubmissionSeed{ID: "sub-2", ExamID: "exam-1", StudentID: "student-2", ClassID: "class-a", ClassName: "Class A", AnonymousCode: "A002", TotalScore: 70, MaxScore: 100})
	aiScore := 45.0
	humanScore := 40.0
	store.AddGrade(reportpkg.GradeSeed{SubmissionID: "sub-1", StudentID: "student-1", ClassID: "class-a", ClassName: "Class A", QuestionID: "q1", QuestionNo: "Q1", QuestionType: "single_choice", AnswerSegmentID: "seg-1", Score: 50, MaxScore: 50, Source: "rule_auto", KnowledgePoints: []string{"motion"}, AnswerPayload: map[string]any{"selected_option": "A"}, AIScore: &aiScore, HumanScore: &humanScore, AIFeedbackText: "matched", TeacherFeedbackText: "good", ErrorClueText: "none"})
	store.AddGrade(reportpkg.GradeSeed{SubmissionID: "sub-1", StudentID: "student-1", ClassID: "class-a", ClassName: "Class A", QuestionID: "q2", QuestionNo: "Q2", QuestionType: "short_answer", AnswerSegmentID: "seg-2", Score: 35, MaxScore: 50, Source: "single_review", KnowledgePoints: []string{"force"}, AIScore: &aiScore, HumanScore: &humanScore, AIFeedbackText: "missing evidence", TeacherFeedbackText: "add steps", ErrorClueText: "missing force"})
	store.AddGrade(reportpkg.GradeSeed{SubmissionID: "sub-2", StudentID: "student-2", ClassID: "class-a", ClassName: "Class A", QuestionID: "q1", QuestionNo: "Q1", QuestionType: "single_choice", AnswerSegmentID: "seg-3", Score: 40, MaxScore: 50, Source: "rule_auto", KnowledgePoints: []string{"motion"}, AnswerPayload: map[string]any{"selected_option": "B"}, AIScore: &aiScore})
	store.AddGrade(reportpkg.GradeSeed{SubmissionID: "sub-2", StudentID: "student-2", ClassID: "class-a", ClassName: "Class A", QuestionID: "q2", QuestionNo: "Q2", QuestionType: "short_answer", AnswerSegmentID: "seg-4", Score: 30, MaxScore: 50, Source: "single_review", KnowledgePoints: []string{"force"}})
	store.SetQuality(reportpkg.QualitySeed{TotalSegments: 4, DoubleMarkDiffs: []float64{2}, ArbitrationCount: 1, OCRTaskCount: 2, OCRFailedCount: 1, LowConfidenceReviewCount: 1})
	return store
}
