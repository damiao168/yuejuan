package ocr_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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
	"edugrade-enterprise/services/api-gateway/internal/logger"
	ocrpkg "edugrade-enterprise/services/api-gateway/internal/ocr"
	"edugrade-enterprise/services/api-gateway/internal/org"
	"edugrade-enterprise/services/api-gateway/internal/paper"
	"edugrade-enterprise/services/api-gateway/internal/segment"
	"edugrade-enterprise/services/api-gateway/internal/server"
	"edugrade-enterprise/services/api-gateway/internal/submission"
	"edugrade-enterprise/services/api-gateway/internal/workerruntime"
)

const tenantID = "00000000-0000-0000-0000-000000000002"
const userID = "00000000-0000-0000-0000-000000000401"

func TestCreateStartCompleteOCRTaskWithLowConfidence(t *testing.T) {
	authStore := authStoreWithPermissions(t, []string{"ocr:manage"})
	submissionStore := submission.NewMemoryStore()
	page := readySubmission(t, submissionStore)
	ocrStore := ocrpkg.NewMemoryStore()
	queue := ocrpkg.NewMemoryQueue()
	runtimeStore := workerruntime.NewMemoryStore()
	router := testRouter(authStore, submissionStore, ocrStore, queue, runtimeStore)
	token := login(t, router)

	task := createTask(t, router, token, page.SubmissionID)
	if len(queue.Tasks()) != 0 {
		t.Fatalf("runtime-backed OCR must not use the legacy in-memory queue, got %d entries", len(queue.Tasks()))
	}
	runtimeTask, err := runtimeStore.GetBySource(context.Background(), tenantID, "ocr_task", task.ID)
	if err != nil || runtimeTask.Status != workerruntime.StatusQueued {
		t.Fatalf("ocr creation should enqueue runtime task: %v %#v", err, runtimeTask)
	}
	runtimeTask = claimRuntimeTask(t, runtimeStore, task.ID)

	req := authedRequest(http.MethodPost, "/api/v1/ocr-tasks/"+task.ID+"/start", nil, token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "processing") {
		t.Fatalf("start expected processing, got %d %s", rec.Code, rec.Body.String())
	}

	body := withRuntime(`{"results":[{"submission_page_id":"`+page.ID+`","text":"Synthetic OCR text","bbox":[10,20,100,40],"confidence":0.52}]}`, runtimeTask)
	req = authedRequest(http.MethodPost, "/api/v1/ocr-tasks/"+task.ID+"/results", bytes.NewBufferString(body), token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"requires_human_review":true`) || !strings.Contains(rec.Body.String(), `"ocr_engine":"paddleocr"`) {
		t.Fatalf("complete expected low-confidence review flag, got %d %s", rec.Code, rec.Body.String())
	}

	assertAuditAction(t, authStore, "ocr.task_created")
	assertAuditAction(t, authStore, "ocr.task_started")
	assertAuditAction(t, authStore, "ocr.task_completed")
}

func TestOCRTaskRequiresReadySubmission(t *testing.T) {
	authStore := authStoreWithPermissions(t, []string{"ocr:manage"})
	submissionStore := submission.NewMemoryStore()
	item, err := submissionStore.Create(context.Background(), tenantID, "exam-1", userID, submission.CreateSubmissionInput{SourceType: "scanner_upload", ExpectedPageCount: 1})
	if err != nil {
		t.Fatalf("create submission: %v", err)
	}
	router := testRouter(authStore, submissionStore, ocrpkg.NewMemoryStore(), ocrpkg.NewMemoryQueue())
	token := login(t, router)

	req := authedRequest(http.MethodPost, "/api/v1/submissions/"+item.ID+"/ocr-tasks", bytes.NewBufferString(validTaskJSON()), token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "submission_not_ready_for_ocr") {
		t.Fatalf("expected submission not ready conflict, got %d %s", rec.Code, rec.Body.String())
	}
}

func TestCompleteBeforeStartRejected(t *testing.T) {
	authStore := authStoreWithPermissions(t, []string{"ocr:manage"})
	submissionStore := submission.NewMemoryStore()
	page := readySubmission(t, submissionStore)
	runtimeStore := workerruntime.NewMemoryStore()
	router := testRouter(authStore, submissionStore, ocrpkg.NewMemoryStore(), ocrpkg.NewMemoryQueue(), runtimeStore)
	token := login(t, router)
	task := createTask(t, router, token, page.SubmissionID)
	runtimeTask := claimRuntimeTask(t, runtimeStore, task.ID)

	body := withRuntime(`{"results":[{"submission_page_id":"`+page.ID+`","text":"Synthetic OCR text","bbox":[10,20,100,40],"confidence":0.9}]}`, runtimeTask)
	req := authedRequest(http.MethodPost, "/api/v1/ocr-tasks/"+task.ID+"/results", bytes.NewBufferString(body), token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "invalid_ocr_status_transition") {
		t.Fatalf("expected transition conflict, got %d %s", rec.Code, rec.Body.String())
	}
}

func TestOCRTaskFail(t *testing.T) {
	authStore := authStoreWithPermissions(t, []string{"ocr:manage"})
	submissionStore := submission.NewMemoryStore()
	page := readySubmission(t, submissionStore)
	runtimeStore := workerruntime.NewMemoryStore()
	router := testRouter(authStore, submissionStore, ocrpkg.NewMemoryStore(), ocrpkg.NewMemoryQueue(), runtimeStore)
	token := login(t, router)
	task := createTask(t, router, token, page.SubmissionID)
	runtimeTask := claimRuntimeTask(t, runtimeStore, task.ID)

	req := authedRequest(http.MethodPost, "/api/v1/ocr-tasks/"+task.ID+"/fail", bytes.NewBufferString(withRuntime(`{"error_message":"worker timeout"}`, runtimeTask)), token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"status":"failed"`) {
		t.Fatalf("fail expected failed status, got %d %s", rec.Code, rec.Body.String())
	}
	assertAuditAction(t, authStore, "ocr.task_failed")
}

func TestOCRPermissionDenied(t *testing.T) {
	submissionStore := submission.NewMemoryStore()
	page := readySubmission(t, submissionStore)
	router := testRouter(authStoreWithPermissions(t, []string{"system:read"}), submissionStore, ocrpkg.NewMemoryStore(), ocrpkg.NewMemoryQueue())
	token := login(t, router)
	req := authedRequest(http.MethodPost, "/api/v1/submissions/"+page.SubmissionID+"/ocr-tasks", bytes.NewBufferString(validTaskJSON()), token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d %s", rec.Code, rec.Body.String())
	}
}

func TestPendingOCRTasksReturnsOnlyQueuedTasks(t *testing.T) {
	authStore := authStoreWithPermissions(t, []string{"ocr:manage"})
	submissionStore := submission.NewMemoryStore()
	pageA := readySubmission(t, submissionStore)
	pageB := readySubmission(t, submissionStore)
	ocrStore := ocrpkg.NewMemoryStore()
	router := testRouter(authStore, submissionStore, ocrStore, ocrpkg.NewMemoryQueue())
	token := login(t, router)
	processingTask := createTask(t, router, token, pageA.SubmissionID)
	queuedTask := createTask(t, router, token, pageB.SubmissionID)

	req := authedRequest(http.MethodPost, "/api/v1/ocr-tasks/"+processingTask.ID+"/start", nil, token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("start expected 200, got %d %s", rec.Code, rec.Body.String())
	}

	req = authedRequest(http.MethodGet, "/api/v1/ocr-tasks/pending?limit=10", nil, token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("pending expected 200, got %d %s", rec.Code, rec.Body.String())
	}
	var response struct {
		Tasks []ocrpkg.Task `json:"tasks"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(response.Tasks) != 1 || response.Tasks[0].ID != queuedTask.ID {
		t.Fatalf("pending should return only queued task %s, got %#v", queuedTask.ID, response.Tasks)
	}
}

func TestOCRTaskInputOmitsStudentIdentityAndIncludesPages(t *testing.T) {
	authStore := authStoreWithPermissions(t, []string{"ocr:manage"})
	submissionStore := submission.NewMemoryStore()
	page := readySubmission(t, submissionStore)
	runtimeStore := workerruntime.NewMemoryStore()
	router := testRouter(authStore, submissionStore, ocrpkg.NewMemoryStore(), ocrpkg.NewMemoryQueue(), runtimeStore)
	token := login(t, router)
	task := createTask(t, router, token, page.SubmissionID)

	req := authedRequest(http.MethodGet, "/api/v1/ocr-tasks/"+task.ID+"/input", nil, token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("input expected 200, got %d %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if strings.Contains(body, "student_id") || strings.Contains(body, "candidate_no") {
		t.Fatalf("input must not expose student identity fields: %s", body)
	}
	if !strings.Contains(body, page.ID) || !strings.Contains(body, page.FileAssetID) {
		t.Fatalf("input should include submission page and file asset ids, got %s", body)
	}
}

func TestCompleteOCRTaskRejectsInvalidBBoxSchema(t *testing.T) {
	authStore := authStoreWithPermissions(t, []string{"ocr:manage"})
	submissionStore := submission.NewMemoryStore()
	page := readySubmission(t, submissionStore)
	runtimeStore := workerruntime.NewMemoryStore()
	router := testRouter(authStore, submissionStore, ocrpkg.NewMemoryStore(), ocrpkg.NewMemoryQueue(), runtimeStore)
	token := login(t, router)
	task := createTask(t, router, token, page.SubmissionID)
	runtimeTask := claimRuntimeTask(t, runtimeStore, task.ID)
	req := authedRequest(http.MethodPost, "/api/v1/ocr-tasks/"+task.ID+"/start", nil, token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("start expected 200, got %d %s", rec.Code, rec.Body.String())
	}

	body := withRuntime(`{"results":[{"submission_page_id":"`+page.ID+`","text":"bad bbox","bbox":[-1,20,100,40],"confidence":0.9}]}`, runtimeTask)
	req = authedRequest(http.MethodPost, "/api/v1/ocr-tasks/"+task.ID+"/results", bytes.NewBufferString(body), token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid bbox expected 400, got %d %s", rec.Code, rec.Body.String())
	}
}

func TestCompleteOCRTaskPersistsWorkerMetadata(t *testing.T) {
	authStore := authStoreWithPermissions(t, []string{"ocr:manage"})
	submissionStore := submission.NewMemoryStore()
	page := readySubmission(t, submissionStore)
	runtimeStore := workerruntime.NewMemoryStore()
	router := testRouter(authStore, submissionStore, ocrpkg.NewMemoryStore(), ocrpkg.NewMemoryQueue(), runtimeStore)
	token := login(t, router)
	task := createTask(t, router, token, page.SubmissionID)
	runtimeTask := claimRuntimeTask(t, runtimeStore, task.ID)
	req := authedRequest(http.MethodPost, "/api/v1/ocr-tasks/"+task.ID+"/start", nil, token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("start expected 200, got %d %s", rec.Code, rec.Body.String())
	}

	body := withRuntime(`{"model_version":"ppocr-v5-server","config_hash":"cfg123","input_hash":"input123","duration_ms":42,"worker_id":"worker-a","preprocess_profile":"default","results":[{"submission_page_id":"`+page.ID+`","text":"Real OCR text","bbox":[10,20,100,40],"confidence":0.91}]}`, runtimeTask)
	req = authedRequest(http.MethodPost, "/api/v1/ocr-tasks/"+task.ID+"/results", bytes.NewBufferString(body), token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("complete expected 200, got %d %s", rec.Code, rec.Body.String())
	}
	body = rec.Body.String()
	for _, want := range []string{`"model_version":"ppocr-v5-server"`, `"config_hash":"cfg123"`, `"input_hash":"input123"`, `"duration_ms":42`, `"worker_id":"worker-a"`, `"preprocess_profile":"default"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("response missing %s: %s", want, body)
		}
	}
}

func TestCompleteOCRTaskIsIdempotentForSamePayload(t *testing.T) {
	authStore := authStoreWithPermissions(t, []string{"ocr:manage"})
	submissionStore := submission.NewMemoryStore()
	page := readySubmission(t, submissionStore)
	runtimeStore := workerruntime.NewMemoryStore()
	router := testRouter(authStore, submissionStore, ocrpkg.NewMemoryStore(), ocrpkg.NewMemoryQueue(), runtimeStore)
	token := login(t, router)
	task := createTask(t, router, token, page.SubmissionID)
	runtimeTask := claimRuntimeTask(t, runtimeStore, task.ID)

	req := authedRequest(http.MethodPost, "/api/v1/ocr-tasks/"+task.ID+"/start", nil, token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("start expected 200, got %d %s", rec.Code, rec.Body.String())
	}
	body := withRuntime(`{"model_version":"ppocr-v5-server","config_hash":"cfg","input_hash":"input","duration_ms":42,"worker_id":"worker-a","preprocess_profile":"default","results":[{"submission_page_id":"`+page.ID+`","text":"OCR text","bbox":[10,20,100,40],"confidence":0.91}]}`, runtimeTask)
	for attempt := 0; attempt < 2; attempt++ {
		req = authedRequest(http.MethodPost, "/api/v1/ocr-tasks/"+task.ID+"/results", bytes.NewBufferString(body), token)
		rec = httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"result_count":1`) {
			t.Fatalf("completion attempt %d expected idempotent 200, got %d %s", attempt+1, rec.Code, rec.Body.String())
		}
	}

	changed := strings.Replace(body, `"OCR text"`, `"changed text"`, 1)
	req = authedRequest(http.MethodPost, "/api/v1/ocr-tasks/"+task.ID+"/results", bytes.NewBufferString(changed), token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "ocr_result_conflict") {
		t.Fatalf("changed duplicate expected 409, got %d %s", rec.Code, rec.Body.String())
	}
}

func testRouter(authStore *auth.MemoryStore, submissionStore submission.Store, ocrStore ocrpkg.Store, queue *ocrpkg.MemoryQueue, runtimeStores ...workerruntime.Store) http.Handler {
	cfg := config.Config{
		Service: config.ServiceConfig{Name: "test", Environment: "test", ReadinessTimeout: time.Millisecond},
		Auth:    config.AuthConfig{SessionTTL: time.Hour},
	}
	optionalStores := []any{}
	if len(runtimeStores) > 0 {
		optionalStores = append(optionalStores, runtimeStores[0])
	}
	return server.NewRouterComplete(cfg, logger.New(io.Discard, "error"), nil, authStore, org.NewMemoryStore(), exam.NewMemoryStore(), paper.NewMemoryStore(), files.NewMemoryStore(), files.NewMemoryObjectStorage(), submissionStore, ocrStore, queue, segment.NewMemoryStore(), optionalStores...)
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
			Username:    "ocr_admin",
			DisplayName: "OCR Admin",
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
	raw, _ := json.Marshal(map[string]string{"tenant_code": "demo", "username": "ocr_admin", "password": "ChangeMe123!"})
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
		t.Fatalf("decode: %v", err)
	}
	return response.AccessToken
}

func readySubmission(t *testing.T, store submission.Store) submission.SubmissionPage {
	t.Helper()
	ctx := context.Background()
	item, err := store.Create(ctx, tenantID, "exam-1", userID, submission.CreateSubmissionInput{SourceType: "scanner_upload", ExpectedPageCount: 1})
	if err != nil {
		t.Fatalf("create submission: %v", err)
	}
	page, err := store.AddPage(ctx, tenantID, item.ID, userID, submission.AddPageInput{FileAssetID: "file-1", PageNo: 1})
	if err != nil {
		t.Fatalf("add page: %v", err)
	}
	if _, err := store.RunQualityCheck(ctx, tenantID, item.ID, userID); err != nil {
		t.Fatalf("quality check: %v", err)
	}
	if _, err := store.UpdateStatus(ctx, tenantID, item.ID, userID, "ready_for_ocr", item.Revision); err != nil {
		t.Fatalf("ready status: %v", err)
	}
	return page
}

func createTask(t *testing.T, router http.Handler, token string, submissionID string) ocrpkg.Task {
	t.Helper()
	req := authedRequest(http.MethodPost, "/api/v1/submissions/"+submissionID+"/ocr-tasks", bytes.NewBufferString(validTaskJSON()), token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create task expected 201, got %d %s", rec.Code, rec.Body.String())
	}
	var response struct {
		Task ocrpkg.Task `json:"task"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return response.Task
}

func validTaskJSON() string {
	return `{"engine":"paddleocr","engine_version":"pp-ocrv5","min_confidence":0.8}`
}

func claimRuntimeTask(t *testing.T, store *workerruntime.MemoryStore, sourceTaskID string) workerruntime.Task {
	t.Helper()
	tasks, err := store.Claim(context.Background(), tenantID, workerruntime.ClaimInput{
		QueueName: "ocr", WorkerService: "ocr-worker", WorkerInstanceID: "ocr-test-1",
		Limit: 1, LeaseSeconds: 300,
	})
	if err != nil || len(tasks) != 1 {
		t.Fatalf("claim OCR runtime task: %v %#v", err, tasks)
	}
	if tasks[0].SourceType != "ocr_task" || tasks[0].SourceID != sourceTaskID {
		t.Fatalf("claimed runtime task does not belong to source %s: %#v", sourceTaskID, tasks[0])
	}
	return tasks[0]
}

func withRuntime(body string, task workerruntime.Task) string {
	return strings.TrimSuffix(body, "}") + fmt.Sprintf(
		`,"runtime_task_id":%q,"runtime_lease_token":%q}`,
		task.ID, task.LeaseToken,
	)
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

func assertAuditAction(t *testing.T, store *auth.MemoryStore, action string) {
	t.Helper()
	for _, audit := range store.Audits() {
		if audit.Action == action {
			return
		}
	}
	t.Fatalf("missing audit action %s in %#v", action, store.Audits())
}
