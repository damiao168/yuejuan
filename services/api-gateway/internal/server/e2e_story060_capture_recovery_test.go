package server

import (
	"context"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"edugrade-enterprise/services/api-gateway/internal/capture"
	"edugrade-enterprise/services/api-gateway/internal/files"
)

func TestStory060CaptureRecoveryE2EWithPostgresTestDatabase(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("EDUGRADE_E2E_DATABASE_URL"))
	if dsn == "" {
		t.Skip("EDUGRADE_E2E_DATABASE_URL is not set; skipping STORY-060 capture recovery workflow")
	}
	db := e2eOpenPostgresTestDB(t, dsn)
	e2eApplyPostgresMigrations(t, db)
	e2eActivatePostgresDemoUsers(t, db, []string{"tenant_admin"})
	router := e2ePostgresRouter(db)
	adminToken := e2eLoginWithTenant(t, router, "demo", "tenant_admin", "ChangeMe123!")
	suffix := strings.ReplaceAll(time.Now().UTC().Format("20060102150405.000000000"), ".", "")
	fixture := e2eCreateStory056AcceptanceFixture(t, db, router, adminToken, suffix)

	sourceAssetID := e2eUploadSyntheticPDF(
		t,
		router,
		adminToken,
		"story060-recovery-"+suffix+".pdf",
		"%PDF-1.4\n% STORY-060 capture recovery "+suffix+"\n",
	)
	batch := e2ePostJSON(t, router, http.MethodPost, "/api/v1/exams/"+fixture.ExamID+"/capture-batches", adminToken, story056JSON(t, map[string]any{
		"name":            "STORY-060 采集自救验收",
		"source_type":     "web_upload",
		"idempotency_key": "story060-recovery-batch-" + suffix,
	}), http.StatusCreated)["batch"].(map[string]any)
	batchID := e2eString(t, batch, "id")
	first := e2ePostJSON(t, router, http.MethodPost, "/api/v1/capture-batches/"+batchID+"/files", adminToken, story056JSON(t, map[string]any{
		"file_asset_id":   sourceAssetID,
		"idempotency_key": "story060-recovery-first-" + suffix,
	}), http.StatusCreated)["file"].(map[string]any)
	firstID := e2eString(t, first, "id")
	e2ePostJSON(t, router, http.MethodPost, "/api/v1/capture-batches/"+batchID+"/process", adminToken, `{}`, http.StatusOK)

	if _, err := db.Exec(`
UPDATE agent_worker_task
SET status='failed',attempt_count=max_attempts,completed_at=now(),error_code='pdf_decode_failed',updated_at=now()
WHERE tenant_id=$1::uuid AND source_type='capture_file' AND source_id=$2::uuid
`, fixture.TenantID, firstID); err != nil {
		t.Fatalf("seed terminal worker failure: %v", err)
	}
	if _, err := db.Exec(`
UPDATE capture_file
SET status='failed',error_code='pdf_decode_failed',updated_at=now()
WHERE tenant_id=$1::uuid AND id=$2::uuid
`, fixture.TenantID, firstID); err != nil {
		t.Fatalf("seed terminal capture file failure: %v", err)
	}
	if _, err := db.Exec(`
UPDATE capture_batch SET status='needs_review',failed_count=1,updated_at=now()
WHERE tenant_id=$1::uuid AND id=$2::uuid
`, fixture.TenantID, batchID); err != nil {
		t.Fatalf("seed capture batch review state: %v", err)
	}

	reuploaded := e2ePostJSON(t, router, http.MethodPost, "/api/v1/capture-batches/"+batchID+"/files", adminToken, story056JSON(t, map[string]any{
		"file_asset_id":   sourceAssetID,
		"idempotency_key": "story060-recovery-second-" + suffix,
	}), http.StatusCreated)["file"].(map[string]any)
	if reuploaded["status"] != "uploaded" {
		t.Fatalf("same hash must be accepted after the prior source failed: %#v", reuploaded)
	}
	reuploadedID := e2eString(t, reuploaded, "id")
	duplicate := e2ePostJSON(t, router, http.MethodPost, "/api/v1/capture-batches/"+batchID+"/files", adminToken, story056JSON(t, map[string]any{
		"file_asset_id":   sourceAssetID,
		"idempotency_key": "story060-recovery-duplicate-" + suffix,
	}), http.StatusCreated)["file"].(map[string]any)
	if duplicate["status"] != "duplicate" {
		t.Fatalf("an active source must continue to protect against duplicates: %#v", duplicate)
	}

	e2ePostJSON(t, router, http.MethodPost, "/api/v1/capture-batches/"+batchID+"/process", adminToken, `{}`, http.StatusOK)
	var firstStatus, firstError, secondStatus, taskStatus string
	var maxAttempts, attemptCount int
	if err := db.QueryRow(`
SELECT f1.status,COALESCE(f1.error_code,''),f2.status,t.status,t.max_attempts,t.attempt_count
FROM capture_file f1
JOIN capture_file f2 ON f2.tenant_id=f1.tenant_id AND f2.id=$3::uuid
JOIN agent_worker_task t ON t.tenant_id=f1.tenant_id AND t.source_type='capture_file' AND t.source_id=f1.id
WHERE f1.tenant_id=$1::uuid AND f1.id=$2::uuid
`, fixture.TenantID, firstID, reuploadedID).Scan(&firstStatus, &firstError, &secondStatus, &taskStatus, &maxAttempts, &attemptCount); err != nil {
		t.Fatalf("query recovered capture state: %v", err)
	}
	if firstStatus != "queued" || firstError != "" || secondStatus != "queued" || taskStatus != "queued" || maxAttempts <= attemptCount {
		t.Fatalf("failed file and runtime task were not safely requeued: first=%s error=%q second=%s task=%s attempts=%d/%d",
			firstStatus, firstError, secondStatus, taskStatus, attemptCount, maxAttempts)
	}

	var sourceHash string
	if err := db.QueryRow(`SELECT hash_sha256 FROM file_asset WHERE tenant_id=$1::uuid AND id=$2::uuid`, fixture.TenantID, sourceAssetID).Scan(&sourceHash); err != nil {
		t.Fatalf("lookup source hash: %v", err)
	}
	captureStore := capture.NewPostgresStoreWithBarcodeKeyring(db, e2eBarcodeKeyring())
	if _, err := captureStore.ApplyFileResult(context.Background(), fixture.TenantID, reuploadedID, []capture.DecodedPageInput{{
		SourceIndex: 1, FileAssetID: sourceAssetID, SHA256: sourceHash, Width: 1000, Height: 1400,
	}}); err != nil {
		t.Fatalf("materialize recovered capture page: %v", err)
	}
	var capturePageID, submissionPageID, submissionID string
	if err := db.QueryRow(`
SELECT id::text,submission_page_id::text,submission_id::text
FROM capture_page
WHERE tenant_id=$1::uuid AND capture_file_id=$2::uuid AND deleted_at IS NULL
`, fixture.TenantID, reuploadedID).Scan(&capturePageID, &submissionPageID, &submissionID); err != nil {
		t.Fatalf("lookup recovered page: %v", err)
	}
	normalized, err := files.NewPostgresStore(db).Create(context.Background(), files.CreateAssetInput{
		TenantID: fixture.TenantID, ExamID: fixture.ExamID, SubmissionID: submissionID,
		OwnerType: "submission_page_normalized", OwnerID: submissionPageID,
		OriginalName: "story060-normalized-" + suffix + ".png", ContentType: "image/png", SizeBytes: 1,
		HashSHA256: "sha256:story060-normalized-" + suffix, StorageBucket: "edugrade-story060-e2e",
		StorageKey: "normalized/" + suffix + ".png", Visibility: "private", UploadedBy: fixture.AdminID,
	})
	if err != nil {
		t.Fatalf("create normalized quality evidence: %v", err)
	}
	if _, err = db.Exec(`
UPDATE submission_page
SET normalized_file_asset_id=$3::uuid,quality_status='failed',
    quality_issues='[{"code":"blur","message":"synthetic threshold failure"}]'::jsonb,
    updated_at=now()
WHERE tenant_id=$1::uuid AND id=$2::uuid
`, fixture.TenantID, submissionPageID, normalized.ID); err != nil {
		t.Fatalf("seed submission page quality false positive: %v", err)
	}
	if _, err = db.Exec(`
UPDATE submission
SET quality_status='failed',status='pages_uploaded',updated_at=now()
WHERE tenant_id=$1::uuid AND id=$2::uuid
`, fixture.TenantID, submissionID); err != nil {
		t.Fatalf("seed submission quality false positive: %v", err)
	}
	if _, err = db.Exec(`
UPDATE capture_page
SET status='quality_rejected',page_identity=page_identity || '{"quality_status":"failed"}'::jsonb,updated_at=now()
WHERE tenant_id=$1::uuid AND id=$2::uuid
`, fixture.TenantID, capturePageID); err != nil {
		t.Fatalf("seed capture page quality false positive: %v", err)
	}

	override := e2ePostJSON(t, router, http.MethodPost, "/api/v1/submission-pages/"+submissionPageID+"/quality-override", adminToken, story056JSON(t, map[string]any{
		"reason": "人工核对页面清晰，倾斜与模糊程度不影响阅卷",
	}), http.StatusOK)
	runs := override["registration_runs"].([]any)
	if len(runs) == 0 {
		t.Fatalf("quality override must restore page registration work: %#v", override)
	}
	var pageQualityStatus, pageStatus, decision, reason, actorID, originalStatus, registrationTaskStatus string
	if err := db.QueryRow(`
SELECT sp.quality_status,cp.status,
       sp.quality_override->>'decision',sp.quality_override->>'reason',
       sp.quality_override->>'actor_id',sp.quality_override->>'original_quality_status',
       t.status
FROM submission_page sp
JOIN capture_page cp ON cp.tenant_id=sp.tenant_id AND cp.submission_page_id=sp.id
JOIN page_registration_run r ON r.tenant_id=cp.tenant_id AND r.capture_page_id=cp.id
JOIN agent_worker_task t ON t.tenant_id=r.tenant_id AND t.id=r.runtime_task_id
WHERE sp.tenant_id=$1::uuid AND sp.id=$2::uuid
ORDER BY r.created_at DESC LIMIT 1
`, fixture.TenantID, submissionPageID).Scan(
		&pageQualityStatus, &pageStatus, &decision, &reason, &actorID, &originalStatus, &registrationTaskStatus,
	); err != nil {
		t.Fatalf("query quality override recovery evidence: %v", err)
	}
	if pageQualityStatus != "passed" || pageStatus != "registration" || decision != "accepted" ||
		reason == "" || actorID != fixture.AdminID || originalStatus != "failed" || registrationTaskStatus != "queued" {
		t.Fatalf("quality override evidence or recovery task is incomplete: quality=%s page=%s decision=%s reason=%q actor=%s original=%s task=%s",
			pageQualityStatus, pageStatus, decision, reason, actorID, originalStatus, registrationTaskStatus)
	}
}
