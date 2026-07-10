package org_test

import (
	"bytes"
	"context"
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

func TestCreateAndListSchoolUsesTenantScope(t *testing.T) {
	authStore := authStoreWithPermissions(t, []string{"org:manage", "student:import"})
	orgStore := org.NewMemoryStore()
	router := testRouter(authStore, orgStore)
	token := login(t, router)

	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/schools", bytes.NewBufferString(`{"name":"第一中学","code":"school-001"}`))
	createReq.Header.Set("Authorization", "Bearer "+token)
	createRec := httptest.NewRecorder()
	router.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create school expected 201, got %d: %s", createRec.Code, createRec.Body.String())
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/schools", nil)
	listReq.Header.Set("Authorization", "Bearer "+token)
	listRec := httptest.NewRecorder()
	router.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list school expected 200, got %d: %s", listRec.Code, listRec.Body.String())
	}
	if !strings.Contains(listRec.Body.String(), "school-001") {
		t.Fatalf("expected created school in list: %s", listRec.Body.String())
	}
}

func TestMemoryStoreTenantIsolation(t *testing.T) {
	store := org.NewMemoryStore()
	if _, err := store.CreateSchool(context.Background(), "tenant-a", org.School{Name: "A", Code: "a"}); err != nil {
		t.Fatalf("create school a: %v", err)
	}
	if _, err := store.CreateSchool(context.Background(), "tenant-b", org.School{Name: "B", Code: "b"}); err != nil {
		t.Fatalf("create school b: %v", err)
	}

	aSchools, err := store.ListSchools(context.Background(), "tenant-a")
	if err != nil {
		t.Fatalf("list tenant a: %v", err)
	}
	if len(aSchools) != 1 || aSchools[0].Code != "a" {
		t.Fatalf("tenant isolation failed: %#v", aSchools)
	}
}

func TestCSVImportReportsRowErrors(t *testing.T) {
	authStore := authStoreWithPermissions(t, []string{"org:manage", "student:import"})
	orgStore := org.NewMemoryStore()
	router := testRouter(authStore, orgStore)
	token := login(t, router)

	body := "student_no,name,school_id,class_id\nS001,张三,school-1,class-1\nS002,,school-1,class-1\nbad,row\n"
	req := httptest.NewRequest(http.MethodPost, "/api/v1/students/import-csv", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("import expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"created":1`) || !strings.Contains(rec.Body.String(), `"errors"`) {
		t.Fatalf("expected created count and errors: %s", rec.Body.String())
	}
}

func TestOrganizationPermissionDenied(t *testing.T) {
	authStore := authStoreWithPermissions(t, []string{"system:read"})
	router := testRouter(authStore, org.NewMemoryStore())
	token := login(t, router)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/schools", bytes.NewBufferString(`{"name":"第一中学","code":"school-001"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestGraderCannotListStudentNamesWithoutOrgManage(t *testing.T) {
	authStore := authStoreWithPermissions(t, []string{"system:read", "review:manage", "grading:manage"})
	router := testRouter(authStore, org.NewMemoryStore())
	token := login(t, router)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/students", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("grader without org:manage should not list students, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestNonPlatformTenantCannotAdministerOtherTenants(t *testing.T) {
	authStore := authStoreWithPermissions(t, []string{"tenant:manage"})
	router := testRouter(authStore, org.NewMemoryStore())
	token := login(t, router)

	for _, request := range []*http.Request{
		httptest.NewRequest(http.MethodPost, "/api/v1/tenants", bytes.NewBufferString(`{"name":"Other Tenant","code":"other"}`)),
		httptest.NewRequest(http.MethodPatch, "/api/v1/tenants/tenant-other", bytes.NewBufferString(`{"status":"disabled"}`)),
	} {
		request.Header.Set("Authorization", "Bearer "+token)
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusForbidden || !strings.Contains(recorder.Body.String(), "platform_tenant_required") {
			t.Fatalf("non-platform tenant administration expected 403, got %d %s", recorder.Code, recorder.Body.String())
		}
	}
}

func TestPlatformTenantCanCreateTenant(t *testing.T) {
	store := org.NewMemoryStore()
	handler := org.NewHandler(store, auth.NewMemoryStore())
	req := httptest.NewRequest(http.MethodPost, "/api/v1/tenants", bytes.NewBufferString(`{"name":"Managed Tenant","code":"managed"}`))
	req = req.WithContext(auth.WithUser(req.Context(), auth.User{ID: "platform-admin", TenantID: auth.PlatformTenantID}))
	rec := httptest.NewRecorder()

	handler.CreateTenant(rec, req)

	if rec.Code != http.StatusCreated || !strings.Contains(rec.Body.String(), `"code":"managed"`) {
		t.Fatalf("platform tenant create expected 201, got %d %s", rec.Code, rec.Body.String())
	}
}

func testRouter(authStore *auth.MemoryStore, orgStore *org.MemoryStore) http.Handler {
	cfg := config.Config{
		Service: config.ServiceConfig{Name: "test", Environment: "test", ReadinessTimeout: time.Millisecond},
		Auth:    config.AuthConfig{SessionTTL: time.Hour},
	}
	return server.NewRouter(cfg, logger.New(io.Discard, "error"), nil, authStore, orgStore, exam.NewMemoryStore(), paper.NewMemoryStore())
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
			ID:          "u-org",
			TenantID:    "tenant-org",
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
		t.Fatalf("login expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var response struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return response.AccessToken
}
