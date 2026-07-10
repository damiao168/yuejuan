package paper_test

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

func TestCreatePaperAndQuestion(t *testing.T) {
	authStore := authStoreWithPermissions(t, []string{"exam:manage"})
	paperStore := paper.NewMemoryStore()
	router := testRouter(authStore, paperStore)
	token := login(t, router)

	paperResp := createPaper(t, router, token, "exam-1")
	if paperResp.VersionNo != 1 {
		t.Fatalf("expected paper version 1, got %d", paperResp.VersionNo)
	}

	question := createQuestion(t, router, token, "exam-1", 100)
	if question.QuestionType != "short_answer" {
		t.Fatalf("unexpected question: %#v", question)
	}

	req := authedRequest(http.MethodGet, "/api/v1/exams/exam-1/questions", nil, token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), question.ID) {
		t.Fatalf("expected question in list, got %d %s", rec.Code, rec.Body.String())
	}
}

func TestCreatePaperFromUploadedFileAssetID(t *testing.T) {
	authStore := authStoreWithPermissions(t, []string{"exam:manage"})
	paperStore := paper.NewMemoryStore()
	router := testRouter(authStore, paperStore)
	token := login(t, router)

	req := authedRequest(http.MethodPost, "/api/v1/exams/exam-1/papers", bytes.NewBufferString(`{"file_asset_id":"file-upload-1"}`), token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create paper from file_asset_id expected 201, got %d %s", rec.Code, rec.Body.String())
	}
	var response struct {
		Paper paper.Paper `json:"paper"`
		Note  string      `json:"note"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if response.Paper.FileAssetID != "file-upload-1" {
		t.Fatalf("expected linked file asset id, got %#v", response.Paper)
	}
	if !strings.Contains(response.Note, "linked") {
		t.Fatalf("expected linked note, got %q", response.Note)
	}
}

func TestInvalidQuestionTypeRejected(t *testing.T) {
	router := testRouter(authStoreWithPermissions(t, []string{"exam:manage"}), paper.NewMemoryStore())
	token := login(t, router)

	body := `{"question_no":"Q1","question_type":"magic","score":10}`
	req := authedRequest(http.MethodPost, "/api/v1/exams/exam-1/questions", bytes.NewBufferString(body), token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d %s", rec.Code, rec.Body.String())
	}
}

func TestQuestionRejectsPaperFromDifferentExam(t *testing.T) {
	authStore := authStoreWithPermissions(t, []string{"exam:manage"})
	paperStore := paper.NewMemoryStore()
	router := testRouter(authStore, paperStore)
	token := login(t, router)
	paperResp := createPaper(t, router, token, "exam-1")

	req := authedRequest(http.MethodPost, "/api/v1/exams/exam-2/questions", bytes.NewBufferString(validQuestionJSONWithPaper(10, paperResp.ID)), token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected cross-exam paper reference to be rejected, got %d %s", rec.Code, rec.Body.String())
	}
}

func TestRubricVersioningAndLock(t *testing.T) {
	router := testRouter(authStoreWithPermissions(t, []string{"exam:manage"}), paper.NewMemoryStore())
	token := login(t, router)
	question := createQuestion(t, router, token, "exam-1", 10)

	r1 := createRubric(t, router, token, question.ID, "draft", 10)
	if r1.Version != "v1" {
		t.Fatalf("expected v1, got %s", r1.Version)
	}
	r2 := createRubric(t, router, token, question.ID, "locked", 10)
	if r2.Version != "v2" || r2.Status != "locked" {
		t.Fatalf("expected locked v2, got %#v", r2)
	}

	req := authedRequest(http.MethodPost, "/api/v1/questions/"+question.ID+"/rubric", bytes.NewBufferString(validRubricJSON("draft", 10)), token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected locked rubric conflict, got %d %s", rec.Code, rec.Body.String())
	}
}

func TestRubricScoreMismatchRejected(t *testing.T) {
	router := testRouter(authStoreWithPermissions(t, []string{"exam:manage"}), paper.NewMemoryStore())
	token := login(t, router)
	question := createQuestion(t, router, token, "exam-1", 10)

	req := authedRequest(http.MethodPost, "/api/v1/questions/"+question.ID+"/rubric", bytes.NewBufferString(validRubricJSON("draft", 8)), token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected mismatch 400, got %d %s", rec.Code, rec.Body.String())
	}
}

func TestValidatePaperConfigFindsScoreMismatch(t *testing.T) {
	authStore := authStoreWithPermissions(t, []string{"exam:manage"})
	paperStore := paper.NewMemoryStore()
	paperStore.SetExamTotal("exam-1", 100)
	router := testRouter(authStore, paperStore)
	token := login(t, router)
	createQuestion(t, router, token, "exam-1", 10)

	req := authedRequest(http.MethodPost, "/api/v1/exams/exam-1/validate-paper-config", nil, token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "total_score_mismatch") {
		t.Fatalf("expected score mismatch issue, got %d %s", rec.Code, rec.Body.String())
	}
}

func TestPaperPermissionDenied(t *testing.T) {
	router := testRouter(authStoreWithPermissions(t, []string{"system:read"}), paper.NewMemoryStore())
	token := login(t, router)

	req := authedRequest(http.MethodPost, "/api/v1/exams/exam-1/questions", bytes.NewBufferString(validQuestionJSON(10)), token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d %s", rec.Code, rec.Body.String())
	}
}

func testRouter(authStore *auth.MemoryStore, paperStore *paper.MemoryStore) http.Handler {
	cfg := config.Config{
		Service: config.ServiceConfig{Name: "test", Environment: "test", ReadinessTimeout: time.Millisecond},
		Auth:    config.AuthConfig{SessionTTL: time.Hour},
	}
	return server.NewRouter(cfg, logger.New(io.Discard, "error"), nil, authStore, org.NewMemoryStore(), exam.NewMemoryStore(), paperStore)
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
			ID:          "u-paper",
			TenantID:    "tenant-paper",
			TenantCode:  "demo",
			Username:    "paper_admin",
			DisplayName: "Paper Admin",
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
	raw, _ := json.Marshal(map[string]string{"tenant_code": "demo", "username": "paper_admin", "password": "ChangeMe123!"})
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

func createPaper(t *testing.T, router http.Handler, token string, examID string) paper.Paper {
	t.Helper()
	req := authedRequest(http.MethodPost, "/api/v1/exams/"+examID+"/papers", bytes.NewBufferString(`{"file":{"original_name":"paper.pdf","content_type":"application/pdf","size_bytes":1024,"hash_sha256":"abc","storage_bucket":"papers","storage_key":"tenant/demo/paper.pdf"}}`), token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create paper expected 201, got %d %s", rec.Code, rec.Body.String())
	}
	var response struct {
		Paper paper.Paper `json:"paper"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return response.Paper
}

func createQuestion(t *testing.T, router http.Handler, token string, examID string, score float64) paper.Question {
	t.Helper()
	req := authedRequest(http.MethodPost, "/api/v1/exams/"+examID+"/questions", bytes.NewBufferString(validQuestionJSON(score)), token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create question expected 201, got %d %s", rec.Code, rec.Body.String())
	}
	var response struct {
		Question paper.Question `json:"question"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return response.Question
}

func createRubric(t *testing.T, router http.Handler, token string, questionID string, status string, score float64) paper.Rubric {
	t.Helper()
	req := authedRequest(http.MethodPost, "/api/v1/questions/"+questionID+"/rubric", bytes.NewBufferString(validRubricJSON(status, score)), token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create rubric expected 201, got %d %s", rec.Code, rec.Body.String())
	}
	var response struct {
		Rubric paper.Rubric `json:"rubric"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return response.Rubric
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

func validQuestionJSON(score float64) string {
	return `{"question_no":"Q1","question_type":"short_answer","score":` + floatString(score) + `,"stem":"Explain Newton's second law","knowledge_points":["force"],"answer_area":{"page":1,"x":10,"y":20,"w":200,"h":80},"answer_key":{"standard_answer":"F=ma","equivalent_answers":["force equals mass times acceleration"],"tolerance":{}}}`
}

func validQuestionJSONWithPaper(score float64, paperID string) string {
	return `{"exam_paper_id":"` + paperID + `","question_no":"Q1","question_type":"short_answer","score":` + floatString(score) + `,"stem":"Explain Newton's second law","knowledge_points":["force"],"answer_area":{"page":1,"x":10,"y":20,"w":200,"h":80},"answer_key":{"standard_answer":"F=ma","equivalent_answers":["force equals mass times acceleration"],"tolerance":{}}}`
}

func validRubricJSON(status string, score float64) string {
	return `{"status":"` + status + `","max_score":` + floatString(score) + `,"points":[{"id":"p1","description":"main point","score":` + floatString(score) + `,"required":true}],"deductions":[],"examples":[]}`
}

func floatString(value float64) string {
	raw, _ := json.Marshal(value)
	return string(raw)
}
