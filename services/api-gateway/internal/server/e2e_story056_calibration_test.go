package server

import (
	"database/sql"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"edugrade-enterprise/services/api-gateway/internal/auth"
	"edugrade-enterprise/services/api-gateway/internal/files"
	"edugrade-enterprise/services/api-gateway/internal/paper"
)

// This proves the production permission is earned from independent evidence,
// then remains revocable until the worker result is actually applied. The
// seeded OMR facts are deliberately database-level test fixtures; the worker
// extraction algorithm itself is validated by the page-processing acceptance
// suite.
func TestStory056TemplateDifferenceCalibrationApprovalAndRevocationE2E(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("EDUGRADE_E2E_DATABASE_URL"))
	if dsn == "" {
		t.Skip("EDUGRADE_E2E_DATABASE_URL is not set; skipping STORY-056 calibration PostgreSQL test")
	}

	db := e2eOpenPostgresTestDB(t, dsn)
	e2eApplyPostgresMigrations(t, db)
	e2eActivatePostgresDemoUsers(t, db, []string{"tenant_admin"})
	router := e2ePostgresRouter(db)
	suffix := time.Now().UTC().Format("20060102150405.000000000")
	adminToken := e2eLoginWithTenant(t, router, "demo", "tenant_admin", "ChangeMe123!")
	approverID, approverUsername := e2eCreateStory056CalibrationApprover(t, db, suffix)
	approverToken := e2eLoginWithTenant(t, router, "demo", approverUsername, "ChangeMe123!")
	fixture := e2eCreateStory056AcceptanceFixtureWithOMRProfile(t, db, router, adminToken, suffix, map[string]any{
		"mode": paper.OMRProfileModeTemplateDifference, "version": paper.OMRProfileVersionTemplateDifferenceBubbleV1,
	})
	e2eSeedStory056AcceptanceAnswersForQuestion(t, db, fixture, suffix+"-calibration", "single_choice", paper.OMRCalibrationMinimumSampleCount)

	seedRun := e2eStartStory056Run(t, router, adminToken, fixture.ExamID, "story056-calibration-seed-"+suffix)
	seedRunID := e2eString(t, seedRun, "id")
	e2eSeedStory056CalibrationOMRFacts(t, db, fixture.TenantID, seedRunID, fixture.QuestionIDs["single_choice"])

	created := e2ePostJSON(t, router, http.MethodPost, "/api/v1/answer-sheet-templates/"+fixture.TemplateID+"/omr-calibrations", adminToken, story056JSON(t, map[string]any{
		"question_id": fixture.QuestionIDs["single_choice"],
	}), http.StatusCreated)["calibration"].(map[string]any)
	session := created["session"].(map[string]any)
	calibrationID := e2eString(t, session, "id")
	cases, ok := created["cases"].([]any)
	if !ok || len(cases) != paper.OMRCalibrationMinimumSampleCount || e2eFloat(t, session, "sample_count") != paper.OMRCalibrationMinimumSampleCount {
		t.Fatalf("calibration must freeze exactly the required deterministic sample: %#v", created)
	}
	if e2eFloat(t, session, "minimum_confidence") != paper.OMRCalibrationMinimumConfidence || e2eString(t, session, "status") != "draft" {
		t.Fatalf("calibration must freeze the minimum evidence threshold: %#v", session)
	}

	// The creator cannot self-approve, before or after doing the label work.
	e2eExpectStatus(t, router, http.MethodPost, "/api/v1/omr-calibrations/"+calibrationID+"/approve", adminToken, story056JSON(t, map[string]any{
		"approval_note": "creator must not approve this calibration",
	}), http.StatusForbidden)
	for _, raw := range cases {
		item, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("unexpected calibration case: %#v", raw)
		}
		e2ePostJSON(t, router, http.MethodPost, "/api/v1/omr-calibrations/"+calibrationID+"/cases/"+e2eString(t, item, "id")+"/label", adminToken, story056JSON(t, map[string]any{
			"expected_option": e2eStory056CalibrationObservedOption(t, item),
		}), http.StatusOK)
	}

	approved := e2ePostJSON(t, router, http.MethodPost, "/api/v1/omr-calibrations/"+calibrationID+"/approve", approverToken, story056JSON(t, map[string]any{
		"approval_note": "independent reviewer verified the complete calibration evidence",
	}), http.StatusOK)["calibration"].(map[string]any)
	approvedSession := approved["session"].(map[string]any)
	if e2eString(t, approvedSession, "status") != "approved" || e2eString(t, approvedSession, "approved_by") != approverID || !strings.HasPrefix(e2eString(t, approvedSession, "evidence_hash"), "sha256:") {
		t.Fatalf("independent approval must freeze a durable evidence hash: %#v", approvedSession)
	}

	firstCase := cases[0].(map[string]any)
	firstSegmentID := e2eString(t, firstCase, "answer_segment_id")
	firstRunID, firstItem, firstLease := e2eStory056CalibrationReprocessAndClaim(t, router, adminToken, firstSegmentID, suffix+"-approved")
	var profileVersion, profileHash, referenceSHA256, runCalibrationID, runEvidenceHash, autoReason string
	var autoEligible bool
	var minimumConfidence float64
	if err := db.QueryRow(`
SELECT profile_version,profile_hash,reference_sha256,calibration_session_id::text,calibration_evidence_hash,
  auto_confirm_eligible,auto_confirm_reason,auto_confirm_min_confidence
FROM omr_run
WHERE tenant_id=$1::uuid AND scoring_run_id=$2::uuid AND deleted_at IS NULL`, fixture.TenantID, firstRunID).Scan(
		&profileVersion, &profileHash, &referenceSHA256, &runCalibrationID, &runEvidenceHash,
		&autoEligible, &autoReason, &minimumConfidence); err != nil {
		t.Fatalf("load approved OMR policy snapshot: %v", err)
	}
	if !autoEligible || autoReason != paper.OMRAutoConfirmReasonCalibrationApproved || runCalibrationID != calibrationID || runEvidenceHash != e2eString(t, approvedSession, "evidence_hash") || minimumConfidence != paper.OMRCalibrationMinimumConfidence {
		t.Fatalf("new OMR work must snapshot the exact approved calibration: eligible=%t reason=%s calibration=%s evidence=%s minimum=%v", autoEligible, autoReason, runCalibrationID, runEvidenceHash, minimumConfidence)
	}
	overlay := e2eCreateStory056Overlay(t, files.NewPostgresStore(db), fixture, firstLease.OMRRunID, 59)
	firstResult := e2ePostJSON(t, router, http.MethodPost, "/api/v1/internal/omr-runs/"+firstLease.OMRRunID+"/result", adminToken,
		story056JSON(t, e2eStory056CalibrationResultPayload(t, firstLease, firstItem, overlay, "story056-calibration-approved-"+firstLease.OMRRunID, profileVersion, profileHash, referenceSHA256)), http.StatusOK)
	if _, found := firstResult["question_grade"]; !found {
		t.Fatalf("approved calibration must permit a high-confidence automatic grade: %#v", firstResult)
	}

	secondCase := cases[1].(map[string]any)
	secondSegmentID := e2eString(t, secondCase, "answer_segment_id")
	secondRunID, secondItem, secondLease := e2eStory056CalibrationReprocessAndClaim(t, router, adminToken, secondSegmentID, suffix+"-revoke")
	var queuedCalibrationID string
	if err := db.QueryRow(`SELECT calibration_session_id::text FROM omr_run WHERE tenant_id=$1::uuid AND scoring_run_id=$2::uuid AND deleted_at IS NULL`, fixture.TenantID, secondRunID).Scan(&queuedCalibrationID); err != nil {
		t.Fatalf("load queued calibrated OMR run: %v", err)
	}
	if queuedCalibrationID != calibrationID {
		t.Fatalf("queued OMR must retain the calibration snapshot before revocation: %s", queuedCalibrationID)
	}
	e2ePostJSON(t, router, http.MethodPost, "/api/v1/omr-calibrations/"+calibrationID+"/revoke", approverToken, story056JSON(t, map[string]any{
		"reason": "observed drift requires an immediate automatic-grade stop",
	}), http.StatusOK)
	secondOverlay := e2eCreateStory056Overlay(t, files.NewPostgresStore(db), fixture, secondLease.OMRRunID, 60)
	secondResult := e2ePostJSON(t, router, http.MethodPost, "/api/v1/internal/omr-runs/"+secondLease.OMRRunID+"/result", adminToken,
		story056JSON(t, e2eStory056CalibrationResultPayload(t, secondLease, secondItem, secondOverlay, "story056-calibration-revoked-"+secondLease.OMRRunID, profileVersion, profileHash, referenceSHA256)), http.StatusOK)
	if _, found := secondResult["question_grade"]; found {
		t.Fatalf("revocation before callback must prevent automatic grading: %#v", secondResult)
	}
	var blockedEligible bool
	var blockedReason, reviewReason string
	var blockedGrades int
	if err := db.QueryRow(`
SELECT o.auto_confirm_eligible,o.auto_confirm_reason,
  (SELECT count(*) FROM question_grade g WHERE g.tenant_id=o.tenant_id AND g.scoring_run_id=o.scoring_run_id AND g.answer_segment_id=o.answer_segment_id AND g.deleted_at IS NULL),
  COALESCE((SELECT reason_code FROM review_task r WHERE r.tenant_id=o.tenant_id AND r.scoring_run_id=o.scoring_run_id AND r.answer_segment_id=o.answer_segment_id AND r.source='omr_ambiguous' AND r.deleted_at IS NULL ORDER BY r.created_at DESC LIMIT 1),'')
FROM omr_run o
WHERE o.tenant_id=$1::uuid AND o.id=$2::uuid`, fixture.TenantID, secondLease.OMRRunID).Scan(&blockedEligible, &blockedReason, &blockedGrades, &reviewReason); err != nil {
		t.Fatalf("inspect revoked OMR result: %v", err)
	}
	if blockedEligible || blockedReason != paper.OMRAutoConfirmReasonCalibrationRevoked || blockedGrades != 0 || reviewReason != "omr_"+paper.OMRAutoConfirmReasonCalibrationRevoked {
		t.Fatalf("revoked calibration must become a human-review result: eligible=%t reason=%s grades=%d review=%s", blockedEligible, blockedReason, blockedGrades, reviewReason)
	}
}

func e2eCreateStory056CalibrationApprover(t *testing.T, db *sql.DB, suffix string) (string, string) {
	t.Helper()
	username := "story056_calibration_approver_" + strings.ReplaceAll(suffix, ".", "_")
	hash, err := auth.HashPassword("ChangeMe123!")
	if err != nil {
		t.Fatalf("hash STORY-056 calibration approver password: %v", err)
	}
	var userID string
	err = db.QueryRow(`
WITH demo AS (
  SELECT id FROM tenant WHERE code='demo' AND deleted_at IS NULL
), admin_role AS (
  SELECT r.id,r.tenant_id FROM role r JOIN demo d ON d.id=r.tenant_id WHERE r.code='tenant_admin' AND r.deleted_at IS NULL
), new_user AS (
  INSERT INTO app_user (tenant_id,username,display_name,password_hash,status)
  SELECT d.id,$1,'STORY-056 Independent Calibration Approver',$2,'active' FROM demo d
  RETURNING id,tenant_id
)
INSERT INTO user_role (tenant_id,user_id,role_id,data_scope)
SELECT r.tenant_id,u.id,r.id,jsonb_build_object('scope','tenant')
FROM admin_role r JOIN new_user u ON u.tenant_id=r.tenant_id
RETURNING user_id::text`, username, hash).Scan(&userID)
	if err != nil {
		t.Fatalf("create independent STORY-056 calibration approver: %v", err)
	}
	return userID, username
}

func e2eSeedStory056CalibrationOMRFacts(t *testing.T, db *sql.DB, tenantID, runID, questionID string) {
	t.Helper()
	result, err := db.Exec(`
WITH candidates AS (
  SELECT o.id,
    CASE ((row_number() OVER (ORDER BY o.id) - 1) % 4)
      WHEN 0 THEN 'A' WHEN 1 THEN 'B' WHEN 2 THEN 'C' ELSE 'D'
    END AS option_label
  FROM omr_run o
  JOIN answer_segment seg ON seg.tenant_id=o.tenant_id AND seg.id=o.answer_segment_id AND seg.deleted_at IS NULL
  WHERE o.tenant_id=$1::uuid AND o.scoring_run_id=$2::uuid AND seg.question_id=$3::uuid AND o.deleted_at IS NULL
  ORDER BY o.id
  LIMIT 100
)
UPDATE omr_run o
SET status='completed',decision='selected',selected_options=jsonb_build_array(c.option_label),confidence=0.99,
  measurements=jsonb_build_array(jsonb_build_object('option',c.option_label,'foreground_delta',0.9)),
  result_version='story056-calibration-seed-v1',duration_ms=1,completed_at=now(),updated_at=now()
FROM candidates c
WHERE o.id=c.id`, tenantID, runID, questionID)
	if err != nil {
		t.Fatalf("seed completed OMR calibration evidence: %v", err)
	}
	if count, _ := result.RowsAffected(); count != paper.OMRCalibrationMinimumSampleCount {
		t.Fatalf("expected %d calibration OMR facts, got %d", paper.OMRCalibrationMinimumSampleCount, count)
	}
	result, err = db.Exec(`
UPDATE agent_worker_task task
SET status='succeeded',result_schema_version='story056-calibration-seed-v1',result_payload_hash='sha256:story056-calibration-seed',duration_ms=1,completed_at=now(),updated_at=now()
FROM omr_run o
WHERE task.tenant_id=o.tenant_id AND task.id=o.runtime_task_id
  AND o.tenant_id=$1::uuid AND o.scoring_run_id=$2::uuid AND o.deleted_at IS NULL`, tenantID, runID)
	if err != nil {
		t.Fatalf("retire seeded calibration worker tasks: %v", err)
	}
	if count, _ := result.RowsAffected(); count != paper.OMRCalibrationMinimumSampleCount {
		t.Fatalf("expected %d retired calibration worker tasks, got %d", paper.OMRCalibrationMinimumSampleCount, count)
	}
	if _, err = db.Exec(`UPDATE scoring_run SET status='completed',queued_count=0,review_count=0,failed_count=0,auto_confirmed_count=0,completed_at=now(),updated_at=now() WHERE tenant_id=$1::uuid AND id=$2::uuid`, tenantID, runID); err != nil {
		t.Fatalf("complete seeded calibration scoring run: %v", err)
	}
}

func e2eStory056CalibrationObservedOption(t *testing.T, item map[string]any) string {
	t.Helper()
	options, ok := item["observed_options"].([]any)
	if !ok || len(options) != 1 {
		t.Fatalf("calibration case must expose exactly one observed option: %#v", item)
	}
	option, ok := options[0].(string)
	if !ok || option == "" {
		t.Fatalf("calibration observed option must be a non-empty string: %#v", item)
	}
	return option
}

func e2eStory056CalibrationReprocessAndClaim(t *testing.T, router http.Handler, adminToken, segmentID, suffix string) (string, map[string]any, story056WorkerLease) {
	t.Helper()
	run := e2ePostJSON(t, router, http.MethodPost, "/api/v1/answer-segments/"+segmentID+"/reprocess-score", adminToken, story056JSON(t, map[string]any{
		"idempotency_key": "story056-calibration-" + suffix,
	}), http.StatusCreated)["scoring_run"].(map[string]any)
	runID := e2eString(t, run, "id")
	item, lease := e2eStory056ClaimSingleOMR(t, router, adminToken, runID, "story056-calibration-worker")
	return runID, item, lease
}

func e2eStory056CalibrationResultPayload(t *testing.T, lease story056WorkerLease, item map[string]any, overlay files.FileAsset, version, profileVersion, profileHash, referenceSHA256 string) map[string]any {
	t.Helper()
	payload := e2eStory056OMRResultPayload(t, lease, item, overlay, version, []map[string]any{{"option": "A", "foreground_delta": 0.9}}, false)
	payload["profile_version"] = profileVersion
	payload["profile_hash"] = profileHash
	payload["reference_sha256"] = referenceSHA256
	payload["thresholds"] = map[string]any{
		"marked_threshold": 0.12, "ambiguous_threshold": 0.04, "minimum_margin": 0.04,
		"border_fraction": 0.12, "reference_mask_dilation_pixels": 1,
	}
	return payload
}
