package segment_test

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
	"edugrade-enterprise/services/api-gateway/internal/files"
	"edugrade-enterprise/services/api-gateway/internal/logger"
	ocrpkg "edugrade-enterprise/services/api-gateway/internal/ocr"
	"edugrade-enterprise/services/api-gateway/internal/org"
	"edugrade-enterprise/services/api-gateway/internal/paper"
	"edugrade-enterprise/services/api-gateway/internal/segment"
	"edugrade-enterprise/services/api-gateway/internal/server"
	"edugrade-enterprise/services/api-gateway/internal/submission"
)

const tenantID = "00000000-0000-0000-0000-000000000002"
const userID = "00000000-0000-0000-0000-000000000501"

func TestGenerateSegmentsIdempotentAndManualUpdate(t *testing.T) {
	authStore := authStoreWithPermissions(t, []string{"segment:manage"})
	submissionStore := submission.NewMemoryStore()
	item := readySubmission(t, submissionStore)
	paperStore := paper.NewMemoryStore()
	createQuestion(t, paperStore, item.ExamID, "Q1", 1, map[string]any{"page": 1.0, "x": 10.0, "y": 20.0, "w": 100.0, "h": 40.0})
	segmentStore := segment.NewMemoryStore()
	router := testRouter(authStore, paperStore, submissionStore, segmentStore)
	token := login(t, router)

	result := generateSegments(t, router, token, item.ID)
	if !result.Valid || len(result.Segments) != 1 {
		t.Fatalf("expected one valid segment, got %#v", result)
	}
	result = generateSegments(t, router, token, item.ID)
	if len(result.Segments) != 1 {
		t.Fatalf("repeat generate must stay idempotent, got %#v", result)
	}

	req := authedRequest(http.MethodPatch, "/api/v1/answer-segments/"+result.Segments[0].ID, bytes.NewBufferString(`{"bbox":[11,21,101,41],"status":"accepted","review_notes":"aligned manually"}`), token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"status":"accepted"`) || !strings.Contains(rec.Body.String(), `"source":"manual"`) {
		t.Fatalf("update expected accepted manual segment, got %d %s", rec.Code, rec.Body.String())
	}

	req = authedRequest(http.MethodGet, "/api/v1/submissions/"+item.ID+"/answer-segments", nil, token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || strings.Count(rec.Body.String(), `"question_no"`) != 1 {
		t.Fatalf("list expected one segment, got %d %s", rec.Code, rec.Body.String())
	}

	assertAuditAction(t, authStore, "segment.generated")
	assertAuditAction(t, authStore, "segment.updated")
}

func TestGenerateSegmentsReportsMissingPageAndAnswerArea(t *testing.T) {
	authStore := authStoreWithPermissions(t, []string{"segment:manage"})
	submissionStore := submission.NewMemoryStore()
	item := readySubmission(t, submissionStore)
	paperStore := paper.NewMemoryStore()
	createQuestion(t, paperStore, item.ExamID, "Q1", 1, map[string]any{"page": 2.0, "x": 10.0, "y": 20.0, "w": 100.0, "h": 40.0})
	createQuestion(t, paperStore, item.ExamID, "Q2", 2, nil)
	router := testRouter(authStore, paperStore, submissionStore, segment.NewMemoryStore())
	token := login(t, router)

	result := generateSegments(t, router, token, item.ID)
	if result.Valid || len(result.Issues) != 2 {
		t.Fatalf("expected two generation issues, got %#v", result)
	}
	codes := marshal(t, result.Issues)
	if !strings.Contains(codes, "submission_page_missing") || !strings.Contains(codes, "answer_area_missing") {
		t.Fatalf("unexpected issues: %s", codes)
	}
}

func TestLegacySegmentGenerationIsExplicitlyDeprecated(t *testing.T) {
	authStore := authStoreWithPermissions(t, []string{"segment:manage"})
	submissionStore := submission.NewMemoryStore()
	item := readySubmission(t, submissionStore)
	paperStore := paper.NewMemoryStore()
	createQuestion(t, paperStore, item.ExamID, "Q1", 1, map[string]any{"page": 1.0, "x": 10.0, "y": 20.0, "w": 100.0, "h": 40.0})
	router := testRouter(authStore, paperStore, submissionStore, segment.NewMemoryStore())
	token := login(t, router)

	req := authedRequest(http.MethodPost, "/api/v1/submissions/"+item.ID+"/segment-answers", nil, token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("legacy compatibility route expected 200, got %d %s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Deprecation") != "true" || rec.Header().Get("Sunset") == "" ||
		!strings.Contains(rec.Header().Get("Warning"), "deprecated") ||
		!strings.Contains(rec.Header().Get("Link"), "successor-version") {
		t.Fatalf("legacy route must advertise its replacement and sunset: %#v", rec.Header())
	}
}

func TestSegmentationRequiresReadySubmission(t *testing.T) {
	authStore := authStoreWithPermissions(t, []string{"segment:manage"})
	submissionStore := submission.NewMemoryStore()
	item, err := submissionStore.Create(context.Background(), tenantID, "exam-1", userID, submission.CreateSubmissionInput{SourceType: "scanner_upload", ExpectedPageCount: 1})
	if err != nil {
		t.Fatalf("create submission: %v", err)
	}
	router := testRouter(authStore, paper.NewMemoryStore(), submissionStore, segment.NewMemoryStore())
	token := login(t, router)
	req := authedRequest(http.MethodPost, "/api/v1/submissions/"+item.ID+"/segment-answers", nil, token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "submission_not_ready_for_segmentation") {
		t.Fatalf("expected not ready conflict, got %d %s", rec.Code, rec.Body.String())
	}
}

func TestSegmentPermissionDenied(t *testing.T) {
	authStore := authStoreWithPermissions(t, []string{"system:read"})
	submissionStore := submission.NewMemoryStore()
	item := readySubmission(t, submissionStore)
	router := testRouter(authStore, paper.NewMemoryStore(), submissionStore, segment.NewMemoryStore())
	token := login(t, router)
	req := authedRequest(http.MethodPost, "/api/v1/submissions/"+item.ID+"/segment-answers", nil, token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d %s", rec.Code, rec.Body.String())
	}
}

func TestSegmentImageRequiresCacheRevalidation(t *testing.T) {
	fileStore := files.NewMemoryStore()
	asset, err := fileStore.Create(context.Background(), files.CreateAssetInput{
		TenantID:      tenantID,
		ExamID:        "exam-1",
		OwnerType:     "answer_segment_crop",
		OwnerID:       "registration-1",
		OriginalName:  "q1-crop.png",
		ContentType:   "image/png",
		SizeBytes:     3,
		HashSHA256:    "crop-hash",
		StorageBucket: "answers",
		StorageKey:    "q1-crop.png",
	})
	if err != nil {
		t.Fatalf("create crop asset: %v", err)
	}
	store := evidenceStore{evidence: segment.SegmentEvidence{
		SegmentID:          "segment-1",
		ExamID:             "exam-1",
		RegistrationRunID:  "registration-1",
		CropFileAssetID:    asset.ID,
		CropSHA256:         "crop-hash",
		ProcessingStatus:   "completed",
		RegistrationStatus: "completed",
	}}
	handler := segment.NewHandler(store, nil, nil, auth.NewMemoryStore(), fileStore, files.NewMemoryObjectStorage())
	req := httptest.NewRequest(http.MethodHead, "/api/v1/answer-segments/segment-1/image", nil)
	req.SetPathValue("id", "segment-1")
	req = req.WithContext(auth.WithUser(req.Context(), auth.User{ID: userID, TenantID: tenantID}))
	rec := httptest.NewRecorder()

	handler.GetImage(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Cache-Control"); got != "private, no-cache" {
		t.Fatalf("Cache-Control = %q, want private revalidation", got)
	}
}

type evidenceStore struct {
	evidence segment.SegmentEvidence
}

func (s evidenceStore) CreateSegments(context.Context, []segment.CreateSegmentInput) ([]segment.Segment, error) {
	return nil, segment.ErrNotFound
}

func (s evidenceStore) ListBySubmission(context.Context, string, string) ([]segment.Segment, error) {
	return nil, segment.ErrNotFound
}

func (s evidenceStore) Update(context.Context, string, string, string, segment.UpdateSegmentInput) (segment.Segment, error) {
	return segment.Segment{}, segment.ErrNotFound
}

func (s evidenceStore) GetEvidence(_ context.Context, tenantID string, id string) (segment.SegmentEvidence, error) {
	if tenantID != s.evidenceTenantID() || id != s.evidence.SegmentID {
		return segment.SegmentEvidence{}, segment.ErrNotFound
	}
	return s.evidence, nil
}

func (s evidenceStore) evidenceTenantID() string {
	return tenantID
}

func testRouter(authStore *auth.MemoryStore, paperStore paper.Store, submissionStore submission.Store, segmentStore segment.Store) http.Handler {
	cfg := config.Config{
		Service: config.ServiceConfig{Name: "test", Environment: "test", ReadinessTimeout: time.Millisecond},
		Auth:    config.AuthConfig{SessionTTL: time.Hour},
	}
	return server.NewRouterComplete(cfg, logger.New(io.Discard, "error"), nil, authStore, org.NewMemoryStore(), exam.NewMemoryStore(), paperStore, files.NewMemoryStore(), files.NewMemoryObjectStorage(), submissionStore, ocrpkg.NewMemoryStore(), ocrpkg.NewMemoryQueue(), segmentStore)
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
			Username:    "segment_admin",
			DisplayName: "Segment Admin",
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
	raw, _ := json.Marshal(map[string]string{"tenant_code": "demo", "username": "segment_admin", "password": "ChangeMe123!"})
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

func readySubmission(t *testing.T, store submission.Store) submission.Submission {
	t.Helper()
	ctx := context.Background()
	item, err := store.Create(ctx, tenantID, "exam-1", userID, submission.CreateSubmissionInput{SourceType: "scanner_upload", ExpectedPageCount: 1})
	if err != nil {
		t.Fatalf("create submission: %v", err)
	}
	if _, err := store.AddPage(ctx, tenantID, item.ID, userID, submission.AddPageInput{FileAssetID: "file-1", PageNo: 1}); err != nil {
		t.Fatalf("add page: %v", err)
	}
	if _, err := store.RunQualityCheck(ctx, tenantID, item.ID, userID); err != nil {
		t.Fatalf("quality check: %v", err)
	}
	out, err := store.UpdateStatus(ctx, tenantID, item.ID, userID, "ready_for_ocr")
	if err != nil {
		t.Fatalf("ready status: %v", err)
	}
	return out
}

func createQuestion(t *testing.T, store paper.Store, examID string, questionNo string, sortOrder int, area map[string]any) {
	t.Helper()
	_, err := store.CreateQuestion(context.Background(), tenantID, examID, userID, paper.CreateQuestionInput{
		QuestionNo:      questionNo,
		QuestionType:    "short_answer",
		Score:           10,
		KnowledgePoints: []string{"mechanics"},
		AnswerArea:      area,
		SortOrder:       sortOrder,
	})
	if err != nil {
		t.Fatalf("create question: %v", err)
	}
}

func generateSegments(t *testing.T, router http.Handler, token string, submissionID string) segment.GenerateResult {
	t.Helper()
	req := authedRequest(http.MethodPost, "/api/v1/submissions/"+submissionID+"/segment-answers", nil, token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("generate expected 200, got %d %s", rec.Code, rec.Body.String())
	}
	var response struct {
		Result segment.GenerateResult `json:"result"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return response.Result
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

func marshal(t *testing.T, value any) string {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(raw)
}
