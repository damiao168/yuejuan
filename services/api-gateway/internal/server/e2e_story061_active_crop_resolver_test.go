package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"edugrade-enterprise/services/api-gateway/internal/files"
	"edugrade-enterprise/services/api-gateway/internal/segment"
	"edugrade-enterprise/services/api-gateway/internal/subjective"
)

const story061ActiveCropPNGBase64 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAusB9Y9Zl1sAAAAASUVORK5CYII="

func TestStory061ActiveCropResolverE2EWithPostgresTestDatabase(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("EDUGRADE_E2E_DATABASE_URL"))
	if dsn == "" {
		t.Skip("EDUGRADE_E2E_DATABASE_URL is not set; skipping STORY-061 active crop resolver PostgreSQL workflow")
	}
	db := e2eOpenPostgresTestDB(t, dsn)
	e2eApplyPostgresMigrations(t, db)
	e2eActivatePostgresDemoUsers(t, db, []string{"tenant_admin"})
	router := e2ePostgresRouter(db)
	suffix := time.Now().UTC().Format("20060102150405.000000000")
	adminToken := e2eLoginWithTenant(t, router, "demo", "tenant_admin", "ChangeMe123!")
	fixture := e2eCreateStory056AcceptanceFixture(t, db, router, adminToken, "story061-"+suffix)

	var studentID string
	if err := db.QueryRowContext(
		context.Background(),
		`SELECT id::text FROM student WHERE tenant_id=$1 AND student_no=$2 AND deleted_at IS NULL`,
		fixture.TenantID,
		"S056-READY-story061-"+suffix,
	).Scan(&studentID); err != nil {
		t.Fatalf("look up STORY-061 student: %v", err)
	}
	var otherTenantID string
	if err := db.QueryRowContext(context.Background(), `SELECT id::text FROM tenant WHERE code='platform' AND deleted_at IS NULL`).Scan(&otherTenantID); err != nil {
		t.Fatalf("look up cross-tenant fixture: %v", err)
	}

	ctx := context.Background()
	var submissionID string
	if err := db.QueryRowContext(ctx, `
INSERT INTO submission (
  tenant_id, exam_id, student_id, candidate_no, source_type, status,
  expected_page_count, actual_page_count, quality_status, quality_issues, collected_by,
  identity_status, identity_evidence
)
VALUES ($1::uuid, $2::uuid, $3::uuid, $4, 'scanner_upload', 'ready_for_ocr', 1, 1, 'passed', '[]'::jsonb, $5::uuid, 'matched', '{}'::jsonb)
RETURNING id::text
`, fixture.TenantID, fixture.ExamID, studentID, "S061-"+suffix, fixture.AdminID).Scan(&submissionID); err != nil {
		t.Fatalf("seed STORY-061 submission: %v", err)
	}

	fileStore := files.NewPostgresStore(db)
	sourceData := []byte("story061-source-" + suffix)
	sourceHash := fmt.Sprintf("%x", sha256.Sum256(sourceData))
	source, err := fileStore.Create(ctx, files.CreateAssetInput{
		TenantID:      fixture.TenantID,
		SchoolID:      fixture.SchoolID,
		ExamID:        fixture.ExamID,
		SubmissionID:  submissionID,
		OwnerType:     "submission_page_original",
		OwnerID:       submissionID,
		OriginalName:  "story061-source-" + suffix + ".png",
		ContentType:   "image/png",
		SizeBytes:     int64(len(sourceData)),
		HashSHA256:    sourceHash,
		StorageBucket: "story061-source",
		StorageKey:    "source/" + suffix + ".png",
		Visibility:    "private",
		UploadedBy:    fixture.AdminID,
	})
	if err != nil {
		t.Fatalf("create STORY-061 source asset: %v", err)
	}

	var submissionPageID string
	if err := db.QueryRowContext(ctx, `
INSERT INTO submission_page (tenant_id, submission_id, file_asset_id, page_no, status, quality_status)
VALUES ($1::uuid, $2::uuid, $3::uuid, 1, 'accepted', 'passed')
RETURNING id::text
`, fixture.TenantID, submissionID, source.ID).Scan(&submissionPageID); err != nil {
		t.Fatalf("seed STORY-061 submission page: %v", err)
	}
	var batchID string
	if err := db.QueryRowContext(ctx, `
INSERT INTO capture_batch (tenant_id, exam_id, name, source_type, status, operator_id)
VALUES ($1::uuid, $2::uuid, $3, 'scanner_upload', 'processing', $4::uuid)
RETURNING id::text
`, fixture.TenantID, fixture.ExamID, "STORY-061 "+suffix, fixture.AdminID).Scan(&batchID); err != nil {
		t.Fatalf("seed STORY-061 capture batch: %v", err)
	}
	var captureFileID string
	if err := db.QueryRowContext(ctx, `
INSERT INTO capture_file (
  tenant_id, capture_batch_id, file_asset_id, original_name, content_type, sha256,
  byte_size, page_count, status, idempotency_key, uploaded_by
)
VALUES ($1::uuid, $2::uuid, $3::uuid, $4, 'image/png', $5, $6, 1, 'completed', $7, $8::uuid)
RETURNING id::text
`, fixture.TenantID, batchID, source.ID, source.OriginalName, sourceHash, len(sourceData), "story061-"+suffix, fixture.AdminID).Scan(&captureFileID); err != nil {
		t.Fatalf("seed STORY-061 capture file: %v", err)
	}
	var capturePageID string
	if err := db.QueryRowContext(ctx, `
INSERT INTO capture_page (
  tenant_id, capture_batch_id, capture_file_id, source_index, submission_id,
  submission_page_id, assigned_page_no, sequence_no, decoded_file_asset_id, status
)
VALUES ($1::uuid, $2::uuid, $3::uuid, 1, $4::uuid, $5::uuid, 1, 1, $6::uuid, 'ready')
RETURNING id::text
`, fixture.TenantID, batchID, captureFileID, submissionID, submissionPageID, source.ID).Scan(&capturePageID); err != nil {
		t.Fatalf("seed STORY-061 capture page: %v", err)
	}
	var registrationID string
	if err := db.QueryRowContext(ctx, `
INSERT INTO page_registration_run (
  tenant_id, capture_page_id, submission_page_id, source_file_asset_id, source_sha256,
  template_id, template_content_hash, page_no, processing_status, match_status,
  confidence, method, profile_version, completed_at
)
VALUES (
  $1::uuid, $2::uuid, $3::uuid, $4::uuid, $5,
  $6::uuid, $7, 1, 'completed', 'matched',
  1, 'feature_homography', 'story061-fixture-v1', now()
)
RETURNING id::text
`, fixture.TenantID, capturePageID, submissionPageID, source.ID, sourceHash, fixture.TemplateID, fixture.TemplateContentHash).Scan(&registrationID); err != nil {
		t.Fatalf("seed STORY-061 registration: %v", err)
	}

	cropData, err := base64.StdEncoding.DecodeString(story061ActiveCropPNGBase64)
	if err != nil {
		t.Fatal(err)
	}
	cropHash := fmt.Sprintf("%x", sha256.Sum256(cropData))
	crop, err := fileStore.Create(ctx, files.CreateAssetInput{
		TenantID:      fixture.TenantID,
		SchoolID:      fixture.SchoolID,
		ExamID:        fixture.ExamID,
		SubmissionID:  submissionID,
		OwnerType:     "answer_segment_crop",
		OwnerID:       registrationID,
		OriginalName:  "story061-crop-" + suffix + ".png",
		ContentType:   "image/png",
		SizeBytes:     int64(len(cropData)),
		HashSHA256:    cropHash,
		StorageBucket: "story061-private-crops",
		StorageKey:    "crops/" + suffix + ".png",
		Visibility:    "private",
		UploadedBy:    fixture.AdminID,
	})
	if err != nil {
		t.Fatalf("create STORY-061 crop asset: %v", err)
	}
	var segmentID string
	if err := db.QueryRowContext(ctx, `
INSERT INTO answer_segment (
  tenant_id, submission_id, submission_page_id, question_id, question_no, bbox, source, status,
  template_id, template_content_hash, registration_run_id, normalized_bbox, pixel_bbox,
  crop_file_asset_id, crop_sha256, question_version, processing_status, confidence
)
VALUES (
  $1::uuid, $2::uuid, $3::uuid, $4::uuid, 'Q1',
  '{"x":0.1,"y":0.2,"width":0.4,"height":0.3}'::jsonb, 'configured_answer_area', 'accepted',
  $5::uuid, $6, $7::uuid,
  '{"x":0.1,"y":0.2,"width":0.4,"height":0.3}'::jsonb,
  '{"x":100,"y":200,"width":400,"height":300}'::jsonb,
  $8::uuid, $9, 1, 'completed', 1
)
RETURNING id::text
`, fixture.TenantID, submissionID, submissionPageID, fixture.QuestionIDs["single_choice"], fixture.TemplateID, fixture.TemplateContentHash, registrationID, crop.ID, cropHash).Scan(&segmentID); err != nil {
		t.Fatalf("seed STORY-061 answer segment: %v", err)
	}

	objects := files.NewMemoryObjectStorage()
	if err := objects.Put(ctx, crop.StorageBucket, crop.StorageKey, bytes.NewReader(cropData), int64(len(cropData)), "image/png"); err != nil {
		t.Fatal(err)
	}
	resolver := subjective.NewActiveCropResolver(segment.NewPostgresStore(db), fileStore, objects)
	resolved, err := resolver.Resolve(ctx, fixture.TenantID, segmentID, fixture.QuestionIDs["single_choice"])
	if err != nil {
		t.Fatalf("resolve current PostgreSQL crop: %v", err)
	}
	if resolved.SHA256 != cropHash || !bytes.Equal(resolved.Data, cropData) {
		t.Fatalf("resolved crop does not match PostgreSQL evidence: %#v", resolved)
	}

	for _, scope := range []struct {
		name       string
		tenantID   string
		questionID string
	}{
		{name: "cross tenant", tenantID: otherTenantID, questionID: fixture.QuestionIDs["single_choice"]},
		{name: "cross question", tenantID: fixture.TenantID, questionID: fixture.QuestionIDs["true_false"]},
	} {
		t.Run(scope.name, func(t *testing.T) {
			if _, err := resolver.Resolve(ctx, scope.tenantID, segmentID, scope.questionID); err != subjective.ErrActiveCropUnavailable {
				t.Fatalf("scope bypass unexpectedly resolved: %v", err)
			}
		})
	}

	if _, err := db.ExecContext(
		ctx,
		`UPDATE file_asset SET owner_id=gen_random_uuid(), updated_at=now() WHERE tenant_id=$1 AND id=$2::uuid`,
		fixture.TenantID,
		crop.ID,
	); err != nil {
		t.Fatalf("mutate crop owner for negative case: %v", err)
	}
	if _, err := resolver.Resolve(ctx, fixture.TenantID, segmentID, fixture.QuestionIDs["single_choice"]); err != subjective.ErrActiveCropUnavailable {
		t.Fatalf("asset owner bypass unexpectedly resolved: %v", err)
	}
}
