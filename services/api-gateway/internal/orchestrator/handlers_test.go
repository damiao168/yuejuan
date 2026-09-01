package orchestrator_test

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
	"edugrade-enterprise/services/api-gateway/internal/orchestrator"
	"edugrade-enterprise/services/api-gateway/internal/server"
)

const tenantID = "00000000-0000-0000-0000-000000000002"
const userID = "00000000-0000-0000-0000-000000000601"
const targetID = "00000000-0000-0000-0000-000000000701"

func TestOrchestrationLifecycleLowConfidenceReviewAndAudit(t *testing.T) {
	authStore := authStoreWithPermissions(t, []string{"orchestrator:manage"})
	orchestratorStore := orchestrator.NewMemoryStore()
	router := testRouter(authStore, orchestratorStore)
	token := login(t, router)

	run := createRun(t, router, token)
	if run.Status != "created" {
		t.Fatalf("expected created run, got %#v", run)
	}
	task := createTask(t, router, token, run.ID, `{"agent_type":"objective_grading_agent","input_ref":{"answer_segment_id":"seg-1"},"max_attempts":2}`)
	if task.Status != "queued" || task.AttemptNo != 1 {
		t.Fatalf("expected queued first attempt, got %#v", task)
	}

	req := authedRequest(http.MethodPost, "/api/v1/agent-tasks/"+task.ID+"/start", nil, token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"status":"running"`) {
		t.Fatalf("start expected running, got %d %s", rec.Code, rec.Body.String())
	}

	body := `{"output_ref":{"grading_result_id":"grade-ref-1"},"evidence_ref":{"file_asset_id":"file-1","bbox":[1,2,3,4]},"confidence":0.52}`
	req = authedRequest(http.MethodPost, "/api/v1/agent-tasks/"+task.ID+"/complete", bytes.NewBufferString(body), token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"status":"requires_human_review"`) || !strings.Contains(rec.Body.String(), `"confidence":0.52`) {
		t.Fatalf("complete expected human review, got %d %s", rec.Code, rec.Body.String())
	}

	req = authedRequest(http.MethodGet, "/api/v1/orchestrations/"+run.ID, nil, token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"status":"requires_human_review"`) || strings.Count(rec.Body.String(), `"agent_type"`) != 1 {
		t.Fatalf("get run expected one human-review task, got %d %s", rec.Code, rec.Body.String())
	}

	req = authedRequest(http.MethodGet, "/api/v1/orchestrations/"+run.ID+"/tasks", nil, token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || strings.Count(rec.Body.String(), `"agent_type"`) != 1 {
		t.Fatalf("list tasks expected one task, got %d %s", rec.Code, rec.Body.String())
	}

	assertAuditAction(t, authStore, "orchestrator.run_created")
	assertAuditAction(t, authStore, "orchestrator.task_created")
	assertAuditAction(t, authStore, "orchestrator.task_started")
	assertAuditAction(t, authStore, "orchestrator.task_completed")
}

func TestInvalidOrchestrationInputsRejected(t *testing.T) {
	authStore := authStoreWithPermissions(t, []string{"orchestrator:manage"})
	router := testRouter(authStore, orchestrator.NewMemoryStore())
	token := login(t, router)

	req := authedRequest(http.MethodPost, "/api/v1/orchestrations", bytes.NewBufferString(`{"workflow_type":"chat_agent","target_type":"exam","target_id":"`+targetID+`"}`), token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "invalid_orchestrator_input") {
		t.Fatalf("invalid workflow expected 400, got %d %s", rec.Code, rec.Body.String())
	}

	run := createRun(t, router, token)
	req = authedRequest(http.MethodPost, "/api/v1/orchestrations/"+run.ID+"/tasks", bytes.NewBufferString(`{"agent_type":"chat_agent","input_ref":{}}`), token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "invalid_orchestrator_input") {
		t.Fatalf("invalid agent expected 400, got %d %s", rec.Code, rec.Body.String())
	}
}

func TestExplicitHumanReviewFlagForcesReview(t *testing.T) {
	authStore := authStoreWithPermissions(t, []string{"orchestrator:manage"})
	router := testRouter(authStore, orchestrator.NewMemoryStore())
	token := login(t, router)
	run := createRun(t, router, token)
	task := createTask(t, router, token, run.ID, `{"agent_type":"subjective_grading_agent","input_ref":{"answer_segment_id":"seg-1"}}`)

	req := authedRequest(http.MethodPost, "/api/v1/agent-tasks/"+task.ID+"/start", nil, token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("start expected 200, got %d %s", rec.Code, rec.Body.String())
	}
	req = authedRequest(http.MethodPost, "/api/v1/agent-tasks/"+task.ID+"/complete", bytes.NewBufferString(`{"confidence":0.95,"requires_human_review":true}`), token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"status":"requires_human_review"`) {
		t.Fatalf("explicit review expected human review, got %d %s", rec.Code, rec.Body.String())
	}
}

func TestAgentTaskFailRetryAndRetryLimit(t *testing.T) {
	authStore := authStoreWithPermissions(t, []string{"orchestrator:manage"})
	router := testRouter(authStore, orchestrator.NewMemoryStore())
	token := login(t, router)
	run := createRun(t, router, token)
	task := createTask(t, router, token, run.ID, `{"agent_type":"evidence_check_agent","input_ref":{"answer_segment_id":"seg-1"},"max_attempts":2}`)

	failed := failTask(t, router, token, task.ID)
	if failed.Status != "failed" {
		t.Fatalf("expected failed task, got %#v", failed)
	}
	req := authedRequest(http.MethodPost, "/api/v1/agent-tasks/"+task.ID+"/retry", nil, token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"status":"queued"`) || !strings.Contains(rec.Body.String(), `"attempt_no":2`) {
		t.Fatalf("retry expected queued second attempt, got %d %s", rec.Code, rec.Body.String())
	}

	failed = failTask(t, router, token, task.ID)
	if failed.AttemptNo != 2 {
		t.Fatalf("expected second failed attempt, got %#v", failed)
	}
	req = authedRequest(http.MethodPost, "/api/v1/agent-tasks/"+task.ID+"/retry", nil, token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "agent_task_retry_exhausted") {
		t.Fatalf("retry limit expected conflict, got %d %s", rec.Code, rec.Body.String())
	}

	assertAuditAction(t, authStore, "orchestrator.task_failed")
	assertAuditAction(t, authStore, "orchestrator.task_retried")
}

func TestCompleteBeforeStartRejected(t *testing.T) {
	authStore := authStoreWithPermissions(t, []string{"orchestrator:manage"})
	router := testRouter(authStore, orchestrator.NewMemoryStore())
	token := login(t, router)
	run := createRun(t, router, token)
	task := createTask(t, router, token, run.ID, `{"agent_type":"fill_blank_agent","input_ref":{}}`)

	req := authedRequest(http.MethodPost, "/api/v1/agent-tasks/"+task.ID+"/complete", bytes.NewBufferString(`{"confidence":0.9}`), token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "invalid_orchestrator_status_transition") {
		t.Fatalf("complete before start expected conflict, got %d %s", rec.Code, rec.Body.String())
	}
}

func TestOrchestratorPermissionDeniedAndUnauthenticated(t *testing.T) {
	router := testRouter(authStoreWithPermissions(t, []string{"system:read"}), orchestrator.NewMemoryStore())
	token := login(t, router)

	req := authedRequest(http.MethodPost, "/api/v1/orchestrations", bytes.NewBufferString(validRunJSON()), token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/v1/orchestrations", bytes.NewBufferString(validRunJSON()))
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d %s", rec.Code, rec.Body.String())
	}
}

func testRouter(authStore *auth.MemoryStore, orchestratorStore orchestrator.Store) http.Handler {
	cfg := config.Config{
		Service: config.ServiceConfig{Name: "test", Environment: "test", ReadinessTimeout: time.Millisecond},
		Auth:    config.AuthConfig{SessionTTL: time.Hour},
	}
	return server.NewMemoryRouter(cfg, logger.New(io.Discard, "error"), nil, files.NewMemoryObjectStorage(), func(stores *server.ApplicationStores) {
		stores.Identity.Auth = authStore
		stores.Capture.Orchestrator = orchestratorStore
	})
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
			Username:    "orchestrator_admin",
			DisplayName: "Orchestrator Admin",
			Status:      "active",
			Roles:       []string{"teacher"},
			Permissions: permissions,
			DataScope:   map[string]any{"scope": "school", "synthetic": true},
		},
		PasswordHash: hash,
	})
	return store
}

func login(t *testing.T, router http.Handler) string {
	t.Helper()
	raw, _ := json.Marshal(map[string]string{"tenant_code": "demo", "username": "orchestrator_admin", "password": "ChangeMe123!"})
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

func createRun(t *testing.T, router http.Handler, token string) orchestrator.Run {
	t.Helper()
	req := authedRequest(http.MethodPost, "/api/v1/orchestrations", bytes.NewBufferString(validRunJSON()), token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create run expected 201, got %d %s", rec.Code, rec.Body.String())
	}
	var response struct {
		Run orchestrator.Run `json:"run"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return response.Run
}

func createTask(t *testing.T, router http.Handler, token string, runID string, raw string) orchestrator.Task {
	t.Helper()
	req := authedRequest(http.MethodPost, "/api/v1/orchestrations/"+runID+"/tasks", bytes.NewBufferString(raw), token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create task expected 201, got %d %s", rec.Code, rec.Body.String())
	}
	var response struct {
		Task orchestrator.Task `json:"task"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return response.Task
}

func failTask(t *testing.T, router http.Handler, token string, taskID string) orchestrator.Task {
	t.Helper()
	req := authedRequest(http.MethodPost, "/api/v1/agent-tasks/"+taskID+"/fail", bytes.NewBufferString(`{"error_message":"worker timeout"}`), token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("fail expected 200, got %d %s", rec.Code, rec.Body.String())
	}
	var response struct {
		Task orchestrator.Task `json:"task"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return response.Task
}

func validRunJSON() string {
	return `{"workflow_type":"grading_pipeline","target_type":"answer_segment","target_id":"` + targetID + `"}`
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
