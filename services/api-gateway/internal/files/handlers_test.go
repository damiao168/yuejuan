package files_test

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"testing"
	"time"

	"edugrade-enterprise/services/api-gateway/internal/auth"
	"edugrade-enterprise/services/api-gateway/internal/config"
	"edugrade-enterprise/services/api-gateway/internal/exam"
	"edugrade-enterprise/services/api-gateway/internal/files"
	"edugrade-enterprise/services/api-gateway/internal/logger"
	"edugrade-enterprise/services/api-gateway/internal/org"
	"edugrade-enterprise/services/api-gateway/internal/paper"
	"edugrade-enterprise/services/api-gateway/internal/server"
)

func TestUploadGetDownloadAndDeleteFile(t *testing.T) {
	authStore := authStoreWithPermissions(t, []string{"file:manage"})
	router := testRouter(authStore, files.NewMemoryStore(), files.NewMemoryObjectStorage())
	token := login(t, router)

	asset := uploadFile(t, router, token, "paper.pdf", "application/pdf", []byte("%PDF-1.4\nsynthetic pdf\n"), map[string]string{
		"owner_type": "exam",
		"owner_id":   "00000000-0000-0000-0000-000000000101",
		"exam_id":    "00000000-0000-0000-0000-000000000101",
	})
	if asset.OriginalName != "paper.pdf" || asset.ContentType != "application/pdf" {
		t.Fatalf("unexpected uploaded asset: %#v", asset)
	}

	req := authedRequest(http.MethodGet, "/api/v1/files/"+asset.ID, nil, token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("metadata expected 200, got %d %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "storage_key") || strings.Contains(rec.Body.String(), "storage_bucket") {
		t.Fatalf("metadata response must not expose object storage path: %s", rec.Body.String())
	}

	req = authedRequest(http.MethodGet, "/api/v1/files/"+asset.ID+"/download", nil, token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("download expected 200, got %d %s", rec.Code, rec.Body.String())
	}
	if rec.Body.String() != "%PDF-1.4\nsynthetic pdf\n" {
		t.Fatalf("download returned wrong content: %q", rec.Body.String())
	}

	req = authedRequest(http.MethodDelete, "/api/v1/files/"+asset.ID, nil, token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete expected 200, got %d %s", rec.Code, rec.Body.String())
	}

	req = authedRequest(http.MethodGet, "/api/v1/files/"+asset.ID, nil, token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("deleted metadata expected 404, got %d %s", rec.Code, rec.Body.String())
	}

	assertAuditAction(t, authStore, "file.uploaded")
	assertAuditAction(t, authStore, "file.downloaded")
	assertAuditAction(t, authStore, "file.deleted")
}

func TestDuplicateUploadRejected(t *testing.T) {
	authStore := authStoreWithPermissions(t, []string{"file:manage"})
	router := testRouter(authStore, files.NewMemoryStore(), files.NewMemoryObjectStorage())
	token := login(t, router)
	content := []byte("%PDF-1.4\nsame\n")
	fields := map[string]string{"owner_type": "exam", "owner_id": "00000000-0000-0000-0000-000000000101", "exam_id": "00000000-0000-0000-0000-000000000101"}

	_ = uploadFile(t, router, token, "paper.pdf", "application/pdf", content, fields)
	body, contentType := multipartUploadBody(t, "paper.pdf", "application/pdf", content, fields)
	req := authedRequest(http.MethodPost, "/api/v1/files", body, token)
	req.Header.Set("Content-Type", contentType)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "duplicate_file") {
		t.Fatalf("duplicate expected 409, got %d %s", rec.Code, rec.Body.String())
	}
}

func TestUnsupportedFileTypeRejected(t *testing.T) {
	router := testRouter(authStoreWithPermissions(t, []string{"file:manage"}), files.NewMemoryStore(), files.NewMemoryObjectStorage())
	token := login(t, router)
	body, contentType := multipartUploadBody(t, "tool.exe", "application/octet-stream", []byte("MZ synthetic"), nil)
	req := authedRequest(http.MethodPost, "/api/v1/files", body, token)
	req.Header.Set("Content-Type", contentType)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unsupported type expected 400, got %d %s", rec.Code, rec.Body.String())
	}
}

func TestInvalidOwnerIDRejected(t *testing.T) {
	router := testRouter(authStoreWithPermissions(t, []string{"file:manage"}), files.NewMemoryStore(), files.NewMemoryObjectStorage())
	token := login(t, router)
	body, contentType := multipartUploadBody(t, "paper.pdf", "application/pdf", []byte("%PDF-1.4\nsynthetic pdf\n"), map[string]string{
		"owner_type": "exam",
		"owner_id":   "exam-1",
	})
	req := authedRequest(http.MethodPost, "/api/v1/files", body, token)
	req.Header.Set("Content-Type", contentType)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "invalid_owner_id") {
		t.Fatalf("invalid owner id expected 400, got %d %s", rec.Code, rec.Body.String())
	}
}

func TestInconsistentOwnerReferenceRejected(t *testing.T) {
	router := testRouter(authStoreWithPermissions(t, []string{"file:manage"}), files.NewMemoryStore(), files.NewMemoryObjectStorage())
	token := login(t, router)
	body, contentType := multipartUploadBody(t, "paper.pdf", "application/pdf", []byte("%PDF-1.4\nsynthetic pdf\n"), map[string]string{
		"owner_type": "exam",
		"owner_id":   "00000000-0000-0000-0000-000000000101",
		"exam_id":    "00000000-0000-0000-0000-000000000102",
	})
	req := authedRequest(http.MethodPost, "/api/v1/files", body, token)
	req.Header.Set("Content-Type", contentType)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "invalid_owner_reference") {
		t.Fatalf("inconsistent owner reference expected 400, got %d %s", rec.Code, rec.Body.String())
	}
}

func TestNormalizedSubmissionPageOwnerTypeAccepted(t *testing.T) {
	router := testRouter(authStoreWithPermissions(t, []string{"file:manage"}), files.NewMemoryStore(), files.NewMemoryObjectStorage())
	token := login(t, router)
	asset := uploadFile(t, router, token, "normalized.png", "image/png", []byte("\x89PNG\r\n\x1a\nsynthetic"), map[string]string{
		"owner_type":    "submission_page_normalized",
		"owner_id":      "00000000-0000-0000-0000-000000000301",
		"submission_id": "00000000-0000-0000-0000-000000000302",
	})
	if asset.OwnerType != "submission_page_normalized" {
		t.Fatalf("normalized page owner type mismatch: %#v", asset)
	}
}

func TestOversizedFileRejected(t *testing.T) {
	router := testRouter(authStoreWithPermissions(t, []string{"file:manage"}), files.NewMemoryStore(), files.NewMemoryObjectStorage())
	token := login(t, router)
	content := append([]byte("%PDF-1.4\n"), bytes.Repeat([]byte("x"), 1024*1024)...)
	body, contentType := multipartUploadBody(t, "large.pdf", "application/pdf", content, nil)
	req := authedRequest(http.MethodPost, "/api/v1/files", body, token)
	req.Header.Set("Content-Type", contentType)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("oversized file expected 400, got %d %s", rec.Code, rec.Body.String())
	}
}

func TestPathFilenameIsSanitized(t *testing.T) {
	router := testRouter(authStoreWithPermissions(t, []string{"file:manage"}), files.NewMemoryStore(), files.NewMemoryObjectStorage())
	token := login(t, router)
	asset := uploadFile(t, router, token, `..\unsafe\paper.pdf`, "application/pdf", []byte("%PDF-1.4\nsynthetic pdf\n"), nil)
	if asset.OriginalName != "paper.pdf" {
		t.Fatalf("expected sanitized filename, got %s", asset.OriginalName)
	}
}

func TestMaliciousFilenamesRejected(t *testing.T) {
	for _, name := range []string{"CON.pdf", "answer.pdf:evil.exe", ".."} {
		if cleaned, err := files.CleanFilename(name); err == nil {
			t.Fatalf("expected %q to be rejected, got %q", name, cleaned)
		}
	}
}

func TestFilePermissionDenied(t *testing.T) {
	router := testRouter(authStoreWithPermissions(t, []string{"system:read"}), files.NewMemoryStore(), files.NewMemoryObjectStorage())
	token := login(t, router)
	body, contentType := multipartUploadBody(t, "paper.pdf", "application/pdf", []byte("%PDF-1.4\nsynthetic pdf\n"), nil)
	req := authedRequest(http.MethodPost, "/api/v1/files", body, token)
	req.Header.Set("Content-Type", contentType)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("permission denied expected 403, got %d %s", rec.Code, rec.Body.String())
	}
}

func testRouter(authStore *auth.MemoryStore, fileStore files.Store, objectStore files.ObjectStorage) http.Handler {
	cfg := config.Config{
		Service: config.ServiceConfig{Name: "test", Environment: "test", ReadinessTimeout: time.Millisecond},
		Auth:    config.AuthConfig{SessionTTL: time.Hour},
		Files: config.FileConfig{
			Bucket:            "test-files",
			MaxUploadBytes:    1024 * 1024,
			AllowedExtensions: []string{".pdf", ".png", ".jpg", ".jpeg", ".csv", ".docx"},
		},
	}
	return server.NewRouterWithFiles(cfg, logger.New(io.Discard, "error"), nil, authStore, org.NewMemoryStore(), exam.NewMemoryStore(), paper.NewMemoryStore(), fileStore, objectStore)
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
			ID:          "00000000-0000-0000-0000-000000000201",
			TenantID:    "00000000-0000-0000-0000-000000000002",
			TenantCode:  "demo",
			Username:    "file_admin",
			DisplayName: "File Admin",
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
	raw, _ := json.Marshal(map[string]string{"tenant_code": "demo", "username": "file_admin", "password": "ChangeMe123!"})
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

func uploadFile(t *testing.T, router http.Handler, token string, filename string, fileContentType string, content []byte, fields map[string]string) files.FileResponse {
	t.Helper()
	body, contentType := multipartUploadBody(t, filename, fileContentType, content, fields)
	req := authedRequest(http.MethodPost, "/api/v1/files", body, token)
	req.Header.Set("Content-Type", contentType)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("upload expected 201, got %d %s", rec.Code, rec.Body.String())
	}
	var response struct {
		File files.FileResponse `json:"file"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return response.File
}

func multipartUploadBody(t *testing.T, filename string, fileContentType string, content []byte, fields map[string]string) (*bytes.Buffer, string) {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	for key, value := range fields {
		if err := writer.WriteField(key, value); err != nil {
			t.Fatalf("write field: %v", err)
		}
	}
	header := textproto.MIMEHeader{}
	header.Set("Content-Disposition", `form-data; name="file"; filename="`+filename+`"`)
	header.Set("Content-Type", fileContentType)
	part, err := writer.CreatePart(header)
	if err != nil {
		t.Fatalf("create part: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("write part: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart: %v", err)
	}
	return body, writer.FormDataContentType()
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
