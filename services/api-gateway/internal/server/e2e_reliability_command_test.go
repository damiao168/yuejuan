package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"edugrade-enterprise/services/api-gateway/internal/auth"
	"edugrade-enterprise/services/api-gateway/internal/capture"
	"edugrade-enterprise/services/api-gateway/internal/exam"
	"edugrade-enterprise/services/api-gateway/internal/idempotency"
	"edugrade-enterprise/services/api-gateway/internal/paper"
	"edugrade-enterprise/services/api-gateway/internal/processing"
	"edugrade-enterprise/services/api-gateway/internal/workerruntime"
	"github.com/google/uuid"
)

type failCompleteOnceIdempotencyStore struct {
	idempotency.Store
	failed bool
}

func (s *failCompleteOnceIdempotencyStore) Complete(ctx context.Context, input idempotency.BeginInput, status int, headers map[string]string, body []byte) error {
	if !s.failed {
		s.failed = true
		return errors.New("injected replay receipt failure")
	}
	return s.Store.Complete(ctx, input, status, headers, body)
}

func TestReliabilityCommandRecoveryWithPostgresTestDatabase(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("EDUGRADE_E2E_DATABASE_URL"))
	if dsn == "" {
		t.Skip("EDUGRADE_E2E_DATABASE_URL is not set outside the PostgreSQL CI job")
	}
	db := e2eOpenPostgresTestDB(t, dsn)
	e2eApplyPostgresMigrations(t, db)
	tenantID, userID, schoolID, gradeID, classID := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	academicYearID, cohortID := uuid.NewString(), uuid.NewString()
	statements := []struct {
		query string
		args  []any
	}{
		{`INSERT INTO tenant(id,tenant_id,name,code,status) VALUES($1::uuid,$1::uuid,'Reliability tenant',$2,'active')`, []any{tenantID, "reliability-" + uuid.NewString()}},
		{`INSERT INTO app_user(id,tenant_id,username,display_name,password_hash,status) VALUES($1::uuid,$2::uuid,'reliability-admin','Reliability admin','not-used','active')`, []any{userID, tenantID}},
		{`INSERT INTO school(id,tenant_id,name,code,status) VALUES($1::uuid,$2::uuid,'Reliability school','reliability-school','active')`, []any{schoolID, tenantID}},
		{`INSERT INTO academic_year(id,tenant_id,school_id,name,start_year,end_year,starts_at,ends_at,is_current,status) VALUES($1::uuid,$2::uuid,$3::uuid,'2026-2027',2026,2027,'2026-09-01','2027-08-31',true,'active')`, []any{academicYearID, tenantID, schoolID}},
		{`INSERT INTO grade_cohort(id,tenant_id,school_id,education_stage,entry_year,expected_graduation_year,name,status) VALUES($1::uuid,$2::uuid,$3::uuid,'senior',2026,2029,'2026 cohort','active')`, []any{cohortID, tenantID, schoolID}},
		{`INSERT INTO grade(id,tenant_id,school_id,name,level_no,academic_year,status,education_stage,academic_year_id,grade_cohort_id) VALUES($1::uuid,$2::uuid,$3::uuid,'Grade 10',10,'2026-2027','active','senior',$4::uuid,$5::uuid)`, []any{gradeID, tenantID, schoolID, academicYearID, cohortID}},
		{`INSERT INTO school_class(id,tenant_id,school_id,grade_id,name,code,status,academic_year_id,grade_cohort_id,class_no) VALUES($1::uuid,$2::uuid,$3::uuid,$4::uuid,'Class 1','class-1','active',$5::uuid,$6::uuid,1)`, []any{classID, tenantID, schoolID, gradeID, academicYearID, cohortID}},
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement.query, statement.args...); err != nil {
			t.Fatalf("seed command fixture: %v", err)
		}
	}
	store := exam.NewPostgresStore(db)
	scope := auth.AccessScope{TenantID: tenantID, TenantWide: true}
	commandID := "reliability-" + uuid.NewString()
	input := exam.CreateSessionInput{SchoolID: schoolID, GradeID: gradeID, Name: "Reliability " + time.Now().UTC().Format(time.RFC3339Nano), ExamType: "formal_exam", GradingMode: "ai_assisted", PublishPolicy: "after_admin_approval", CommandID: commandID, Subjects: []exam.SessionSubjectInput{{Subject: "math", TotalScore: 100, DurationMinutes: 90}}}
	first, err := store.CreateExamSession(context.Background(), scope, userID, input)
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.CreateExamSession(context.Background(), scope, userID, input)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID || len(first.Exams) != 1 || len(second.Exams) != 1 || first.Exams[0].ID != second.Exams[0].ID {
		t.Fatalf("replay duplicated business facts: %#v %#v", first, second)
	}
	var sessionCount, examCount int
	if err = db.QueryRow(`SELECT count(*),(SELECT count(*) FROM exam WHERE tenant_id=$1::uuid AND exam_session_id=$2::uuid) FROM exam_session WHERE tenant_id=$1::uuid AND created_by=$3::uuid AND command_id=$4`, tenantID, first.ID, userID, commandID).Scan(&sessionCount, &examCount); err != nil {
		t.Fatal(err)
	}
	if sessionCount != 1 || examCount != 1 {
		t.Fatalf("business fact counts session=%d exams=%d", sessionCount, examCount)
	}
	recovered, err := store.RecoverExamSessionCommand(context.Background(), scope, userID, commandID)
	if err != nil || recovered.Session == nil || recovered.Session.ID != first.ID {
		t.Fatalf("recovery=%#v err=%v", recovered, err)
	}
	notAccepted, err := store.RecoverExamSessionCommand(context.Background(), scope, userID, "not-accepted-"+uuid.NewString())
	if err != nil || notAccepted.Status != "not_accepted" {
		t.Fatalf("unaccepted command recovery=%#v err=%v", notAccepted, err)
	}
	processingCommand := "processing-" + uuid.NewString()
	if _, err = db.Exec(`INSERT INTO idempotency_record(tenant_id,actor_id,method,route,idempotency_key,request_hash,expires_at) VALUES($1::uuid,$2::uuid,'POST','/api/v1/exam-sessions',$3,'hash',now()+interval '1 day')`, tenantID, userID, processingCommand); err != nil {
		t.Fatal(err)
	}
	processingResult, err := store.RecoverExamSessionCommand(context.Background(), scope, userID, processingCommand)
	if err != nil || processingResult.Status != "processing" {
		t.Fatalf("processing command recovery=%#v err=%v", processingResult, err)
	}
	if _, err = db.Exec(`UPDATE idempotency_record SET state='completed',response_status=400,response_body=$4 WHERE tenant_id=$1::uuid AND actor_id=$2::uuid AND idempotency_key=$3`, tenantID, userID, processingCommand, []byte(`{"error":{"code":"invalid_exam_session"}}`)); err != nil {
		t.Fatal(err)
	}
	rejectedResult, err := store.RecoverExamSessionCommand(context.Background(), scope, userID, processingCommand)
	if err != nil || rejectedResult.Status != "rejected" || rejectedResult.HTTPStatus != 400 || rejectedResult.ErrorCode != "invalid_exam_session" {
		t.Fatalf("rejected command recovery=%#v err=%v", rejectedResult, err)
	}
	input.Name += " changed"
	if _, err = store.CreateExamSession(context.Background(), scope, userID, input); !errors.Is(err, exam.ErrCommandConflict) {
		t.Fatalf("changed replay error=%v", err)
	}

	newCommandInput := input
	newCommandInput.Name = first.Name
	newCommandInput.CommandID = "new-operation-" + uuid.NewString()
	separate, err := store.CreateExamSession(context.Background(), scope, userID, newCommandInput)
	if err != nil || separate.ID == first.ID {
		t.Fatalf("explicit new operation was incorrectly coalesced: %#v err=%v", separate, err)
	}
	concurrentInput := newCommandInput
	concurrentInput.CommandID = "concurrent-" + uuid.NewString()
	startConcurrent := make(chan struct{})
	type createResult struct {
		session exam.ExamSession
		err     error
	}
	concurrentResults := make(chan createResult, 2)
	for range 2 {
		go func() {
			<-startConcurrent
			session, createErr := store.CreateExamSession(context.Background(), scope, userID, concurrentInput)
			concurrentResults <- createResult{session: session, err: createErr}
		}()
	}
	close(startConcurrent)
	left, right := <-concurrentResults, <-concurrentResults
	if left.err != nil || right.err != nil || left.session.ID != right.session.ID {
		t.Fatalf("concurrent recovery left=%#v right=%#v", left, right)
	}

	// Exercise the production handler and idempotency middleware together: the
	// business transaction commits, transport-receipt persistence fails, and an
	// exact retry takes over the stale reservation without duplicating facts.
	middlewareCommandID := "middleware-" + uuid.NewString()
	middlewareInput := exam.CreateSessionInput{
		SchoolID: schoolID, GradeID: gradeID, Name: "Middleware recovery", ExamType: "formal_exam",
		GradingMode: "ai_assisted", PublishPolicy: "after_admin_approval", CommandID: middlewareCommandID,
		ClassIDs: []string{classID}, Subjects: []exam.SessionSubjectInput{{Subject: "physics", TotalScore: 100, DurationMinutes: 90,
			Sections: []exam.BlueprintSectionInput{{Title: "Questions", QuestionType: "single_choice", QuestionCount: 20, ScorePerQuestion: 5}}}},
	}
	middlewareBody, err := json.Marshal(middlewareInput)
	if err != nil {
		t.Fatal(err)
	}
	transportStore := &failCompleteOnceIdempotencyStore{Store: idempotency.NewPostgresStore(db)}
	examHandler := exam.NewHandler(store, auth.NewMemoryStore())
	commandHandler := idempotency.Middleware(transportStore, idempotency.Options{Enforce: true, ProcessingTimeout: time.Nanosecond})(http.HandlerFunc(examHandler.CreateExamSession))
	request := func() *http.Request {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/exam-sessions", bytes.NewReader(middlewareBody))
		req.Pattern = "POST /api/v1/exam-sessions"
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set(idempotency.Header, middlewareCommandID)
		ctx := auth.WithUser(req.Context(), auth.User{ID: userID, TenantID: tenantID})
		ctx = auth.WithAccessScope(ctx, scope)
		return req.WithContext(ctx)
	}
	firstResponse := httptest.NewRecorder()
	commandHandler.ServeHTTP(firstResponse, request())
	if firstResponse.Code != http.StatusServiceUnavailable || !strings.Contains(firstResponse.Body.String(), "idempotency_persist_failed") {
		t.Fatalf("injected receipt failure response=%d %s", firstResponse.Code, firstResponse.Body.String())
	}
	committed, err := store.RecoverExamSessionCommand(context.Background(), scope, userID, middlewareCommandID)
	if err != nil || committed.Status != "succeeded" || committed.Session == nil {
		t.Fatalf("business fact was not committed before lost receipt: %#v err=%v", committed, err)
	}
	secondResponse := httptest.NewRecorder()
	commandHandler.ServeHTTP(secondResponse, request())
	if secondResponse.Code != http.StatusCreated || !strings.Contains(secondResponse.Body.String(), committed.Session.ID) {
		t.Fatalf("exact retry did not recover committed resource: %d %s", secondResponse.Code, secondResponse.Body.String())
	}
	thirdResponse := httptest.NewRecorder()
	commandHandler.ServeHTTP(thirdResponse, request())
	if thirdResponse.Code != http.StatusCreated || thirdResponse.Header().Get("Idempotency-Replayed") != "true" || thirdResponse.Body.String() != secondResponse.Body.String() {
		t.Fatalf("completed replay mismatch: second=%d/%s third=%d/%s replayed=%s", secondResponse.Code, secondResponse.Body.String(), thirdResponse.Code, thirdResponse.Body.String(), thirdResponse.Header().Get("Idempotency-Replayed"))
	}
	var middlewareSessionCount int
	if err = db.QueryRow(`SELECT count(*) FROM exam_session WHERE tenant_id=$1::uuid AND created_by=$2::uuid AND command_id=$3`, tenantID, userID, middlewareCommandID).Scan(&middlewareSessionCount); err != nil || middlewareSessionCount != 1 {
		t.Fatalf("middleware retry duplicated sessions: count=%d err=%v", middlewareSessionCount, err)
	}
	t.Run("CMD-02 actual Web client recovers post-commit 503 against PostgreSQL", func(t *testing.T) {
		mux := http.NewServeMux()
		faultStore := &failCompleteOnceIdempotencyStore{Store: idempotency.NewPostgresStore(db)}
		mux.Handle("POST /api/v1/exam-sessions", idempotency.Middleware(faultStore, idempotency.Options{Enforce: true, ProcessingTimeout: time.Nanosecond})(http.HandlerFunc(examHandler.CreateExamSession)))
		mux.HandleFunc("GET /api/v1/exam-sessions/commands/{commandId}", examHandler.RecoverExamSessionCommand)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := auth.WithUser(r.Context(), auth.User{ID: userID, TenantID: tenantID})
			mux.ServeHTTP(w, r.WithContext(auth.WithAccessScope(ctx, scope)))
		}))
		defer server.Close()
		fixture := filepath.Join(t.TempDir(), "command.json")
		if err := os.WriteFile(fixture, middlewareBody, 0600); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		command := exec.CommandContext(ctx, "node", "scripts/ci/exam-command-client.mjs", server.URL, fixture)
		command.Dir = filepath.Join("..", "..", "..", "..")
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("actual client recovery failed (Node/npm ci required): %v\n%s", err, output)
		}
		var evidence struct {
			CommandIDs []string `json:"commandIds"`
			SessionIDs []string `json:"sessionIds"`
		}
		if err := json.Unmarshal(bytes.TrimSpace(output), &evidence); err != nil {
			t.Fatalf("client evidence: %v\n%s", err, output)
		}
		if len(evidence.CommandIDs) != 2 || len(evidence.SessionIDs) != 2 {
			t.Fatalf("incomplete client evidence: %s", output)
		}
		for i, commandID := range evidence.CommandIDs {
			var sessions, children int
			if err := db.QueryRow(`SELECT count(*),(SELECT count(*) FROM exam WHERE tenant_id=$1::uuid AND exam_session_id=$4::uuid) FROM exam_session WHERE tenant_id=$1::uuid AND created_by=$2::uuid AND command_id=$3`, tenantID, userID, commandID, evidence.SessionIDs[i]).Scan(&sessions, &children); err != nil {
				t.Fatal(err)
			}
			if sessions != 1 || children != len(middlewareInput.Subjects) {
				t.Fatalf("actual client duplicated business resources: sessions=%d children=%d", sessions, children)
			}
		}
		t.Logf("CMD-02/CMD-04/CMD-07 real client + PostgreSQL evidence: %s", output)
	})

	fileID := uuid.NewString()
	if _, err = db.Exec(`INSERT INTO file_asset(id,tenant_id,school_id,exam_id,owner_type,owner_id,original_name,content_type,size_bytes,hash_sha256,storage_bucket,storage_key,visibility,uploaded_by)
VALUES($1::uuid,$2::uuid,$3::uuid,$4::uuid,'import',$4::uuid,'paper.pdf','application/pdf',128,$5,'tests','paper.pdf','private',$6::uuid)`, fileID, tenantID, schoolID, first.Exams[0].ID, uuid.NewString()+uuid.NewString(), userID); err != nil {
		t.Fatalf("seed paper file: %v", err)
	}
	paperStore := paper.NewPostgresStore(db)
	createImport := paper.CreatePaperImportInput{CommandID: "paper-create-" + uuid.NewString(), Subject: "math", Sources: []paper.CreatePaperImportSourceInput{{FileAssetID: fileID, DocumentIndex: 0, RoleHint: "question"}}}
	type importCreation struct {
		job paper.PaperImportJob
		err error
	}
	importStart := make(chan struct{})
	importResults := make(chan importCreation, 2)
	for range 2 {
		go func() {
			<-importStart
			created, createErr := paperStore.CreatePaperImport(context.Background(), tenantID, first.Exams[0].ID, userID, createImport)
			importResults <- importCreation{created, createErr}
		}()
	}
	close(importStart)
	importLeft, importRight := <-importResults, <-importResults
	if importLeft.err != nil || importRight.err != nil || importLeft.job.ID != importRight.job.ID || importLeft.job.RunID != importRight.job.RunID {
		t.Fatalf("concurrent initial command created inconsistent imports: %#v %#v", importLeft, importRight)
	}
	job := importLeft.job
	replayed, err := paperStore.CreatePaperImport(context.Background(), tenantID, first.Exams[0].ID, userID, createImport)
	if err != nil || replayed.ID != job.ID || replayed.Generation != 1 || replayed.RunID != job.RunID {
		t.Fatalf("import replay=%#v err=%v", replayed, err)
	}
	changedImport := createImport
	changedImport.Sources = append([]paper.CreatePaperImportSourceInput(nil), createImport.Sources...)
	changedImport.Sources[0].RoleHint = "answer"
	if _, err = paperStore.CreatePaperImport(context.Background(), tenantID, first.Exams[0].ID, userID, changedImport); !errors.Is(err, paper.ErrConflict) {
		t.Fatalf("changed import command replay error=%v", err)
	}
	var runCount, recoverableIntent int
	if err = db.QueryRow(`SELECT count(*),count(*) FILTER (WHERE status='processing' AND dispatch_status='pending') FROM paper_import_run WHERE tenant_id=$1::uuid AND paper_import_id=$2::uuid`, tenantID, job.ID).Scan(&runCount, &recoverableIntent); err != nil {
		t.Fatal(err)
	}
	if runCount != 1 || recoverableIntent != 1 {
		t.Fatalf("initial import lacks a unique durable dispatch intent: runs=%d intents=%d", runCount, recoverableIntent)
	}
	claimedDispatch, dispatchOK, err := paperStore.ClaimPendingPaperImportDispatch(context.Background(), "initial-dispatch", time.Minute)
	if err != nil || !dispatchOK || claimedDispatch.RunID != job.RunID || claimedDispatch.Generation != job.Generation || len(claimedDispatch.Sources) != len(job.Sources) {
		t.Fatalf("dispatch did not load its immutable run: %#v ok=%v err=%v", claimedDispatch, dispatchOK, err)
	}

	runtimeStore := workerruntime.NewPostgresStore(db)
	oldTask, err := runtimeStore.CreateTask(context.Background(), tenantID, userID, workerruntime.CreateTaskInput{TaskType: "layout", QueueName: "page-processing", SourceType: "paper_import_job", SourceID: job.ID, Priority: 10, PayloadSchemaVersion: "paper-import-decode-v3", IdempotencyKey: "old-run-" + uuid.NewString(), MaxAttempts: 3, RetryBackoffSeconds: 1})
	if err != nil {
		t.Fatalf("create old task: %v", err)
	}
	if _, err = db.Exec(`UPDATE agent_worker_task SET paper_import_run_id=$3::uuid,paper_import_generation=$4,task_protocol_version=2 WHERE tenant_id=$1::uuid AND id=$2::uuid`, tenantID, oldTask.ID, job.RunID, job.Generation); err != nil {
		t.Fatal(err)
	}
	claimed, err := runtimeStore.Claim(context.Background(), tenantID, workerruntime.ClaimInput{QueueName: "page-processing", WorkerService: "test", WorkerInstanceID: "old-worker", Limit: 1, LeaseSeconds: 300})
	if err != nil || len(claimed) != 1 {
		t.Fatalf("claim old task=%#v err=%v", claimed, err)
	}
	if _, err = paperStore.CancelPaperImportGeneration(context.Background(), tenantID, job.ID, job.Generation); err != nil {
		t.Fatalf("cancel generation 1: %v", err)
	}
	if _, err = db.Exec(`CREATE FUNCTION fail_test_paper_run_insert() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected run insert failure'; END $$`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`CREATE TRIGGER fail_test_paper_run_insert BEFORE INSERT ON paper_import_run FOR EACH ROW EXECUTE FUNCTION fail_test_paper_run_insert()`); err != nil {
		t.Fatal(err)
	}
	failedRunInput := paper.ReplacePaperImportSourcesInput{CommandID: "paper-failed-run-" + uuid.NewString(), ExpectedGeneration: job.Generation, Sources: []paper.ReplacePaperImportSourceInput{{ID: job.Sources[0].ID, DocumentIndex: 0, RoleHint: "question"}}}
	if _, err = paperStore.ReplacePaperImportSources(context.Background(), tenantID, job.ID, userID, failedRunInput); err == nil {
		t.Fatal("injected run insert failure unexpectedly committed")
	}
	var generationAfterRunRollback int64
	var statusAfterRunRollback string
	if err = db.QueryRow(`SELECT current_generation,status FROM paper_import_job WHERE tenant_id=$1::uuid AND id=$2::uuid`, tenantID, job.ID).Scan(&generationAfterRunRollback, &statusAfterRunRollback); err != nil {
		t.Fatal(err)
	}
	if generationAfterRunRollback != job.Generation || statusAfterRunRollback != "cancelled" {
		t.Fatalf("failed run creation polluted current job: generation=%d status=%s", generationAfterRunRollback, statusAfterRunRollback)
	}
	if _, err = db.Exec(`DROP TRIGGER fail_test_paper_run_insert ON paper_import_run`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`DROP FUNCTION fail_test_paper_run_insert()`); err != nil {
		t.Fatal(err)
	}
	restarted, err := paperStore.ReplacePaperImportSources(context.Background(), tenantID, job.ID, userID, paper.ReplacePaperImportSourcesInput{CommandID: "paper-rerun-" + uuid.NewString(), ExpectedGeneration: job.Generation, Sources: []paper.ReplacePaperImportSourceInput{{ID: job.Sources[0].ID, DocumentIndex: 0, RoleHint: "question"}}})
	if err != nil {
		t.Fatalf("rerun same source: %v", err)
	}
	if restarted.Generation != 2 || restarted.RunID == job.RunID || restarted.Status != "processing" {
		t.Fatalf("rerun identity=%#v", restarted)
	}
	if err := paperStore.FailPendingPaperImportDispatch(context.Background(), claimedDispatch, "initial-dispatch", "old source download failed"); !errors.Is(err, paper.ErrConflict) {
		t.Fatalf("stale preparation failure accepted: %v", err)
	}
	err = paperStore.CompletePaperImportDecode(context.Background(), tenantID, job.ID, paper.PaperImportDecodeResult{TaskID: claimed[0].ID, LeaseToken: claimed[0].LeaseToken, DurationMS: 1, Pages: []paper.PaperImportDecodedPage{{SourceID: job.Sources[0].ID, DocumentIndex: 0, PageNo: 1, FileAssetID: fileID}}})
	if !errors.Is(err, paper.ErrConflict) {
		t.Fatalf("old generation callback error=%v", err)
	}
	current, err := paperStore.GetPaperImport(context.Background(), tenantID, job.ID)
	if err != nil || current.Generation != 2 || current.ResultGeneration != 0 || current.Status != "processing" {
		t.Fatalf("old callback polluted current import: %#v err=%v", current, err)
	}
	legacyTask, err := runtimeStore.CreateTask(context.Background(), tenantID, userID, workerruntime.CreateTaskInput{TaskType: "layout", QueueName: "page-processing", SourceType: "paper_import_job", SourceID: job.ID, Priority: 10, PayloadSchemaVersion: "paper-import-decode-v1", IdempotencyKey: "legacy-protocol-" + uuid.NewString(), MaxAttempts: 1})
	if err != nil {
		t.Fatalf("create legacy protocol task: %v", err)
	}
	legacyClaim, err := runtimeStore.Claim(context.Background(), tenantID, workerruntime.ClaimInput{QueueName: "page-processing", WorkerService: "legacy-test", WorkerInstanceID: "legacy-worker", Limit: 1, LeaseSeconds: 300})
	if err != nil || len(legacyClaim) != 1 || legacyClaim[0].ID != legacyTask.ID {
		t.Fatalf("claim legacy protocol task=%#v err=%v", legacyClaim, err)
	}
	err = paperStore.CompletePaperImportDecode(context.Background(), tenantID, job.ID, paper.PaperImportDecodeResult{TaskID: legacyClaim[0].ID, LeaseToken: legacyClaim[0].LeaseToken, DurationMS: 1, Pages: []paper.PaperImportDecodedPage{{SourceID: current.Sources[0].ID, DocumentIndex: 0, PageNo: 1, FileAssetID: fileID}}})
	if !errors.Is(err, paper.ErrConflict) {
		t.Fatalf("protocol-v1 callback error=%v", err)
	}
	if _, err = paperStore.CompletePaperImportCandidates(context.Background(), tenantID, job.ID, nil, []paper.QuestionCandidate{{CandidateID: "legacy-unversioned-result"}}, nil, nil, nil, nil); !errors.Is(err, paper.ErrConflict) {
		t.Fatalf("legacy unversioned candidate write error=%v", err)
	}
	afterLegacyWrite, err := paperStore.GetPaperImport(context.Background(), tenantID, job.ID)
	if err != nil || afterLegacyWrite.Generation != current.Generation || afterLegacyWrite.Status != "processing" || len(afterLegacyWrite.QuestionCandidates) != 0 {
		t.Fatalf("legacy unversioned write polluted current import: %#v err=%v", afterLegacyWrite, err)
	}
	failureTask, err := runtimeStore.CreateTask(context.Background(), tenantID, userID, workerruntime.CreateTaskInput{TaskType: "layout", QueueName: "page-processing", SourceType: "paper_import_job", SourceID: job.ID, Priority: 20, PayloadSchemaVersion: "paper-import-decode-v3", IdempotencyKey: "failure-atomicity-" + uuid.NewString(), MaxAttempts: 1})
	if err != nil {
		t.Fatalf("create failure atomicity task: %v", err)
	}
	if _, err = db.Exec(`UPDATE agent_worker_task SET paper_import_run_id=$3::uuid,paper_import_generation=$4,task_protocol_version=2 WHERE tenant_id=$1::uuid AND id=$2::uuid`, tenantID, failureTask.ID, current.RunID, current.Generation); err != nil {
		t.Fatal(err)
	}
	failureClaim, err := runtimeStore.Claim(context.Background(), tenantID, workerruntime.ClaimInput{QueueName: "page-processing", WorkerService: "failure-test", WorkerInstanceID: "failure-worker", Limit: 1, LeaseSeconds: 300})
	if err != nil || len(failureClaim) != 1 || failureClaim[0].ID != failureTask.ID {
		t.Fatalf("claim failure atomicity task=%#v err=%v", failureClaim, err)
	}
	if _, err = db.Exec(`CREATE FUNCTION fail_test_paper_runtime_failure() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF OLD.status='processing' AND NEW.status='failed' THEN RAISE EXCEPTION 'injected runtime failure'; END IF; RETURN NEW; END $$`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`CREATE TRIGGER fail_test_paper_runtime_failure BEFORE UPDATE ON paper_import_job FOR EACH ROW EXECUTE FUNCTION fail_test_paper_runtime_failure()`); err != nil {
		t.Fatal(err)
	}
	failureInput := paper.PaperImportRuntimeFailure{TaskID: failureTask.ID, LeaseToken: failureClaim[0].LeaseToken, Retryable: false, ErrorCode: "page_processing_failed", ErrorDetail: map[string]any{"injected": true}}
	if err = paperStore.FailPaperImportRuntime(context.Background(), tenantID, job.ID, failureInput); err == nil {
		t.Fatal("injected runtime failure unexpectedly committed")
	}
	var rolledBackTaskStatus, rolledBackJobStatus string
	if err = db.QueryRow(`SELECT task.status,job.status FROM agent_worker_task task CROSS JOIN paper_import_job job WHERE task.tenant_id=$1::uuid AND task.id=$2::uuid AND job.tenant_id=$1::uuid AND job.id=$3::uuid`, tenantID, failureTask.ID, job.ID).Scan(&rolledBackTaskStatus, &rolledBackJobStatus); err != nil {
		t.Fatal(err)
	}
	if rolledBackTaskStatus != "leased" || rolledBackJobStatus != "processing" {
		t.Fatalf("runtime failure was partially committed: task=%s job=%s", rolledBackTaskStatus, rolledBackJobStatus)
	}
	if _, err = db.Exec(`DROP TRIGGER fail_test_paper_runtime_failure ON paper_import_job`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`DROP FUNCTION fail_test_paper_runtime_failure()`); err != nil {
		t.Fatal(err)
	}
	if err = paperStore.FailPaperImportRuntime(context.Background(), tenantID, job.ID, failureInput); err != nil {
		t.Fatalf("commit runtime terminal failure after rollback: %v", err)
	}
	failedJob, err := paperStore.GetPaperImport(context.Background(), tenantID, job.ID)
	if err != nil || failedJob.Status != "failed" || failedJob.Generation != 2 {
		t.Fatalf("terminal failure state=%#v err=%v", failedJob, err)
	}
	retryCommand := "paper-retry-after-failure-" + uuid.NewString()
	retryInput := paper.ReplacePaperImportSourcesInput{CommandID: retryCommand, ExpectedGeneration: failedJob.Generation, Sources: []paper.ReplacePaperImportSourceInput{{ID: failedJob.Sources[0].ID, DocumentIndex: 0, RoleHint: "question"}}}
	retried, err := paperStore.ReplacePaperImportSources(context.Background(), tenantID, job.ID, userID, retryInput)
	if err != nil {
		t.Fatalf("retry terminal import: %v", err)
	}
	retriedReplay, err := paperStore.ReplacePaperImportSources(context.Background(), tenantID, job.ID, userID, retryInput)
	if err != nil || retried.Generation != 3 || retried.Status != "processing" || retried.RunID == failedJob.RunID || retriedReplay.RunID != retried.RunID || retriedReplay.Generation != retried.Generation {
		t.Fatalf("terminal retry/replay first=%#v replay=%#v err=%v", retried, retriedReplay, err)
	}
	raceTask, err := runtimeStore.CreateTask(context.Background(), tenantID, userID, workerruntime.CreateTaskInput{TaskType: "layout", QueueName: "page-processing", SourceType: "paper_import_job", SourceID: job.ID, Priority: 30, PayloadSchemaVersion: "paper-import-decode-v3", IdempotencyKey: "cancel-complete-race-" + uuid.NewString(), MaxAttempts: 1})
	if err != nil {
		t.Fatalf("create cancel/complete race task: %v", err)
	}
	if _, err = db.Exec(`UPDATE agent_worker_task SET paper_import_run_id=$3::uuid,paper_import_generation=$4,task_protocol_version=2 WHERE tenant_id=$1::uuid AND id=$2::uuid`, tenantID, raceTask.ID, retried.RunID, retried.Generation); err != nil {
		t.Fatal(err)
	}
	raceClaim, err := runtimeStore.Claim(context.Background(), tenantID, workerruntime.ClaimInput{QueueName: "page-processing", WorkerService: "race-test", WorkerInstanceID: "race-worker", Limit: 1, LeaseSeconds: 300})
	if err != nil || len(raceClaim) != 1 || raceClaim[0].ID != raceTask.ID {
		t.Fatalf("claim cancel/complete race task=%#v err=%v", raceClaim, err)
	}
	raceStart := make(chan struct{})
	raceResults := make(chan error, 2)
	go func() {
		<-raceStart
		raceResults <- paperStore.CompletePaperImportDecode(context.Background(), tenantID, job.ID, paper.PaperImportDecodeResult{TaskID: raceClaim[0].ID, LeaseToken: raceClaim[0].LeaseToken, DurationMS: 1, Pages: []paper.PaperImportDecodedPage{{SourceID: retried.Sources[0].ID, DocumentIndex: 0, PageNo: 1, FileAssetID: fileID}}})
	}()
	go func() {
		<-raceStart
		_, cancelErr := paperStore.CancelPaperImportGeneration(context.Background(), tenantID, job.ID, retried.Generation)
		raceResults <- cancelErr
	}()
	close(raceStart)
	for range 2 {
		raceErr := <-raceResults
		if raceErr != nil && !errors.Is(raceErr, paper.ErrConflict) {
			t.Fatalf("cancel/complete race returned an invalid outcome: %v", raceErr)
		}
	}
	var finalRaceStatus string
	var activeRaceTasks, publishedRaceResult int
	if err = db.QueryRow(`SELECT job.status,
(SELECT count(*) FROM agent_worker_task task WHERE task.tenant_id=job.tenant_id AND task.paper_import_run_id=$3::uuid AND task.status IN ('queued','leased','running')),
CASE WHEN job.result_generation=$4 THEN 1 ELSE 0 END
FROM paper_import_job job WHERE job.tenant_id=$1::uuid AND job.id=$2::uuid`, tenantID, job.ID, retried.RunID, retried.Generation).Scan(&finalRaceStatus, &activeRaceTasks, &publishedRaceResult); err != nil {
		t.Fatal(err)
	}
	if finalRaceStatus != "cancelled" || activeRaceTasks != 0 || publishedRaceResult != 0 {
		t.Fatalf("cancel/complete race left a partial state: status=%s active=%d published=%d", finalRaceStatus, activeRaceTasks, publishedRaceResult)
	}

	submissionID, pageID := uuid.NewString(), uuid.NewString()
	if _, err = db.Exec(`INSERT INTO submission(id,tenant_id,exam_id,candidate_no,source_type,status,collected_by) VALUES($1::uuid,$2::uuid,$3::uuid,$4,'pdf_upload','pages_uploaded',$5::uuid)`, submissionID, tenantID, first.Exams[0].ID, "projection-"+uuid.NewString(), userID); err != nil {
		t.Fatalf("seed projection submission: %v", err)
	}
	if _, err = db.Exec(`INSERT INTO submission_page(id,tenant_id,submission_id,file_asset_id,page_no,status) VALUES($1::uuid,$2::uuid,$3::uuid,$4::uuid,1,'accepted')`, pageID, tenantID, submissionID, fileID); err != nil {
		t.Fatalf("seed projection page: %v", err)
	}
	projectionStore := processing.NewPostgresStore(db)
	if err = projectionStore.RefreshExam(context.Background(), tenantID, first.Exams[0].ID); err != nil {
		t.Fatalf("initial projection: %v", err)
	}
	summary, err := projectionStore.Summary(context.Background(), tenantID, first.Exams[0].ID)
	if err != nil || summary.TotalPages != 1 {
		t.Fatalf("initial summary=%#v err=%v", summary, err)
	}
	var exceptionID string
	if err = db.QueryRow(`SELECT id::text FROM operational_exception WHERE tenant_id=$1::uuid AND page_id=$2::uuid AND status='open'`, tenantID, pageID).Scan(&exceptionID); err != nil {
		t.Fatalf("load projected exception: %v", err)
	}
	if _, err = projectionStore.AssignException(context.Background(), tenantID, exceptionID, userID, processing.AssignInput{AssigneeID: userID}); err != nil {
		t.Fatalf("assign projected exception: %v", err)
	}
	if err = projectionStore.RefreshExam(context.Background(), tenantID, first.Exams[0].ID); err != nil {
		t.Fatalf("repeat unchanged projection: %v", err)
	}
	assigned, err := projectionStore.GetException(context.Background(), tenantID, exceptionID)
	if err != nil || assigned.Status != "assigned" || assigned.AssignedTo != userID {
		t.Fatalf("unchanged refresh lost manual assignment: %#v err=%v", assigned, err)
	}
	if _, err = db.Exec(`UPDATE submission_page SET deleted_at=now(),updated_at=now() WHERE tenant_id=$1::uuid AND id=$2::uuid`, tenantID, pageID); err != nil {
		t.Fatal(err)
	}
	if err = projectionStore.RefreshExam(context.Background(), tenantID, first.Exams[0].ID); err != nil {
		t.Fatalf("projection after delete: %v", err)
	}
	summary, err = projectionStore.Summary(context.Background(), tenantID, first.Exams[0].ID)
	if err != nil || summary.TotalPages != 0 {
		t.Fatalf("deleted page remains in current summary=%#v err=%v", summary, err)
	}
	var staleState, unresolvedException int
	if err = db.QueryRow(`SELECT count(*),(SELECT count(*) FROM operational_exception WHERE tenant_id=$1::uuid AND page_id=$2::uuid AND status IN ('open','assigned')) FROM submission_page_processing_state WHERE tenant_id=$1::uuid AND page_id=$2::uuid`, tenantID, pageID).Scan(&staleState, &unresolvedException); err != nil {
		t.Fatal(err)
	}
	if staleState != 0 || unresolvedException != 0 {
		t.Fatalf("invalid projection facts remain: states=%d exceptions=%d", staleState, unresolvedException)
	}
	closed, err := projectionStore.GetException(context.Background(), tenantID, exceptionID)
	if err != nil || closed.Status != "resolved" || closed.Resolution != "source_deleted" {
		t.Fatalf("deleted source did not close assigned exception: %#v err=%v", closed, err)
	}

	resolvedSubmissionID, resolvedPageID := uuid.NewString(), uuid.NewString()
	if _, err = db.Exec(`INSERT INTO submission(id,tenant_id,exam_id,candidate_no,source_type,status,collected_by) VALUES($1::uuid,$2::uuid,$3::uuid,$4,'pdf_upload','pages_uploaded',$5::uuid)`, resolvedSubmissionID, tenantID, first.Exams[0].ID, "projection-resolved-"+uuid.NewString(), userID); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO submission_page(id,tenant_id,submission_id,file_asset_id,page_no,status) VALUES($1::uuid,$2::uuid,$3::uuid,$4::uuid,1,'accepted')`, resolvedPageID, tenantID, resolvedSubmissionID, fileID); err != nil {
		t.Fatal(err)
	}
	if err = projectionStore.RefreshExam(context.Background(), tenantID, first.Exams[0].ID); err != nil {
		t.Fatal(err)
	}
	var resolvedExceptionID string
	if err = db.QueryRow(`SELECT id::text FROM operational_exception WHERE tenant_id=$1::uuid AND page_id=$2::uuid AND status='open'`, tenantID, resolvedPageID).Scan(&resolvedExceptionID); err != nil {
		t.Fatal(err)
	}
	if _, err = projectionStore.ResolveException(context.Background(), tenantID, resolvedExceptionID, userID, processing.ResolveInput{Resolution: "operator_confirmed"}); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if _, err := db.Exec(`UPDATE submission_page SET updated_at=clock_timestamp() WHERE id=$1::uuid`, resolvedPageID); err != nil {
			t.Fatal(err)
		}
		if err = projectionStore.RefreshExam(context.Background(), tenantID, first.Exams[0].ID); err != nil {
			t.Fatal(err)
		}
	}
	resolvedException, err := projectionStore.GetException(context.Background(), tenantID, resolvedExceptionID)
	if err != nil || resolvedException.Status != "resolved" || resolvedException.Resolution != "operator_confirmed" {
		t.Fatalf("unchanged refresh lost manual resolution: %#v err=%v", resolvedException, err)
	}
	if _, err = db.Exec(`UPDATE submission SET deleted_at=now(),updated_at=now() WHERE tenant_id=$1::uuid AND id=$2::uuid`, tenantID, resolvedSubmissionID); err != nil {
		t.Fatal(err)
	}
	if err = projectionStore.RefreshExam(context.Background(), tenantID, first.Exams[0].ID); err != nil {
		t.Fatal(err)
	}

	movedSubmissionID, movedPageID := uuid.NewString(), uuid.NewString()
	if _, err = db.Exec(`INSERT INTO submission(id,tenant_id,exam_id,candidate_no,source_type,status,collected_by) VALUES($1::uuid,$2::uuid,$3::uuid,$4,'pdf_upload','pages_uploaded',$5::uuid)`, movedSubmissionID, tenantID, first.Exams[0].ID, "projection-move-"+uuid.NewString(), userID); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO submission_page(id,tenant_id,submission_id,file_asset_id,page_no,status) VALUES($1::uuid,$2::uuid,$3::uuid,$4::uuid,1,'accepted')`, movedPageID, tenantID, movedSubmissionID, fileID); err != nil {
		t.Fatal(err)
	}
	if err = projectionStore.RefreshExam(context.Background(), tenantID, first.Exams[0].ID); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`UPDATE submission SET exam_id=$3::uuid,updated_at=now() WHERE tenant_id=$1::uuid AND id=$2::uuid`, tenantID, movedSubmissionID, separate.Exams[0].ID); err != nil {
		t.Fatal(err)
	}
	// Destination-first processing must not hide the old exception behind the
	// page state's new exam ownership.
	if err = projectionStore.RefreshExam(context.Background(), tenantID, separate.Exams[0].ID); err != nil {
		t.Fatal(err)
	}
	if err = projectionStore.RefreshExam(context.Background(), tenantID, first.Exams[0].ID); err != nil {
		t.Fatal(err)
	}
	var oldOpen, newOpen, oldClosed int
	if err := db.QueryRow(`SELECT count(*) FILTER(WHERE exam_id=$3::uuid AND status IN ('open','assigned')),count(*) FILTER(WHERE exam_id=$4::uuid AND status='open'),count(*) FILTER(WHERE exam_id=$3::uuid AND status='resolved' AND resolution='source_reassigned') FROM operational_exception WHERE tenant_id=$1::uuid AND page_id=$2::uuid`, tenantID, movedPageID, first.Exams[0].ID, separate.Exams[0].ID).Scan(&oldOpen, &newOpen, &oldClosed); err != nil {
		t.Fatal(err)
	}
	if oldOpen != 0 || newOpen != 1 || oldClosed != 1 {
		t.Fatalf("destination-first lifecycle: old open=%d new open=%d old history=%d", oldOpen, newOpen, oldClosed)
	}
	oldSummary, oldErr := projectionStore.Summary(context.Background(), tenantID, first.Exams[0].ID)
	newSummary, newErr := projectionStore.Summary(context.Background(), tenantID, separate.Exams[0].ID)
	if oldErr != nil || newErr != nil || oldSummary.TotalPages != 0 || newSummary.TotalPages != 1 {
		t.Fatalf("moved page projection old=%#v/%v new=%#v/%v", oldSummary, oldErr, newSummary, newErr)
	}
	regroupedSubmissionID := uuid.NewString()
	if _, err = db.Exec(`INSERT INTO submission(id,tenant_id,exam_id,candidate_no,source_type,status,collected_by) VALUES($1::uuid,$2::uuid,$3::uuid,$4,'pdf_upload','pages_uploaded',$5::uuid)`, regroupedSubmissionID, tenantID, separate.Exams[0].ID, "projection-regroup-"+uuid.NewString(), userID); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`UPDATE submission_page SET submission_id=$3::uuid,updated_at=now() WHERE tenant_id=$1::uuid AND id=$2::uuid`, tenantID, movedPageID, regroupedSubmissionID); err != nil {
		t.Fatal(err)
	}
	if err = projectionStore.RefreshExam(context.Background(), tenantID, separate.Exams[0].ID); err != nil {
		t.Fatal(err)
	}
	regroupedSummary, regroupedErr := projectionStore.Summary(context.Background(), tenantID, separate.Exams[0].ID)
	if regroupedErr != nil || regroupedSummary.TotalPages != 1 {
		t.Fatalf("page regroup duplicated projection: %#v err=%v", regroupedSummary, regroupedErr)
	}
	deletedSubmissionID, deletedSubmissionPageID := uuid.NewString(), uuid.NewString()
	if _, err = db.Exec(`INSERT INTO submission(id,tenant_id,exam_id,candidate_no,source_type,status,collected_by) VALUES($1::uuid,$2::uuid,$3::uuid,$4,'pdf_upload','pages_uploaded',$5::uuid)`, deletedSubmissionID, tenantID, separate.Exams[0].ID, "projection-submission-delete-"+uuid.NewString(), userID); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO submission_page(id,tenant_id,submission_id,file_asset_id,page_no,status) VALUES($1::uuid,$2::uuid,$3::uuid,$4::uuid,1,'accepted')`, deletedSubmissionPageID, tenantID, deletedSubmissionID, fileID); err != nil {
		t.Fatal(err)
	}
	if err = projectionStore.RefreshExam(context.Background(), tenantID, separate.Exams[0].ID); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`UPDATE submission SET deleted_at=now(),updated_at=now() WHERE tenant_id=$1::uuid AND id=$2::uuid`, tenantID, deletedSubmissionID); err != nil {
		t.Fatal(err)
	}
	if err = projectionStore.RefreshExam(context.Background(), tenantID, separate.Exams[0].ID); err != nil {
		t.Fatal(err)
	}
	deletedSubmissionSummary, deletedSubmissionErr := projectionStore.Summary(context.Background(), tenantID, separate.Exams[0].ID)
	var deletedSubmissionState int
	if err = db.QueryRow(`SELECT count(*) FROM submission_page_processing_state WHERE tenant_id=$1::uuid AND page_id=$2::uuid`, tenantID, deletedSubmissionPageID).Scan(&deletedSubmissionState); err != nil {
		t.Fatal(err)
	}
	if deletedSubmissionErr != nil || deletedSubmissionSummary.TotalPages != 1 || deletedSubmissionState != 0 {
		t.Fatalf("deleted submission remains projected: summary=%#v state=%d err=%v", deletedSubmissionSummary, deletedSubmissionState, deletedSubmissionErr)
	}

	// A claimed projector acknowledges only its claimed version. A request
	// arriving after the claim remains pending and an unrelated owner cannot
	// publish or acknowledge the leased work.
	if _, err = db.Exec(`UPDATE processing_projection_cursor SET projected_version=requested_version,lease_owner=NULL,lease_expires_at=NULL,available_at=now()`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`SELECT request_processing_projection_refresh($1::uuid,$2::uuid)`, tenantID, first.Exams[0].ID); err != nil {
		t.Fatal(err)
	}
	claimedProjection, ok, err := projectionStore.ClaimProjection(context.Background(), "projector-a", 5*time.Minute)
	if err != nil || !ok || claimedProjection.ExamID != first.Exams[0].ID {
		t.Fatalf("claim projection=%#v ok=%v err=%v", claimedProjection, ok, err)
	}
	if _, err = db.Exec(`SELECT request_processing_projection_refresh($1::uuid,$2::uuid)`, tenantID, first.Exams[0].ID); err != nil {
		t.Fatal(err)
	}
	if err = projectionStore.ApplyProjection(context.Background(), "projector-b", claimedProjection); !errors.Is(err, processing.ErrProjectionLeaseLost) {
		t.Fatalf("foreign projector lease error=%v", err)
	}
	if err = projectionStore.ApplyProjection(context.Background(), "projector-a", claimedProjection); err != nil {
		t.Fatalf("apply claimed projection: %v", err)
	}
	var requestedVersion, projectedVersion int64
	if err = db.QueryRow(`SELECT requested_version,projected_version FROM processing_projection_cursor WHERE tenant_id=$1::uuid AND exam_id=$2::uuid`, tenantID, first.Exams[0].ID).Scan(&requestedVersion, &projectedVersion); err != nil {
		t.Fatal(err)
	}
	if projectedVersion != claimedProjection.RequestedVersion || requestedVersion <= projectedVersion {
		t.Fatalf("new projection request was incorrectly acknowledged: requested=%d projected=%d claimed=%d", requestedVersion, projectedVersion, claimedProjection.RequestedVersion)
	}
	nextProjection, ok, err := projectionStore.ClaimProjection(context.Background(), "projector-b", 5*time.Minute)
	if err != nil || !ok || nextProjection.ExamID != first.Exams[0].ID {
		t.Fatalf("reclaim pending projection=%#v ok=%v err=%v", nextProjection, ok, err)
	}
	if err = projectionStore.ApplyProjection(context.Background(), "projector-b", nextProjection); err != nil {
		t.Fatalf("apply pending projection: %v", err)
	}
	t.Run("PROJ expired and same-owner reclaimed leases are fenced", func(t *testing.T) {
		ctx := context.Background()
		if _, err := db.Exec(`SELECT request_processing_projection_refresh($1::uuid,$2::uuid)`, tenantID, first.Exams[0].ID); err != nil {
			t.Fatal(err)
		}
		old, ok, err := projectionStore.ClaimProjection(ctx, "same-owner", time.Minute)
		if err != nil || !ok {
			t.Fatalf("claim: %v %v", ok, err)
		}
		if _, err := db.Exec(`UPDATE processing_projection_cursor SET lease_expires_at=now()-interval '1 second' WHERE tenant_id=$1::uuid AND exam_id=$2::uuid`, old.TenantID, old.ExamID); err != nil {
			t.Fatal(err)
		}
		if err := projectionStore.ApplyProjection(ctx, "same-owner", old); !errors.Is(err, processing.ErrProjectionLeaseLost) {
			t.Fatalf("expired apply: %v", err)
		}
		if err := projectionStore.FailProjection(ctx, "same-owner", old, "stale", time.Second); !errors.Is(err, processing.ErrProjectionLeaseLost) {
			t.Fatalf("expired fail: %v", err)
		}
		fresh, ok, err := projectionStore.ClaimProjection(ctx, "same-owner", time.Minute)
		if err != nil || !ok {
			t.Fatalf("reclaim: %v %v", ok, err)
		}
		if err := projectionStore.ApplyProjection(ctx, "same-owner", old); !errors.Is(err, processing.ErrProjectionLeaseLost) {
			t.Fatalf("old claim reused new lease: %v", err)
		}
		if err := projectionStore.FailProjection(ctx, "same-owner", old, "stale", time.Second); !errors.Is(err, processing.ErrProjectionLeaseLost) {
			t.Fatalf("old failure cleared new lease: %v", err)
		}
		if err := projectionStore.ApplyProjection(ctx, "same-owner", fresh); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("IMP durable preparation retries and exhaustion", func(t *testing.T) {
		ctx := context.Background()
		if _, err := db.Exec(`UPDATE paper_import_run SET dispatch_available_at=now()+interval '1 day' WHERE status='processing'`); err != nil {
			t.Fatal(err)
		}
		service := paper.NewDocumentImportService(paperStore, nil, nil, "", "", time.Second)
		input := createImport
		input.CommandID = "dispatch-recovery-" + uuid.NewString()
		prepared, err := service.Start(ctx, tenantID, first.Exams[0].ID, userID, input)
		if err != nil || prepared.Status != "processing" {
			t.Fatalf("accept must persist intent without performing file I/O: %#v %v", prepared, err)
		}
		for attempt := 1; attempt <= 3; attempt++ {
			owner := "dispatch-" + uuid.NewString()
			claimed, ok, err := paperStore.ClaimPendingPaperImportDispatch(ctx, owner, time.Minute)
			if err != nil || !ok || claimed.RunID != prepared.RunID {
				t.Fatalf("claim %d: %#v %v %v", attempt, claimed, ok, err)
			}
			if attempt == 1 {
				if _, err := db.Exec(`UPDATE paper_import_run SET dispatch_lease_expires_at=now()-interval '1 second' WHERE id=$1::uuid`, prepared.RunID); err != nil {
					t.Fatal(err)
				}
				if err := paperStore.FailPendingPaperImportDispatch(ctx, claimed, owner, "expired"); !errors.Is(err, paper.ErrConflict) {
					t.Fatalf("expired preparation failure: %v", err)
				}
				assets := []paper.PaperImportOCRAsset{{SourceID: claimed.Sources[0].ID, DocumentIndex: claimed.Sources[0].DocumentIndex, RoleHint: claimed.Sources[0].RoleHint, FileAssetID: claimed.Sources[0].FileAssetID, ContentType: "application/pdf"}}
				if err := paperStore.QueuePaperImportOCR(ctx, tenantID, claimed, userID, assets); !errors.Is(err, paper.ErrConflict) {
					t.Fatalf("expired preparation scheduled a task: %v", err)
				}
				if _, err := db.Exec(`UPDATE paper_import_run SET dispatch_lease_expires_at=now()+interval '1 minute' WHERE id=$1::uuid`, prepared.RunID); err != nil {
					t.Fatal(err)
				}
			}
			if err := paperStore.FailPendingPaperImportDispatch(ctx, claimed, owner, "download unavailable"); err != nil {
				t.Fatal(err)
			}
			var jobStatus, runStatus, dispatchStatus string
			var count int
			var generation int64
			var backedOff bool
			if err := db.QueryRow(`SELECT j.status,r.status,r.dispatch_status,r.dispatch_attempt_count,j.current_generation,r.dispatch_available_at>now() FROM paper_import_job j JOIN paper_import_run r ON r.id=$2::uuid WHERE j.id=$1::uuid`, prepared.ID, prepared.RunID).Scan(&jobStatus, &runStatus, &dispatchStatus, &count, &generation, &backedOff); err != nil {
				t.Fatal(err)
			}
			want := "processing"
			if attempt == 3 {
				want = "failed"
			}
			if jobStatus != want || runStatus != want || dispatchStatus != "failed" || count != attempt || generation != prepared.Generation || !backedOff {
				t.Fatalf("attempt %d: job=%s run=%s dispatch=%s count=%d generation=%d backoff=%v", attempt, jobStatus, runStatus, dispatchStatus, count, generation, backedOff)
			}
			if _, err := db.Exec(`UPDATE paper_import_run SET dispatch_available_at=now() WHERE id=$1::uuid`, prepared.RunID); err != nil {
				t.Fatal(err)
			}
		}
		if _, ok, err := paperStore.ClaimPendingPaperImportDispatch(ctx, "after-exhaustion", time.Minute); err != nil || ok {
			t.Fatalf("exhausted preparation reclaimed: %v %v", ok, err)
		}
	})
	t.Run("IMP abandoned preparation lease retries then exhausts atomically", func(t *testing.T) {
		ctx := context.Background()
		if _, err := db.Exec(`UPDATE paper_import_run SET dispatch_available_at=now()+interval '1 day' WHERE status='processing'`); err != nil {
			t.Fatal(err)
		}
		service := paper.NewDocumentImportService(paperStore, nil, nil, "", "", time.Second)
		input := createImport
		input.CommandID = "abandoned-dispatch-" + uuid.NewString()
		prepared, err := service.Start(ctx, tenantID, first.Exams[0].ID, userID, input)
		if err != nil {
			t.Fatal(err)
		}
		var lastClaim paper.PaperImportJob
		var lastOwner string
		for attempt := 1; attempt <= 3; attempt++ {
			owner := "crashed-dispatch-" + uuid.NewString()
			claimed, ok, err := paperStore.ClaimPendingPaperImportDispatch(ctx, owner, time.Minute)
			if err != nil || !ok || claimed.RunID != prepared.RunID {
				t.Fatalf("abandoned claim %d: %#v ok=%v err=%v", attempt, claimed, ok, err)
			}
			lastClaim, lastOwner = claimed, owner
			if _, err := db.Exec(`UPDATE paper_import_run SET dispatch_lease_expires_at=now()-interval '1 second' WHERE id=$1::uuid`, prepared.RunID); err != nil {
				t.Fatal(err)
			}
			if attempt == 3 {
				if _, err := db.Exec(`CREATE FUNCTION reject_dispatch_exhaustion_test() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.error_code='paper_import_dispatch_exhausted' THEN RAISE EXCEPTION 'injected dispatch exhaustion failure'; END IF; RETURN NEW; END $$; CREATE TRIGGER reject_dispatch_exhaustion_test BEFORE UPDATE ON paper_import_job FOR EACH ROW EXECUTE FUNCTION reject_dispatch_exhaustion_test()`); err != nil {
					t.Fatal(err)
				}
				if _, err := paperStore.ReconcileExpiredPaperImportTask(ctx); err == nil {
					t.Fatal("dispatch exhaustion injection did not fail")
				}
				var jobStatus, runStatus, leaseOwner, sourceStatus string
				if err := db.QueryRow(`SELECT j.status,r.status,COALESCE(r.dispatch_lease_owner,''),s.processing_status
FROM paper_import_job j JOIN paper_import_run r ON r.id=$2::uuid JOIN paper_import_source s ON s.paper_import_id=j.id AND s.deleted_at IS NULL
WHERE j.id=$1::uuid LIMIT 1`, prepared.ID, prepared.RunID).Scan(&jobStatus, &runStatus, &leaseOwner, &sourceStatus); err != nil {
					t.Fatal(err)
				}
				if jobStatus != "processing" || runStatus != "processing" || leaseOwner != owner || sourceStatus == "failed" {
					t.Fatalf("partial dispatch exhaustion: job=%s run=%s owner=%s source=%s", jobStatus, runStatus, leaseOwner, sourceStatus)
				}
				if _, err := db.Exec(`DROP TRIGGER reject_dispatch_exhaustion_test ON paper_import_job; DROP FUNCTION reject_dispatch_exhaustion_test()`); err != nil {
					t.Fatal(err)
				}
			}
			fresh := paper.NewPostgresStore(db)
			if repaired, err := fresh.ReconcileExpiredPaperImportTask(ctx); err != nil || !repaired {
				t.Fatalf("abandoned reconcile %d: repaired=%v err=%v", attempt, repaired, err)
			}
			if err := paperStore.FailPendingPaperImportDispatch(ctx, claimed, owner, "late crashed owner callback"); !errors.Is(err, paper.ErrConflict) {
				t.Fatalf("late dispatch owner %d was not fenced: %v", attempt, err)
			}
			if attempt < 3 {
				var jobStatus, runStatus, dispatchStatus string
				var attempts int
				var backedOff bool
				if err := db.QueryRow(`SELECT j.status,r.status,r.dispatch_status,r.dispatch_attempt_count,r.dispatch_available_at>now()
FROM paper_import_job j JOIN paper_import_run r ON r.id=$2::uuid WHERE j.id=$1::uuid`, prepared.ID, prepared.RunID).Scan(&jobStatus, &runStatus, &dispatchStatus, &attempts, &backedOff); err != nil {
					t.Fatal(err)
				}
				if jobStatus != "processing" || runStatus != "processing" || dispatchStatus != "failed" || attempts != attempt || !backedOff {
					t.Fatalf("abandoned retry %d: job=%s run=%s dispatch=%s attempts=%d backoff=%v", attempt, jobStatus, runStatus, dispatchStatus, attempts, backedOff)
				}
				if _, err := db.Exec(`UPDATE paper_import_run SET dispatch_available_at=now() WHERE id=$1::uuid`, prepared.RunID); err != nil {
					t.Fatal(err)
				}
			}
		}
		var jobStatus, runStatus, dispatchStatus, jobCode, runCode, leaseOwner, sourceStatus string
		var attempts int
		if err := db.QueryRow(`SELECT j.status,r.status,r.dispatch_status,COALESCE(j.error_code,''),COALESCE(r.error_code,''),COALESCE(r.dispatch_lease_owner,''),r.dispatch_attempt_count,s.processing_status
FROM paper_import_job j JOIN paper_import_run r ON r.id=$2::uuid JOIN paper_import_source s ON s.paper_import_id=j.id AND s.deleted_at IS NULL
WHERE j.id=$1::uuid LIMIT 1`, prepared.ID, prepared.RunID).Scan(&jobStatus, &runStatus, &dispatchStatus, &jobCode, &runCode, &leaseOwner, &attempts, &sourceStatus); err != nil {
			t.Fatal(err)
		}
		if jobStatus != "failed" || runStatus != "failed" || dispatchStatus != "failed" || jobCode != "paper_import_dispatch_exhausted" || runCode != jobCode || leaseOwner != "" || attempts != 3 || sourceStatus != "failed" {
			t.Fatalf("abandoned terminal state: job=%s run=%s dispatch=%s codes=%s/%s owner=%s attempts=%d source=%s", jobStatus, runStatus, dispatchStatus, jobCode, runCode, leaseOwner, attempts, sourceStatus)
		}
		if _, ok, err := paperStore.ClaimPendingPaperImportDispatch(ctx, "fourth-attempt", time.Minute); err != nil || ok {
			t.Fatalf("exhausted abandoned dispatch reclaimed: ok=%v err=%v", ok, err)
		}
		if err := paperStore.FailPendingPaperImportDispatch(ctx, lastClaim, lastOwner, "very late callback"); !errors.Is(err, paper.ErrConflict) {
			t.Fatalf("terminal run accepted late owner: %v", err)
		}
	})
	t.Run("IMP final abandoned attempt atomically fails its run", func(t *testing.T) {
		ctx := context.Background()
		input := createImport
		input.CommandID = "abandoned-final-" + uuid.NewString()
		job, err := paperStore.CreatePaperImport(ctx, tenantID, first.Exams[0].ID, userID, input)
		if err != nil {
			t.Fatal(err)
		}
		assets := []paper.PaperImportOCRAsset{{SourceID: job.Sources[0].ID, DocumentIndex: job.Sources[0].DocumentIndex, RoleHint: job.Sources[0].RoleHint, FileAssetID: job.Sources[0].FileAssetID, ContentType: "application/pdf"}}
		if err := paperStore.QueuePaperImportOCR(ctx, tenantID, job, userID, assets); err != nil {
			t.Fatal(err)
		}
		var taskID string
		if err := db.QueryRow(`UPDATE agent_worker_task SET max_attempts=1,queue_name='audit-expired-import' WHERE paper_import_run_id=$1::uuid RETURNING id::text`, job.RunID).Scan(&taskID); err != nil {
			t.Fatal(err)
		}
		tasks, err := runtimeStore.Claim(ctx, tenantID, workerruntime.ClaimInput{QueueName: "audit-expired-import", WorkerService: "audit", WorkerInstanceID: "crashed", Limit: 1, LeaseSeconds: 60})
		if err != nil || len(tasks) != 1 {
			t.Fatalf("claim: %#v %v", tasks, err)
		}
		if _, err := db.Exec(`UPDATE agent_worker_task SET lease_expires_at=now()-interval '1 second' WHERE id=$1::uuid`, taskID); err != nil {
			t.Fatal(err)
		}
		if _, err := runtimeStore.Claim(ctx, tenantID, workerruntime.ClaimInput{QueueName: "audit-expired-import", WorkerService: "audit", WorkerInstanceID: "new", Limit: 1, LeaseSeconds: 60}); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`CREATE FUNCTION reject_exhaustion_test() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.error_code='paper_import_attempts_exhausted' THEN RAISE EXCEPTION 'injected exhaustion failure'; END IF; RETURN NEW; END $$; CREATE TRIGGER reject_exhaustion_test BEFORE UPDATE ON paper_import_job FOR EACH ROW EXECUTE FUNCTION reject_exhaustion_test()`); err != nil {
			t.Fatal(err)
		}
		if _, err := paperStore.ReconcileExpiredPaperImportTask(ctx); err == nil {
			t.Fatal("exhaustion injection did not fail")
		}
		var taskStatus, jobStatus, attemptStatus string
		read := func() {
			t.Helper()
			if err := db.QueryRow(`SELECT t.status,j.status,a.status FROM agent_worker_task t JOIN paper_import_job j ON j.id=$2::uuid JOIN agent_worker_task_attempt a ON a.task_id=t.id AND a.attempt_no=t.attempt_count WHERE t.id=$1::uuid`, taskID, job.ID).Scan(&taskStatus, &jobStatus, &attemptStatus); err != nil {
				t.Fatal(err)
			}
		}
		read()
		if taskStatus != "leased" || jobStatus != "processing" || attemptStatus != "leased" {
			t.Fatalf("partial exhaustion: %s %s %s", taskStatus, jobStatus, attemptStatus)
		}
		if _, err := db.Exec(`DROP TRIGGER reject_exhaustion_test ON paper_import_job; DROP FUNCTION reject_exhaustion_test()`); err != nil {
			t.Fatal(err)
		}
		fresh := paper.NewPostgresStore(db)
		if repaired, err := fresh.ReconcileExpiredPaperImportTask(ctx); err != nil || !repaired {
			t.Fatalf("restart recovery: %v %v", repaired, err)
		}
		read()
		if taskStatus != "dead_letter" || jobStatus != "failed" || attemptStatus != "lease_expired" {
			t.Fatalf("incomplete exhaustion: %s %s %s", taskStatus, jobStatus, attemptStatus)
		}
		if repaired, err := fresh.ReconcileExpiredPaperImportTask(ctx); err != nil || repaired {
			t.Fatalf("duplicate recovery: %v %v", repaired, err)
		}
	})
	t.Run("PROJ production capture delete restore follows the effective page set", func(t *testing.T) {
		ctx := context.Background()
		examID := separate.Exams[0].ID
		if _, err := db.Exec(`UPDATE exam SET status='collecting' WHERE id=$1::uuid`, examID); err != nil {
			t.Fatal(err)
		}
		t.Run("capture batch durable command recovery", func(t *testing.T) {
			store := capture.NewPostgresStore(db)
			input := capture.CreateBatchInput{Name: "Command batch", SourceType: "web_upload", IdempotencyKey: uuid.NewString()}
			type result struct {
				batch capture.Batch
				err   error
			}
			start := make(chan struct{})
			results := make(chan result, 2)
			for range 2 {
				go func() {
					<-start
					batch, err := store.CreateBatch(ctx, tenantID, examID, userID, input)
					results <- result{batch, err}
				}()
			}
			close(start)
			left, right := <-results, <-results
			if left.err != nil || right.err != nil || left.batch.ID != right.batch.ID {
				t.Fatalf("concurrent command: %#v %#v", left, right)
			}
			changed := input
			changed.Name = "Changed request"
			if _, err := store.CreateBatch(ctx, tenantID, examID, userID, changed); !errors.Is(err, capture.ErrConflict) {
				t.Fatalf("changed request: %v", err)
			}
			if _, err := db.Exec(`UPDATE capture_batch SET deleted_at=now() WHERE tenant_id=$1::uuid AND id=$2::uuid`, tenantID, left.batch.ID); err != nil {
				t.Fatal(err)
			}
			fresh := capture.NewPostgresStore(db)
			replayed, err := fresh.CreateBatch(ctx, tenantID, examID, userID, input)
			if err != nil || replayed.ID != left.batch.ID {
				t.Fatalf("deleted batch command duplicated: %#v %v", replayed, err)
			}
			recovery, err := fresh.RecoverBatchCommand(ctx, tenantID, examID, userID, input.IdempotencyKey)
			if err != nil || recovery.Status != "succeeded" || recovery.Batch == nil || recovery.Batch.ID != left.batch.ID {
				t.Fatalf("recovery: %#v %v", recovery, err)
			}
			input.IdempotencyKey = uuid.NewString()
			newBatch, err := fresh.CreateBatch(ctx, tenantID, examID, userID, input)
			if err != nil || newBatch.ID == left.batch.ID {
				t.Fatalf("new command incorrectly merged: %#v %v", newBatch, err)
			}
			var count int
			if err := db.QueryRow(`SELECT count(*) FROM capture_batch WHERE tenant_id=$1::uuid AND name='Command batch'`, tenantID).Scan(&count); err != nil || count != 2 {
				t.Fatalf("command count=%d %v", count, err)
			}
			handler := capture.NewHandler(fresh, nil, exam.NewPostgresStore(db), workerruntime.NewPostgresStore(db), auth.NewPostgresStore(db))
			transport := &failCompleteOnceIdempotencyStore{Store: idempotency.NewPostgresStore(db)}
			mux := http.NewServeMux()
			mux.Handle("POST /api/v1/exams/{examId}/capture-batches", idempotency.Middleware(transport, idempotency.Options{Enforce: true, TTL: time.Hour, ProcessingTimeout: time.Nanosecond})(http.HandlerFunc(handler.CreateBatch)))
			commandInput := capture.CreateBatchInput{Name: "Lost batch response", SourceType: "web_upload", IdempotencyKey: uuid.NewString()}
			raw, _ := json.Marshal(commandInput)
			request := func() *http.Request {
				req := httptest.NewRequest("POST", "/api/v1/exams/"+examID+"/capture-batches", bytes.NewReader(raw))
				req.Header.Set("Idempotency-Key", commandInput.IdempotencyKey)
				return req.WithContext(auth.WithAccessScope(auth.WithUser(ctx, auth.User{ID: userID, TenantID: tenantID}), scope))
			}
			response := httptest.NewRecorder()
			mux.ServeHTTP(response, request())
			if response.Code != 503 {
				t.Fatalf("receipt fault: %d %s", response.Code, response.Body.String())
			}
			recoveredAfter503, err := fresh.RecoverBatchCommand(ctx, tenantID, examID, userID, commandInput.IdempotencyKey)
			if err != nil || recoveredAfter503.Status != "succeeded" || recoveredAfter503.Batch == nil {
				t.Fatalf("receipt failure lost business fact: %#v %v", recoveredAfter503, err)
			}
			if err := db.QueryRow(`SELECT count(*) FROM capture_batch WHERE tenant_id=$1::uuid AND operator_id=$2::uuid AND idempotency_key=$3`, tenantID, userID, commandInput.IdempotencyKey).Scan(&count); err != nil || count != 1 {
				t.Fatalf("503 command count=%d %v", count, err)
			}
			retry := httptest.NewRecorder()
			mux.ServeHTTP(retry, request())
			if retry.Code != http.StatusCreated || !strings.Contains(retry.Body.String(), recoveredAfter503.Batch.ID) {
				t.Fatalf("stale capture command did not recover committed batch: %d %s", retry.Code, retry.Body.String())
			}
			if err := db.QueryRow(`SELECT count(*) FROM capture_batch WHERE tenant_id=$1::uuid AND operator_id=$2::uuid AND idempotency_key=$3`, tenantID, userID, commandInput.IdempotencyKey).Scan(&count); err != nil || count != 1 {
				t.Fatalf("capture takeover duplicated batch: count=%d %v", count, err)
			}
		})
		before, err := projectionStore.Summary(ctx, tenantID, examID)
		if err != nil {
			t.Fatal(err)
		}
		captureStore := capture.NewPostgresStore(db)
		batch, err := captureStore.CreateBatch(ctx, tenantID, examID, userID, capture.CreateBatchInput{Name: "Projection lifecycle", SourceType: "web_upload"})
		if err != nil {
			t.Fatal(err)
		}
		var subID, spID, cfID, cpID string
		if err := db.QueryRow(`INSERT INTO submission(tenant_id,exam_id,source_type,status,collected_by) VALUES($1::uuid,$2::uuid,'pdf_upload','pages_uploaded',$3::uuid) RETURNING id::text`, tenantID, examID, userID).Scan(&subID); err != nil {
			t.Fatal(err)
		}
		if err := db.QueryRow(`INSERT INTO submission_page(tenant_id,submission_id,file_asset_id,page_no,status) VALUES($1::uuid,$2::uuid,$3::uuid,1,'accepted') RETURNING id::text`, tenantID, subID, fileID).Scan(&spID); err != nil {
			t.Fatal(err)
		}
		if err := db.QueryRow(`INSERT INTO capture_file(tenant_id,capture_batch_id,file_asset_id,original_name,content_type,sha256,byte_size,idempotency_key,uploaded_by) VALUES($1::uuid,$2::uuid,$3::uuid,'lifecycle.pdf','application/pdf',repeat('a',64),1,'lifecycle-file',$4::uuid) RETURNING id::text`, tenantID, batch.ID, fileID, userID).Scan(&cfID); err != nil {
			t.Fatal(err)
		}
		if err := db.QueryRow(`INSERT INTO capture_page(tenant_id,capture_batch_id,capture_file_id,source_index,submission_id,submission_page_id,assigned_page_no,sequence_no,decoded_file_asset_id,status) VALUES($1::uuid,$2::uuid,$3::uuid,1,$4::uuid,$5::uuid,1,1,$6::uuid,'needs_review') RETURNING id::text`, tenantID, batch.ID, cfID, subID, spID, fileID).Scan(&cpID); err != nil {
			t.Fatal(err)
		}
		if err := projectionStore.RefreshExam(ctx, tenantID, examID); err != nil {
			t.Fatal(err)
		}
		active, err := projectionStore.Summary(ctx, tenantID, examID)
		if err != nil || active.TotalPages != before.TotalPages+1 {
			t.Fatalf("capture page not counted: %#v %v", active, err)
		}
		deleted, err := captureStore.DeletePage(ctx, tenantID, cpID, userID, capture.PageLifecycleInput{Revision: 1, Reason: "projection acceptance"})
		if err != nil {
			t.Fatal(err)
		}
		if err := projectionStore.RefreshExam(ctx, tenantID, examID); err != nil {
			t.Fatal(err)
		}
		after, err := projectionStore.Summary(ctx, tenantID, examID)
		if err != nil || after.TotalPages != before.TotalPages {
			t.Fatalf("deleted capture page remains counted: %#v %v", after, err)
		}
		if _, err := captureStore.RestorePage(ctx, tenantID, cpID, userID, capture.PageLifecycleInput{Revision: deleted.Revision, Reason: "restore acceptance"}); err != nil {
			t.Fatal(err)
		}
		if err := projectionStore.RefreshExam(ctx, tenantID, examID); err != nil {
			t.Fatal(err)
		}
		restored, err := projectionStore.Summary(ctx, tenantID, examID)
		if err != nil || restored.TotalPages != before.TotalPages+1 {
			t.Fatalf("restored capture page missing: %#v %v", restored, err)
		}
		var reopened int
		if err := db.QueryRow(`SELECT count(*) FROM operational_exception WHERE tenant_id=$1::uuid AND page_id=$2::uuid AND status='open'`, tenantID, spID).Scan(&reopened); err != nil || reopened != 1 {
			t.Fatalf("restored issue did not recur: %d %v", reopened, err)
		}
	})
	t.Run("IMP frozen exam rejects new commands but retains accepted command recovery", func(t *testing.T) {
		ctx := context.Background()
		current, err := paperStore.GetPaperImport(ctx, tenantID, job.ID)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`UPDATE exam SET status='collecting' WHERE tenant_id=$1::uuid AND id=$2::uuid`, tenantID, first.Exams[0].ID); err != nil {
			t.Fatal(err)
		}
		accepted, err := paperStore.CreatePaperImport(ctx, tenantID, first.Exams[0].ID, userID, createImport)
		if err != nil || accepted.ID != job.ID {
			t.Fatalf("frozen exam lost accepted command recovery: %#v %v", accepted, err)
		}
		newInput := createImport
		newInput.CommandID = uuid.NewString()
		if _, err := paperStore.CreatePaperImport(ctx, tenantID, first.Exams[0].ID, userID, newInput); !errors.Is(err, paper.ErrExamFrozen) {
			t.Fatalf("frozen exam accepted a new import: %v", err)
		}
		if _, err := paperStore.AddPaperImportSources(ctx, tenantID, job.ID, userID, paper.AddPaperImportSourcesInput{CommandID: uuid.NewString(), ExpectedGeneration: current.Generation, Sources: createImport.Sources}); !errors.Is(err, paper.ErrExamFrozen) {
			t.Fatalf("frozen exam accepted new sources: %v", err)
		}
		if _, err := paperStore.ReplacePaperImportSources(ctx, tenantID, job.ID, userID, paper.ReplacePaperImportSourcesInput{CommandID: uuid.NewString(), ExpectedGeneration: current.Generation, Sources: []paper.ReplacePaperImportSourceInput{{ID: current.Sources[0].ID, DocumentIndex: 0, RoleHint: "question"}}}); !errors.Is(err, paper.ErrExamFrozen) {
			t.Fatalf("frozen exam accepted rerun: %v", err)
		}
		unchanged, err := paperStore.GetPaperImport(ctx, tenantID, job.ID)
		if err != nil || unchanged.Generation != current.Generation || unchanged.Status != current.Status {
			t.Fatalf("rejected frozen commands changed current run: %#v %v", unchanged, err)
		}
	})
}

func TestReliabilityMigrationFrom000122PostgresTestDatabase(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("EDUGRADE_E2E_DATABASE_URL"))
	if dsn == "" {
		t.Skip("EDUGRADE_E2E_DATABASE_URL is not set outside the PostgreSQL CI job")
	}
	db := e2eOpenPostgresTestDB(t, dsn)
	e2eApplyPostgresMigrationsThrough(t, db, "000122_processing_projection_refresh.sql")
	tenantID, userID, schoolID, examID := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	if _, err := db.Exec(`INSERT INTO tenant(id,tenant_id,name,code,status) VALUES($1::uuid,$1::uuid,'Migration tenant',$2,'active')`, tenantID, "migration-"+uuid.NewString()); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO app_user(id,tenant_id,username,display_name,password_hash,status) VALUES($1::uuid,$2::uuid,'migration-admin','Migration admin','not-used','active')`, userID, tenantID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO school(id,tenant_id,name,code,status) VALUES($1::uuid,$2::uuid,'Migration school',$3,'active')`, schoolID, tenantID, "migration-school-"+uuid.NewString()); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO exam(id,tenant_id,school_id,name,subject,exam_type,total_score,status,grading_mode,appeal_enabled,publish_policy,created_by) VALUES($1::uuid,$2::uuid,$3::uuid,'Migration exam','math','formal_exam',100,'draft','ai_assisted',true,'after_admin_approval',$4::uuid)`, examID, tenantID, schoolID, userID); err != nil {
		t.Fatal(err)
	}
	reviewImportID, processingImportID := uuid.NewString(), uuid.NewString()
	draft := []byte(`[{"candidate_id":"q1","stem":"人工确认内容","human_confirmed_fields":["stem"]}]`)
	if _, err := db.Exec(`INSERT INTO paper_import_job(id,tenant_id,exam_id,status,subject,draft_questions,created_by) VALUES($1::uuid,$2::uuid,$3::uuid,'review_required','math',$4::jsonb,$5::uuid),($6::uuid,$2::uuid,$3::uuid,'processing','math','[]'::jsonb,$5::uuid)`, reviewImportID, tenantID, examID, string(draft), userID, processingImportID); err != nil {
		t.Fatal(err)
	}
	e2eApplyPostgresMigrations(t, db)
	gradeID, academicYearID, cohortID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	if _, err := db.Exec(`INSERT INTO academic_year(id,tenant_id,school_id,name,start_year,end_year,starts_at,ends_at,is_current,status) VALUES($1::uuid,$2::uuid,$3::uuid,'2026-2027',2026,2027,'2026-09-01','2027-08-31',true,'active')`, academicYearID, tenantID, schoolID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO grade_cohort(id,tenant_id,school_id,education_stage,entry_year,expected_graduation_year,name,status) VALUES($1::uuid,$2::uuid,$3::uuid,'senior',2026,2029,'Migration cohort','active')`, cohortID, tenantID, schoolID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO grade(id,tenant_id,school_id,name,level_no,academic_year,status,education_stage,academic_year_id,grade_cohort_id) VALUES($1::uuid,$2::uuid,$3::uuid,'Migration grade',10,'2026-2027','active','senior',$4::uuid,$5::uuid)`, gradeID, tenantID, schoolID, academicYearID, cohortID); err != nil {
		t.Fatal(err)
	}
	examCommandID := "repair-command-" + uuid.NewString()
	examStore := exam.NewPostgresStore(db)
	examSession, err := examStore.CreateExamSession(context.Background(), auth.AccessScope{TenantID: tenantID, TenantWide: true}, userID, exam.CreateSessionInput{
		SchoolID: schoolID, GradeID: gradeID, Name: "Repairable exam session", ExamType: "formal_exam",
		GradingMode: "ai_assisted", PublishPolicy: "after_admin_approval", CommandID: examCommandID,
		Subjects: []exam.SessionSubjectInput{{Subject: "math", TotalScore: 100, DurationMinutes: 90}},
	})
	if err != nil {
		t.Fatalf("seed durable exam command: %v", err)
	}
	transportRequestHash := "transport-" + uuid.NewString()
	if _, err = db.Exec(`INSERT INTO idempotency_record(tenant_id,actor_id,method,route,idempotency_key,request_hash,state,expires_at) VALUES($1::uuid,$2::uuid,'POST','/api/v1/exam-sessions',$3,$4,'processing',now()+interval '1 day')`, tenantID, userID, examCommandID, transportRequestHash); err != nil {
		t.Fatal(err)
	}
	var draftPreserved bool
	var reviewRunStatus, reviewRevision, processingJobStatus, processingRunStatus, processingError string
	if err := db.QueryRow(`SELECT job.draft_questions=$3::jsonb,job.source_revision,run.status FROM paper_import_job job JOIN paper_import_run run ON run.tenant_id=job.tenant_id AND run.paper_import_id=job.id AND run.generation=job.current_generation WHERE job.tenant_id=$1::uuid AND job.id=$2::uuid`, tenantID, reviewImportID, string(draft)).Scan(&draftPreserved, &reviewRevision, &reviewRunStatus); err != nil {
		t.Fatal(err)
	}
	if !draftPreserved || reviewRevision != "legacy-unverified" || reviewRunStatus != "review_required" {
		t.Fatalf("review history changed during migration: preserved=%v revision=%s run=%s", draftPreserved, reviewRevision, reviewRunStatus)
	}
	var historicalSnapshotUnknown bool
	if err := db.QueryRow(`SELECT source_snapshot IS NULL FROM paper_import_run WHERE paper_import_id=$1::uuid`, reviewImportID).Scan(&historicalSnapshotUnknown); err != nil || !historicalSnapshotUnknown {
		t.Fatalf("migration invented historical sources: %v %v", historicalSnapshotUnknown, err)
	}
	if err := db.QueryRow(`SELECT job.status,run.status,COALESCE(run.error_code,'') FROM paper_import_job job JOIN paper_import_run run ON run.tenant_id=job.tenant_id AND run.paper_import_id=job.id AND run.generation=job.current_generation WHERE job.tenant_id=$1::uuid AND job.id=$2::uuid`, tenantID, processingImportID).Scan(&processingJobStatus, &processingRunStatus, &processingError); err != nil {
		t.Fatal(err)
	}
	if processingJobStatus != "failed" || processingRunStatus != "failed" || processingError != "paper_import_protocol_upgrade_required" {
		t.Fatalf("unverifiable processing history was not safely stopped: job=%s run=%s error=%s", processingJobStatus, processingRunStatus, processingError)
	}

	var databaseName string
	if err := db.QueryRow(`SELECT current_database()`).Scan(&databaseName); err != nil {
		t.Fatal(err)
	}
	databaseURL, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	databaseURL.Path = "/" + databaseName
	repairBinary := filepath.Join(t.TempDir(), "reliability-repair.exe")
	build := exec.Command("go", "build", "-o", repairBinary, "../../cmd/reliability-repair")
	if output, buildErr := build.CombinedOutput(); buildErr != nil {
		t.Fatalf("build reliability repair tool: %v\n%s", buildErr, output)
	}
	runRepair := func(arguments ...string) string {
		t.Helper()
		base := []string{"--database-url", databaseURL.String(), "--tenant-id", tenantID}
		command := exec.Command(repairBinary, append(base, arguments...)...)
		output, runErr := command.CombinedOutput()
		if runErr != nil {
			t.Fatalf("run reliability repair tool: %v\n%s", runErr, output)
		}
		return string(output)
	}
	var repairFileID string
	if err := db.QueryRow(`INSERT INTO file_asset(tenant_id,exam_id,owner_type,owner_id,original_name,content_type,size_bytes,hash_sha256,storage_bucket,storage_key,visibility,uploaded_by) VALUES($1::uuid,$2::uuid,'exam',$2::uuid,'repair.pdf','application/pdf',1,repeat('a',64),'test','repair.pdf','private',$3::uuid) RETURNING id::text`, tenantID, examID, userID).Scan(&repairFileID); err != nil {
		t.Fatal(err)
	}
	paperStore := paper.NewPostgresStore(db)
	orphan, err := paperStore.CreatePaperImport(context.Background(), tenantID, examID, userID, paper.CreatePaperImportInput{CommandID: "repair-orphan-" + uuid.NewString(), Subject: "math", Sources: []paper.CreatePaperImportSourceInput{{FileAssetID: repairFileID, DocumentIndex: 0, RoleHint: "question"}}})
	if err != nil {
		t.Fatal(err)
	}
	assets := []paper.PaperImportOCRAsset{{SourceID: orphan.Sources[0].ID, FileAssetID: repairFileID, DocumentIndex: 0, RoleHint: "question", ContentType: "application/pdf"}}
	if err := paperStore.QueuePaperImportOCR(context.Background(), tenantID, orphan, userID, assets); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE agent_worker_task SET status='cancelled',cancelled_at=now(),completed_at=now() WHERE paper_import_run_id=$1::uuid`, orphan.RunID); err != nil {
		t.Fatal(err)
	}
	scopedDryRun := runRepair("--import-id", orphan.ID)
	if !strings.Contains(scopedDryRun, "create_recovery_generation") || strings.Contains(scopedDryRun, examCommandID) {
		t.Fatalf("repair scope/action mismatch: %s", scopedDryRun)
	}
	_ = runRepair("--import-id", orphan.ID, "--apply")
	repaired, err := paperStore.GetPaperImport(context.Background(), tenantID, orphan.ID)
	if err != nil || repaired.Generation != orphan.Generation+1 || repaired.RunID == orphan.RunID {
		t.Fatalf("terminal-only import not given new run: %#v %v", repaired, err)
	}
	if repeated := runRepair("--import-id", orphan.ID, "--apply"); !strings.Contains(repeated, `"mutations":0`) {
		t.Fatalf("duplicate recovery generation: %s", repeated)
	}
	var cancelledHistory int
	if err := db.QueryRow(`SELECT count(*) FROM agent_worker_task WHERE paper_import_run_id=$1::uuid AND status='cancelled'`, orphan.RunID).Scan(&cancelledHistory); err != nil || cancelledHistory != 1 {
		t.Fatalf("repair rewrote cancelled task history: %d %v", cancelledHistory, err)
	}
	dryRunOutput := runRepair()
	if !strings.Contains(dryRunOutput, `"mode": "dry-run"`) || !strings.Contains(dryRunOutput, reviewImportID) || !strings.Contains(dryRunOutput, examCommandID) || !strings.Contains(dryRunOutput, `"action": "complete_replay_from_business_fact"`) {
		t.Fatalf("repair dry-run did not report scoped findings: %s", dryRunOutput)
	}
	firstApplyOutput := runRepair("--apply")
	if !strings.Contains(firstApplyOutput, `"applied":true`) {
		t.Fatalf("repair apply did not report completion: %s", firstApplyOutput)
	}
	var replayState string
	var replayStatus int
	var replayRequestHash string
	var replayBody []byte
	if err := db.QueryRow(`SELECT state,response_status,request_hash,response_body FROM idempotency_record WHERE tenant_id=$1::uuid AND actor_id=$2::uuid AND method='POST' AND route='/api/v1/exam-sessions' AND idempotency_key=$3`, tenantID, userID, examCommandID).Scan(&replayState, &replayStatus, &replayRequestHash, &replayBody); err != nil {
		t.Fatal(err)
	}
	if replayState != "completed" || replayStatus != 201 || replayRequestHash != transportRequestHash || !strings.Contains(string(replayBody), examSession.ID) {
		t.Fatalf("exam replay receipt not repaired safely: state=%s status=%d hash=%s body=%s", replayState, replayStatus, replayRequestHash, replayBody)
	}
	var firstRepairStatus string
	var firstRepairUpdated time.Time
	if err := db.QueryRow(`SELECT status,updated_at FROM paper_import_run WHERE tenant_id=$1::uuid AND paper_import_id=$2::uuid`, tenantID, reviewImportID).Scan(&firstRepairStatus, &firstRepairUpdated); err != nil {
		t.Fatal(err)
	}
	if firstRepairStatus != "needs_review" {
		t.Fatalf("legacy candidate provenance was not marked for review: %s", firstRepairStatus)
	}
	secondApplyOutput := runRepair("--apply")
	if !strings.Contains(secondApplyOutput, `"mutations":0`) {
		t.Fatalf("second repair should be a no-op: %s", secondApplyOutput)
	}
	var secondRepairStatus string
	var secondRepairUpdated time.Time
	if err := db.QueryRow(`SELECT status,updated_at FROM paper_import_run WHERE tenant_id=$1::uuid AND paper_import_id=$2::uuid`, tenantID, reviewImportID).Scan(&secondRepairStatus, &secondRepairUpdated); err != nil {
		t.Fatal(err)
	}
	if secondRepairStatus != firstRepairStatus || !secondRepairUpdated.Equal(firstRepairUpdated) {
		t.Fatalf("second repair execution was not idempotent: first=%s/%s second=%s/%s", firstRepairStatus, firstRepairUpdated, secondRepairStatus, secondRepairUpdated)
	}
}
