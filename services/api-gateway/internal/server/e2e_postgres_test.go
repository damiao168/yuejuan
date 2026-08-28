package server

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"edugrade-enterprise/services/api-gateway/internal/appeal"
	"edugrade-enterprise/services/api-gateway/internal/assessment"
	"edugrade-enterprise/services/api-gateway/internal/auth"
	"edugrade-enterprise/services/api-gateway/internal/capture"
	"edugrade-enterprise/services/api-gateway/internal/config"
	"edugrade-enterprise/services/api-gateway/internal/dashboard"
	"edugrade-enterprise/services/api-gateway/internal/evidence"
	"edugrade-enterprise/services/api-gateway/internal/exam"
	"edugrade-enterprise/services/api-gateway/internal/files"
	"edugrade-enterprise/services/api-gateway/internal/grading"
	"edugrade-enterprise/services/api-gateway/internal/logger"
	ocrpkg "edugrade-enterprise/services/api-gateway/internal/ocr"
	"edugrade-enterprise/services/api-gateway/internal/org"
	"edugrade-enterprise/services/api-gateway/internal/paper"
	"edugrade-enterprise/services/api-gateway/internal/review"
	"edugrade-enterprise/services/api-gateway/internal/score"
	"edugrade-enterprise/services/api-gateway/internal/segment"
	"edugrade-enterprise/services/api-gateway/internal/subjective"
	"edugrade-enterprise/services/api-gateway/internal/submission"
	"edugrade-enterprise/services/api-gateway/internal/workerruntime"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

func TestCoreWorkflowE2EWithPostgresTestDatabase(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("EDUGRADE_E2E_DATABASE_URL"))
	if dsn == "" {
		t.Skip("EDUGRADE_E2E_DATABASE_URL is not set; skipping PostgreSQL E2E test database workflow")
	}
	db := e2eOpenPostgresTestDB(t, dsn)
	e2eApplyPostgresMigrations(t, db)
	e2eActivatePostgresDemoUsers(t, db, []string{"tenant_admin", "teacher", "grader", "arbitrator", "student"})
	e2eActivatePostgresUsers(t, db, "platform", []string{"platform_admin"})
	router := e2ePostgresRouter(db)
	suffix := time.Now().UTC().Format("20060102150405.000000000")

	platformToken := e2eLoginWithTenant(t, router, "platform", "platform_admin", "ChangeMe123!")
	adminToken := e2eLoginWithTenant(t, router, "demo", "tenant_admin", "ChangeMe123!")
	teacherToken := e2eLoginWithTenant(t, router, "demo", "teacher", "ChangeMe123!")
	teacherID := e2eLookupUserID(t, db, "demo", "teacher")
	graderID := e2eLookupUserID(t, db, "demo", "grader")
	var demoTenantID string
	if err := db.QueryRow(`SELECT id::text FROM tenant WHERE code='demo' AND deleted_at IS NULL`).Scan(&demoTenantID); err != nil {
		t.Fatalf("lookup demo tenant: %v", err)
	}
	graderToken := e2eLoginWithTenant(t, router, "demo", "grader", "ChangeMe123!")
	appealReviewerID := e2eLookupUserID(t, db, "demo", "arbitrator")
	appealReviewerToken := e2eLoginWithTenant(t, router, "demo", "arbitrator", "ChangeMe123!")

	provisionedAdminUsername := "story041_admin_" + strings.ReplaceAll(suffix, ".", "_")
	tenant := e2ePostJSON(t, router, http.MethodPost, "/api/v1/tenants", platformToken, `{"name":"Story 041 Synthetic Tenant `+suffix+`","code":"story041-`+suffix+`","admin_username":"`+provisionedAdminUsername+`","admin_display_name":"Story 041 School Admin","admin_password":"Story041Admin!"}`, http.StatusCreated)["tenant"].(map[string]any)
	if tenant["status"] != "active" {
		t.Fatalf("test database tenant should be active: %#v", tenant)
	}
	school := e2ePostJSON(t, router, http.MethodPost, "/api/v1/schools", adminToken, `{"name":"Story 041 Synthetic School","code":"story041-`+suffix+`"}`, http.StatusCreated)["school"].(map[string]any)
	schoolID := e2eString(t, school, "id")
	grade := e2ePostJSON(t, router, http.MethodPost, "/api/v1/grades", adminToken, `{"school_id":"`+schoolID+`","name":"Story 041 Grade","level_no":10,"academic_year":"2026"}`, http.StatusCreated)["grade"].(map[string]any)
	gradeID := e2eString(t, grade, "id")
	var schoolHasSeniorStage bool
	if err := db.QueryRow(`SELECT education_stages @> '["senior"]'::jsonb FROM school WHERE tenant_id=$1::uuid AND id=$2::uuid`, demoTenantID, schoolID).Scan(&schoolHasSeniorStage); err != nil || !schoolHasSeniorStage {
		t.Fatalf("creating a senior grade must update the school's education stages, present=%t err=%v", schoolHasSeniorStage, err)
	}
	class := e2ePostJSON(t, router, http.MethodPost, "/api/v1/classes", adminToken, `{"school_id":"`+schoolID+`","grade_id":"`+gradeID+`","name":"Story 041 Class","code":"story041-`+suffix+`"}`, http.StatusCreated)["class"].(map[string]any)
	classID := e2eString(t, class, "id")
	student := e2ePostJSON(t, router, http.MethodPost, "/api/v1/students", adminToken, `{"school_id":"`+schoolID+`","class_id":"`+classID+`","student_no":"SYN-`+suffix+`","name":"Story 041 Synthetic Student"}`, http.StatusCreated)["student"].(map[string]any)
	studentID := e2eString(t, student, "id")
	otherStudent := e2ePostJSON(t, router, http.MethodPost, "/api/v1/students", adminToken, `{"school_id":"`+schoolID+`","class_id":"`+classID+`","student_no":"SYN-OTHER-`+suffix+`","name":"Story 041 Other Synthetic Student"}`, http.StatusCreated)["student"].(map[string]any)
	otherStudentID := e2eString(t, otherStudent, "id")
	otherUsername := "story041_other_" + strings.ReplaceAll(suffix, ".", "_")
	e2eSeedPostgresStudentScopes(t, db, studentID, otherStudentID, otherUsername)
	studentToken := e2eLoginWithTenant(t, router, "demo", "student", "ChangeMe123!")
	otherStudentToken := e2eLoginWithTenant(t, router, "demo", otherUsername, "ChangeMe123!")

	examResp := e2ePostJSON(t, router, http.MethodPost, "/api/v1/exams", adminToken, `{"school_id":"`+schoolID+`","name":"Story 041 Synthetic Exam","subject":"physics","exam_type":"midterm","total_score":5,"grading_mode":"ai_assisted","appeal_enabled":true,"publish_policy":"manual_after_confirmation","class_ids":["`+classID+`"]}`, http.StatusCreated)["exam"].(map[string]any)
	examID := e2eString(t, examResp, "id")
	var candidateCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM exam_candidate_snapshot WHERE tenant_id=$1::uuid AND exam_id=$2::uuid`, demoTenantID, examID).Scan(&candidateCount); err != nil || candidateCount != 2 {
		t.Fatalf("exam creation must freeze two candidate identities, count=%d err=%v", candidateCount, err)
	}
	if _, err := db.Exec(`UPDATE student SET name='Story 041 Current Renamed Student' WHERE tenant_id=$1::uuid AND id=$2::uuid`, demoTenantID, studentID); err != nil {
		t.Fatalf("rename current student after snapshot: %v", err)
	}
	transferClass := e2ePostJSON(t, router, http.MethodPost, "/api/v1/classes", adminToken, `{"school_id":"`+schoolID+`","grade_id":"`+gradeID+`","name":"Story 041 Transfer Class","code":"story041-transfer-`+suffix+`"}`, http.StatusCreated)["class"].(map[string]any)
	transferClassID := e2eString(t, transferClass, "id")
	e2ePostJSON(t, router, http.MethodPut, "/api/v1/students/"+studentID+"/enrollment", adminToken, `{"class_id":"`+transferClassID+`"}`, http.StatusOK)
	enrollments := e2eGetJSON(t, router, "/api/v1/students/"+studentID+"/enrollments", adminToken, http.StatusOK)["enrollments"].([]any)
	if len(enrollments) != 2 || enrollments[0].(map[string]any)["class_id"] != transferClassID || enrollments[1].(map[string]any)["class_id"] != classID {
		t.Fatalf("student transfer must preserve enrollment history: %#v", enrollments)
	}
	roster := e2eGetJSON(t, router, "/api/v1/exams/"+examID+"/roster", adminToken, http.StatusOK)["roster"].(map[string]any)
	foundFrozenIdentity := false
	for _, rawEntry := range roster["entries"].([]any) {
		entry := rawEntry.(map[string]any)
		if entry["student_id"] == studentID {
			foundFrozenIdentity = entry["student_name"] == "Story 041 Synthetic Student" && entry["class_id"] == classID
		}
	}
	if !foundFrozenIdentity {
		t.Fatalf("exam roster must retain the student name captured at exam creation: %#v", roster)
	}
	paperFileID := e2eUploadSyntheticPDF(t, router, adminToken, "story041-paper-"+suffix+".pdf", "%PDF-1.4\n% story 041 synthetic paper "+suffix+"\n")
	paperResp := e2ePostJSON(t, router, http.MethodPost, "/api/v1/exams/"+examID+"/papers", adminToken, `{"file_asset_id":"`+paperFileID+`"}`, http.StatusCreated)["paper"].(map[string]any)
	paperID := e2eString(t, paperResp, "id")
	question := e2ePostJSON(t, router, http.MethodPost, "/api/v1/exams/"+examID+"/questions", adminToken, `{"exam_paper_id":"`+paperID+`","question_no":"Q1","question_type":"short_answer","score":5,"stem":"Story 041 synthetic short answer","knowledge_points":["story041"],"answer_area":{"page":1,"x":0.1,"y":0.2,"w":0.6,"h":0.2},"sort_order":1}`, http.StatusCreated)["question"].(map[string]any)
	questionID := e2eString(t, question, "id")
	e2ePostJSON(t, router, http.MethodPost, "/api/v1/questions/"+questionID+"/rubric", adminToken, `{"status":"approved","max_score":5,"points":[{"id":"p1","description":"story041 synthetic rubric point","score":5,"required":true}],"deductions":[],"examples":[]}`, http.StatusCreated)
	biologyProfiles := e2eGetJSON(t, router, "/api/v1/assessment/subject-profiles?stage=senior&subject=biology", adminToken, http.StatusOK)["subject_profiles"].([]any)
	if len(biologyProfiles) != 1 {
		t.Fatalf("expected one senior biology assessment profile for the cross-subject rejection check, got %#v", biologyProfiles)
	}
	biologyProfileID := e2eString(t, biologyProfiles[0].(map[string]any), "id")
	e2ePostJSON(t, router, http.MethodPut, "/api/v1/exams/"+examID+"/questions/"+questionID+"/assessment-profile", adminToken, `{"subject_profile_id":"`+biologyProfileID+`","archetype_code":"short_constructed","allowed_evidence_types":["text_span","concept"],"risk_tier":"R2","scoring_policy":{"mode":"AI_ASSIST","require_evidence":true,"human_review_below_confidence":true},"expected_revision":0}`, http.StatusBadRequest)

	profiles := e2eGetJSON(t, router, "/api/v1/assessment/subject-profiles?stage=senior&subject=physics", adminToken, http.StatusOK)["subject_profiles"].([]any)
	if len(profiles) != 1 {
		t.Fatalf("expected one senior physics assessment profile, got %#v", profiles)
	}
	profileID := e2eString(t, profiles[0].(map[string]any), "id")
	e2ePostJSON(t, router, http.MethodPut, "/api/v1/exams/"+examID+"/questions/"+questionID+"/assessment-profile", adminToken, `{"subject_profile_id":"`+profileID+`","archetype_code":"short_constructed","allowed_evidence_types":["text_span"],"risk_tier":"R2","scoring_policy":{"mode":"AI_ASSIST","require_evidence":true,"human_review_below_confidence":true},"expected_revision":0}`, http.StatusOK)
	if _, err := db.Exec(`UPDATE exam SET status='ready', updated_at=now() WHERE id=$1::uuid`, examID); err != nil {
		t.Fatalf("freeze assessment snapshot for PostgreSQL workflow: %v", err)
	}
	snapshot := e2eGetJSON(t, router, "/api/v1/exams/"+examID+"/questions/"+questionID+"/assessment-snapshot", adminToken, http.StatusOK)["assessment_snapshot"].(map[string]any)
	if e2eString(t, snapshot, "subject_code") != "physics" || e2eString(t, snapshot, "archetype_code") != "short_constructed" {
		t.Fatalf("grading must use the frozen assessment snapshot: %#v", snapshot)
	}
	validation := e2ePostJSON(t, router, http.MethodPost, "/api/v1/exams/"+examID+"/validate-paper-config", adminToken, `{}`, http.StatusOK)["result"].(map[string]any)
	if validation["valid"] != true {
		t.Fatalf("test database paper config should be valid: %#v", validation)
	}

	answerFileID := e2eUploadSyntheticPDF(t, router, adminToken, "story041-answer-"+suffix+".pdf", "%PDF-1.4\n% story 041 synthetic answer "+suffix+"\n")
	submissionResp := e2ePostJSON(t, router, http.MethodPost, "/api/v1/exams/"+examID+"/submissions", adminToken, `{"student_id":"`+studentID+`","candidate_no":"SYN-`+suffix+`","source_type":"pdf_upload","expected_page_count":1}`, http.StatusCreated)["submission"].(map[string]any)
	submissionID := e2eString(t, submissionResp, "id")
	pageResp := e2ePostJSON(t, router, http.MethodPost, "/api/v1/submissions/"+submissionID+"/pages", adminToken, `{"file_asset_id":"`+answerFileID+`","page_no":1}`, http.StatusCreated)["page"].(map[string]any)
	pageID := e2eString(t, pageResp, "id")
	e2ePostJSON(t, router, http.MethodPost, "/api/v1/submissions/"+submissionID+"/quality-check", adminToken, `{}`, http.StatusOK)
	e2ePostJSON(t, router, http.MethodPost, "/api/v1/submissions/"+submissionID+"/status", adminToken, `{"status":"ready_for_ocr","expected_revision":1}`, http.StatusOK)

	ocrTask := e2ePostJSON(t, router, http.MethodPost, "/api/v1/submissions/"+submissionID+"/ocr-tasks", adminToken, `{"engine":"mock_ocr","engine_version":"story041-synthetic","min_confidence":0.8}`, http.StatusCreated)["task"].(map[string]any)
	ocrTaskID := e2eString(t, ocrTask, "id")
	runtimeClaim := e2ePostJSON(t, router, http.MethodPost, "/api/v1/internal/worker/tasks/claim", adminToken, `{"queue_name":"ocr","worker_service":"story041-e2e","worker_instance_id":"story041-e2e-1","limit":1,"lease_seconds":300}`, http.StatusOK)
	runtimeTasks := runtimeClaim["tasks"].([]any)
	if len(runtimeTasks) != 1 {
		t.Fatalf("expected one OCR runtime task, got %#v", runtimeTasks)
	}
	runtimeTask := runtimeTasks[0].(map[string]any)
	if e2eString(t, runtimeTask, "source_id") != ocrTaskID {
		t.Fatalf("runtime task does not reference OCR source: %#v", runtimeTask)
	}
	runtimeTaskID := e2eString(t, runtimeTask, "id")
	runtimeLeaseToken := e2eString(t, runtimeTask, "lease_token")
	e2ePostJSON(t, router, http.MethodPost, "/api/v1/ocr-tasks/"+ocrTaskID+"/start", adminToken, `{}`, http.StatusOK)
	answerText := "Story 041 synthetic answer mentions sunlight and plant energy."
	ocrDone := e2ePostJSON(t, router, http.MethodPost, "/api/v1/ocr-tasks/"+ocrTaskID+"/results", adminToken, `{"worker_id":"story041-e2e-1","model_version":"mock-ocr-story041","config_hash":"story041-config","input_hash":"story041-input","duration_ms":12,"preprocess_profile":"story041-synthetic","runtime_task_id":"`+runtimeTaskID+`","runtime_lease_token":"`+runtimeLeaseToken+`","results":[{"submission_page_id":"`+pageID+`","text":"`+answerText+`","bbox":[0.1,0.2,0.6,0.2],"confidence":0.76,"source_image_file_id":"`+answerFileID+`"}]}`, http.StatusOK)["task"].(map[string]any)
	if ocrDone["status"] != "completed" {
		t.Fatalf("ocr task should complete in test database workflow: %#v", ocrDone)
	}
	segmentResult := e2ePostJSON(t, router, http.MethodPost, "/api/v1/submissions/"+submissionID+"/segment-answers", adminToken, `{}`, http.StatusOK)["result"].(map[string]any)
	segments := segmentResult["segments"].([]any)
	if len(segments) != 1 {
		t.Fatalf("test database workflow expected one answer segment, got %#v", segments)
	}
	segmentID := e2eString(t, segments[0].(map[string]any), "id")
	e2ePostJSON(t, router, http.MethodPut, "/api/v1/answer-segments/"+segmentID+"/answer", adminToken, `{"answer_text":"`+answerText+`","answer_payload":{"answer":"`+answerText+`"},"source":"ocr_text","confidence":0.76}`, http.StatusOK)
	aiGrade := e2ePostJSON(t, router, http.MethodPost, "/api/v1/answer-segments/"+segmentID+"/subjective-ai-grade", adminToken, `{"model_policy":{"model_version":"mock-llm-story041","prompt_version":"story041-synthetic","min_confidence":0.8}}`, http.StatusCreated)["grade"].(map[string]any)
	if aiGrade["mock"] != true || aiGrade["needs_human_review"] != true {
		t.Fatalf("PostgreSQL subjective grade must be explicit mock requiring review: %#v", aiGrade)
	}
	aiGradeID := e2eString(t, aiGrade, "id")
	evidenceJob := e2ePostJSON(t, router, http.MethodPost, "/api/v1/ai-grades/"+aiGradeID+"/verify-evidence", adminToken, `{}`, http.StatusCreated)["job"].(map[string]any)
	if evidenceJob["needs_human_review"] != true {
		t.Fatalf("PostgreSQL evidence job should require review for mock grade: %#v", evidenceJob)
	}

	reviewTask := e2ePostJSON(t, router, http.MethodPost, "/api/v1/review-tasks", adminToken, `{"answer_segment_id":"`+segmentID+`","source":"evidence_verification_failed","priority":5}`, http.StatusCreated)["task"].(map[string]any)
	reviewTaskID := e2eString(t, reviewTask, "id")
	e2eExpectStatus(t, router, http.MethodPost, "/api/v1/exams/"+examID+"/publish", adminToken, `{"reason":"too early"}`, http.StatusConflict)
	e2eExpectStatus(t, router, http.MethodPost, "/api/v1/review-tasks/"+reviewTaskID+"/assign", teacherToken, `{"assigned_to":"`+graderID+`","expected_revision":1}`, http.StatusForbidden)
	e2ePostJSON(t, router, http.MethodPost, "/api/v1/review-tasks/"+reviewTaskID+"/assign", adminToken, `{"assigned_to":"`+graderID+`","expected_revision":1}`, http.StatusOK)
	e2eExpectStatus(t, router, http.MethodGet, "/api/v1/review-tasks/"+reviewTaskID+"/original-image", teacherToken, "", http.StatusForbidden)
	draftStore := review.NewPostgresStore(db)
	draft, err := draftStore.SaveDraft(context.Background(), demoTenantID, reviewTaskID, graderID, review.SaveDraftInput{
		Comments:         "grader-owned draft",
		RubricSelections: []review.RubricSelection{},
		ViewerState: map[string]any{
			"mode": "segment",
		},
	})
	if err != nil {
		t.Fatalf("save PostgreSQL review draft: %v", err)
	}
	if _, err := db.Exec(`UPDATE review_task SET assigned_to=$3::uuid,updated_at=now() WHERE tenant_id=$1::uuid AND id=$2::uuid`, demoTenantID, reviewTaskID, teacherID); err != nil {
		t.Fatalf("simulate review task reassignment: %v", err)
	}
	if _, err := draftStore.GetDraft(context.Background(), demoTenantID, reviewTaskID, graderID); !errors.Is(err, review.ErrNotFound) {
		t.Fatalf("former reviewer must not read draft after reassignment, got %v", err)
	}
	if _, err := draftStore.SaveDraft(context.Background(), demoTenantID, reviewTaskID, graderID, review.SaveDraftInput{
		Comments:         "stale reviewer overwrite",
		ExpectedRevision: draft.Revision,
	}); !errors.Is(err, review.ErrForbidden) {
		t.Fatalf("former reviewer must not write draft after reassignment, got %v", err)
	}
	if _, err := db.Exec(`UPDATE review_task SET assigned_to=$3::uuid,updated_at=now() WHERE tenant_id=$1::uuid AND id=$2::uuid`, demoTenantID, reviewTaskID, graderID); err != nil {
		t.Fatalf("restore review task assignment: %v", err)
	}
	e2eExpectStatus(t, router, http.MethodPost, "/api/v1/review-tasks/"+reviewTaskID+"/submit", graderToken, `{"expected_revision":2,"score":6,"rubric_selections":[{"point_id":"p1","score":6}],"comments":"over max"}`, http.StatusBadRequest)
	e2ePostJSON(t, router, http.MethodPost, "/api/v1/review-tasks/"+reviewTaskID+"/submit", graderToken, `{"expected_revision":2,"score":4,"rubric_selections":[{"point_id":"p1","score":4}],"comments":"story041 synthetic human grade","reason":"manual review after mock AI"}`, http.StatusCreated)

	finalized := e2ePostJSON(t, router, http.MethodPost, "/api/v1/exams/"+examID+"/finalize", adminToken, `{}`, http.StatusCreated)
	if finalized["status"] != "pending_confirmation" || e2eFloat(t, finalized, "created_finals") != 1 {
		t.Fatalf("PostgreSQL finalize should create one final grade: %#v", finalized)
	}
	e2eExpectStatus(t, router, http.MethodPost, "/api/v1/exams/"+examID+"/publish", adminToken, `{"reason":"before confirmation"}`, http.StatusConflict)
	e2ePostJSON(t, router, http.MethodPost, "/api/v1/exams/"+examID+"/confirm-grades", adminToken, `{"reason":"story041 synthetic confirmation"}`, http.StatusOK)
	e2ePostJSON(t, router, http.MethodPut, "/api/v1/exams/"+examID+"/roster/"+otherStudentID+"/attendance", adminToken, `{"status":"absent","reason":"synthetic student intentionally has no answer sheet"}`, http.StatusOK)
	e2ePostJSON(t, router, http.MethodPost, "/api/v1/exams/"+examID+"/publish", adminToken, `{"reason":"story041 synthetic publish"}`, http.StatusOK)
	publishedExam := e2eGetJSON(t, router, "/api/v1/exams/"+examID, adminToken, http.StatusOK)["exam"].(map[string]any)
	if publishedExam["status"] != "published" {
		t.Fatalf("publishing grades must advance the exam to published: %#v", publishedExam)
	}
	publishedQuality := e2eGetJSON(t, router, "/api/v1/exams/"+examID+"/grades/quality?stage=publish", adminToken, http.StatusOK)
	if publishedQuality["can_publish"] != true {
		t.Fatalf("published grades must remain quality-passed: %#v", publishedQuality)
	}
	studentGrade := e2eGetJSON(t, router, "/api/v1/students/"+studentID+"/exams/"+examID+"/grade", studentToken, http.StatusOK)["grade"].(map[string]any)
	if studentGrade["total_score"] != float64(4) || studentGrade["status"] != "published" {
		t.Fatalf("student should see own published grade in test database workflow: %#v", studentGrade)
	}
	e2eExpectStatus(t, router, http.MethodGet, "/api/v1/students/"+studentID+"/exams/"+examID+"/grade", otherStudentToken, "", http.StatusForbidden)
	finalID := e2eString(t, studentGrade["items"].([]any)[0].(map[string]any), "id")
	appealResp := e2ePostJSON(t, router, http.MethodPost, "/api/v1/appeals", studentToken, `{"exam_id":"`+examID+`","student_id":"`+studentID+`","target_type":"question","final_grade_id":"`+finalID+`","reason":"Story 041 synthetic appeal"}`, http.StatusCreated)["appeal"].(map[string]any)
	appealID := e2eString(t, appealResp, "id")
	e2ePostJSON(t, router, http.MethodPost, "/api/v1/appeals/"+appealID+"/assign", adminToken, `{"assigned_to":"`+appealReviewerID+`","expected_revision":1}`, http.StatusOK)
	e2ePostJSON(t, router, http.MethodPost, "/api/v1/appeals/"+appealID+"/recommendation", appealReviewerToken, `{"recommendation":"adjust_score","reason":"Story 041 synthetic independent appeal review","recommended_score":5,"expected_revision":2}`, http.StatusOK)
	appealReviewed := e2ePostJSON(t, router, http.MethodPost, "/api/v1/appeals/"+appealID+"/review", adminToken, `{"status":"score_adjusted","reason":"Story 041 synthetic appeal adjustment","adjusted_score":5,"expected_revision":3}`, http.StatusOK)
	if appealReviewed["score_adjustment"] == nil {
		t.Fatalf("PostgreSQL appeal review should create score adjustment: %#v", appealReviewed)
	}
	audits := e2eGetJSON(t, router, "/api/v1/audit-logs?limit=200", adminToken, http.StatusOK)["audit_logs"].([]any)
	e2eAssertAuditActions(t, audits, []string{"score.roster_attendance_updated", "score.published", "appeal.assigned", "appeal.teacher_recommendation_submitted", "appeal.reviewed", "review.human_grade_submitted"})
}

func e2ePostgresRouter(db *sql.DB) http.Handler {
	cfg := config.Config{
		Service: config.ServiceConfig{Name: "api-gateway-postgres-e2e-test", Environment: "test", ReadinessTimeout: time.Millisecond},
		Auth:    config.AuthConfig{SessionTTL: time.Hour},
		Barcode: config.BarcodeConfig{
			ActiveKeyID: e2eBarcodeKeyring().ActiveKeyID,
			HMACKeys:    e2eBarcodeKeyring().Keys,
		},
		Files: config.FileConfig{
			Bucket:            "edugrade-story041-e2e",
			MaxUploadBytes:    2 * 1024 * 1024,
			AllowedExtensions: []string{".pdf", ".png", ".jpg", ".jpeg", ".csv", ".docx"},
		},
		Security: config.SecurityConfig{MaxRequestBodyBytes: 2 * 1024 * 1024},
	}
	return NewRouterComplete(
		cfg,
		logger.New(io.Discard, "error"),
		nil,
		auth.NewPostgresStore(db),
		org.NewPostgresStore(db),
		exam.NewPostgresStore(db),
		paper.NewPostgresStore(db),
		files.NewPostgresStore(db),
		files.NewMemoryObjectStorage(),
		submission.NewPostgresStore(db),
		ocrpkg.NewPostgresStore(db),
		ocrpkg.NewMemoryQueue(),
		segment.NewPostgresStore(db),
		workerruntime.NewPostgresStore(db),
		grading.NewPostgresStore(db),
		subjective.NewPostgresStore(db),
		assessment.NewPostgresStore(db),
		evidence.NewPostgresStore(db),
		review.NewPostgresStore(db),
		score.NewPostgresStore(db),
		appeal.NewPostgresStore(db),
		capture.NewPostgresStoreWithBarcodeKeyring(db, e2eBarcodeKeyring()),
		dashboard.NewPostgresOrganizationSummaryStore(db),
	)
}

func e2eBarcodeKeyring() capture.BarcodeKeyring {
	return capture.BarcodeKeyring{
		ActiveKeyID: "story060-e2e",
		Keys:        map[string][]byte{"story060-e2e": []byte(strings.Repeat("s", 32))},
	}
}

func e2eOpenPostgresTestDB(t *testing.T, dsn string) *sql.DB {
	t.Helper()
	config, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("parse PostgreSQL test database URL: %v", err)
	}
	adminConfig := config.Copy()
	adminConfig.Database = "postgres"
	adminDB := stdlib.OpenDB(*adminConfig)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := adminDB.PingContext(ctx); err != nil {
		_ = adminDB.Close()
		t.Fatalf("ping PostgreSQL administration database: %v", err)
	}
	databaseName := "edugrade_e2e_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	quotedDatabaseName := pgx.Identifier{databaseName}.Sanitize()
	if _, err := adminDB.ExecContext(ctx, "CREATE DATABASE "+quotedDatabaseName); err != nil {
		_ = adminDB.Close()
		t.Fatalf("create isolated PostgreSQL E2E database: %v", err)
	}
	testConfig := config.Copy()
	testConfig.Database = databaseName
	db := stdlib.OpenDB(*testConfig)
	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		_, _ = adminDB.ExecContext(ctx, "DROP DATABASE "+quotedDatabaseName+" WITH (FORCE)")
		_ = adminDB.Close()
		t.Fatalf("ping PostgreSQL test database: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		if _, cleanupErr := adminDB.ExecContext(cleanupCtx, "DROP DATABASE "+quotedDatabaseName+" WITH (FORCE)"); cleanupErr != nil {
			t.Errorf("drop isolated PostgreSQL E2E database %s: %v", databaseName, cleanupErr)
		}
		_ = adminDB.Close()
	})
	return db
}

func e2eApplyPostgresMigrations(t *testing.T, db *sql.DB) {
	t.Helper()
	files, err := filepath.Glob(filepath.Join("..", "..", "migrations", "*.sql"))
	if err != nil {
		t.Fatalf("find migrations: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("no migrations found for PostgreSQL E2E")
	}
	sort.Strings(files)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if _, err := db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS edugrade_e2e_applied_migration (
  name TEXT PRIMARY KEY,
  applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
)`); err != nil {
		t.Fatalf("create E2E migration ledger: %v", err)
	}
	for _, file := range files {
		name := filepath.Base(file)
		var applied bool
		err := db.QueryRowContext(ctx, `SELECT true FROM edugrade_e2e_applied_migration WHERE name=$1`, name).Scan(&applied)
		if err == nil {
			continue
		}
		if err != sql.ErrNoRows {
			t.Fatalf("read E2E migration ledger for %s: %v", name, err)
		}
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read migration %s: %v", file, err)
		}
		for _, statement := range e2eSplitSQLStatements(string(raw)) {
			if _, err := db.ExecContext(ctx, statement); err != nil {
				t.Fatalf("apply migration %s failed on statement %q: %v", filepath.Base(file), statement, err)
			}
		}
		if _, err := db.ExecContext(ctx, `INSERT INTO edugrade_e2e_applied_migration (name) VALUES ($1)`, name); err != nil {
			t.Fatalf("record E2E migration %s: %v", name, err)
		}
	}
}

func e2eSplitSQLStatements(sqlText string) []string {
	statements := []string{}
	var current strings.Builder
	inSingleQuote := false
	dollarQuoteTag := ""
	for i := 0; i < len(sqlText); {
		if dollarQuoteTag != "" {
			if strings.HasPrefix(sqlText[i:], dollarQuoteTag) {
				current.WriteString(dollarQuoteTag)
				i += len(dollarQuoteTag)
				dollarQuoteTag = ""
				continue
			}
			current.WriteByte(sqlText[i])
			i++
			continue
		}
		ch := sqlText[i]
		if inSingleQuote {
			current.WriteByte(ch)
			if ch == '\'' {
				if i+1 < len(sqlText) && sqlText[i+1] == '\'' {
					current.WriteByte(sqlText[i+1])
					i += 2
					continue
				}
				inSingleQuote = false
			}
			i++
			continue
		}
		if ch == '\'' {
			inSingleQuote = true
			current.WriteByte(ch)
			i++
			continue
		}
		if ch == '-' && i+1 < len(sqlText) && sqlText[i+1] == '-' {
			for i < len(sqlText) && sqlText[i] != '\n' {
				current.WriteByte(sqlText[i])
				i++
			}
			continue
		}
		if ch == '$' {
			if tag, ok := e2eDollarQuoteTag(sqlText[i:]); ok {
				dollarQuoteTag = tag
				current.WriteString(tag)
				i += len(tag)
				continue
			}
		}
		current.WriteByte(ch)
		if ch == ';' && !inSingleQuote {
			statement := strings.TrimSpace(current.String())
			if statement != "" {
				statements = append(statements, statement)
			}
			current.Reset()
		}
		i++
	}
	if tail := strings.TrimSpace(current.String()); tail != "" {
		statements = append(statements, tail)
	}
	return statements
}

func e2eDollarQuoteTag(sqlText string) (string, bool) {
	if sqlText == "" || sqlText[0] != '$' {
		return "", false
	}
	for i := 1; i < len(sqlText); i++ {
		if sqlText[i] == '$' {
			return sqlText[:i+1], true
		}
		if !e2eIsDollarQuoteTagChar(sqlText[i]) {
			return "", false
		}
	}
	return "", false
}

func e2eIsDollarQuoteTagChar(ch byte) bool {
	return ch == '_' ||
		(ch >= 'a' && ch <= 'z') ||
		(ch >= 'A' && ch <= 'Z') ||
		(ch >= '0' && ch <= '9')
}

func e2eLookupUserID(t *testing.T, db *sql.DB, tenantCode string, username string) string {
	t.Helper()
	var id string
	if err := db.QueryRow(`
SELECT u.id::text
FROM app_user u
JOIN tenant t ON t.id = u.tenant_id
WHERE t.code = $1 AND u.username = $2 AND u.deleted_at IS NULL
`, tenantCode, username).Scan(&id); err != nil {
		t.Fatalf("lookup user %s/%s: %v", tenantCode, username, err)
	}
	return id
}

func e2eSeedPostgresStudentScopes(t *testing.T, db *sql.DB, studentID string, otherStudentID string, otherUsername string) {
	t.Helper()
	hash, err := auth.HashPassword("ChangeMe123!")
	if err != nil {
		t.Fatalf("hash synthetic other student password: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err = db.ExecContext(ctx, `
WITH demo AS (
  SELECT id FROM tenant WHERE code = 'demo'
),
student_role AS (
  SELECT r.id, r.tenant_id FROM role r JOIN demo d ON d.id = r.tenant_id WHERE r.code = 'student'
),
existing_student AS (
  SELECT u.id, u.tenant_id FROM app_user u JOIN demo d ON d.id = u.tenant_id WHERE u.username = 'student'
),
other_user AS (
  INSERT INTO app_user (tenant_id, username, display_name, password_hash, status)
  SELECT d.id, $3, 'Story 041 Other Synthetic Student', $4, 'active'
  FROM demo d
  ON CONFLICT (tenant_id, username)
  DO UPDATE SET password_hash = EXCLUDED.password_hash, status = 'active', updated_at = now()
  RETURNING id, tenant_id
),
upsert_existing_scope AS (
  INSERT INTO user_role (tenant_id, user_id, role_id, data_scope)
  SELECT sr.tenant_id, es.id, sr.id, jsonb_build_object('scope', 'self', 'student_id', $1::uuid)
  FROM student_role sr CROSS JOIN existing_student es
  ON CONFLICT (tenant_id, user_id, role_id)
  DO UPDATE SET data_scope = EXCLUDED.data_scope, updated_at = now()
  RETURNING id
)
INSERT INTO user_role (tenant_id, user_id, role_id, data_scope)
SELECT sr.tenant_id, ou.id, sr.id, jsonb_build_object('scope', 'self', 'student_id', $2::uuid)
FROM student_role sr CROSS JOIN other_user ou
ON CONFLICT (tenant_id, user_id, role_id)
DO UPDATE SET data_scope = EXCLUDED.data_scope, updated_at = now()
`, studentID, otherStudentID, otherUsername, hash)
	if err != nil {
		t.Fatalf("seed PostgreSQL student scopes: %v", err)
	}
}

func e2eActivatePostgresDemoUsers(t *testing.T, db *sql.DB, usernames []string) {
	t.Helper()
	e2eActivatePostgresUsers(t, db, "demo", usernames)
}

func e2eActivatePostgresUsers(t *testing.T, db *sql.DB, tenantCode string, usernames []string) {
	t.Helper()
	hash, err := auth.HashPassword("ChangeMe123!")
	if err != nil {
		t.Fatalf("hash synthetic demo user password: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, username := range usernames {
		result, err := db.ExecContext(ctx, `
UPDATE app_user u
SET password_hash = $2, status = 'active', updated_at = now()
FROM tenant t
WHERE t.id = u.tenant_id
  AND t.code = $1
  AND u.username = $3
  AND u.deleted_at IS NULL
`, tenantCode, hash, username)
		if err != nil {
			t.Fatalf("activate PostgreSQL user %s/%s: %v", tenantCode, username, err)
		}
		affected, err := result.RowsAffected()
		if err != nil {
			t.Fatalf("check activated PostgreSQL demo user %s: %v", username, err)
		}
		if affected != 1 {
			t.Fatalf("expected to activate one PostgreSQL user %s/%s, affected %d", tenantCode, username, affected)
		}
	}
}
