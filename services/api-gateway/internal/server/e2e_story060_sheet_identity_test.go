package server

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"edugrade-enterprise/services/api-gateway/internal/capture"
)

func TestStory060MultiPageInsertionMissingPageAndPrintPackageE2EWithPostgresTestDatabase(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("EDUGRADE_E2E_DATABASE_URL"))
	if dsn == "" {
		t.Skip("EDUGRADE_E2E_DATABASE_URL is not set; skipping STORY-060 multi-page PostgreSQL acceptance workflow")
	}
	db := e2eOpenPostgresTestDB(t, dsn)
	e2eApplyPostgresMigrations(t, db)
	e2eActivatePostgresDemoUsers(t, db, []string{"tenant_admin"})
	router := e2ePostgresRouter(db)
	adminToken := e2eLoginWithTenant(t, router, "demo", "tenant_admin", "ChangeMe123!")
	suffix := strings.ReplaceAll(time.Now().UTC().Format("20060102150405.000000000"), ".", "")
	fixture := e2eCreateStory056AcceptanceFixture(t, db, router, adminToken, suffix)

	layout := story056JSON(t, map[string]any{"pages": []any{
		map[string]any{
			"page_no": 1, "width": 1000, "height": 1400,
			"registration_marks": []any{}, "identity_regions": []any{},
			"question_regions": []any{map[string]any{"label": "Q1", "x": 0.08, "y": 0.12, "width": 0.84, "height": 0.22}},
		},
		map[string]any{
			"page_no": 2, "width": 1000, "height": 1400,
			"registration_marks": []any{}, "identity_regions": []any{},
			"question_regions": []any{map[string]any{"label": "Q2", "x": 0.08, "y": 0.12, "width": 0.84, "height": 0.32}},
		},
		map[string]any{
			"page_no": 3, "width": 1000, "height": 1400,
			"registration_marks": []any{}, "identity_regions": []any{},
			"question_regions": []any{map[string]any{"label": "Q3", "x": 0.08, "y": 0.12, "width": 0.84, "height": 0.42}},
		},
	}})
	if _, err := db.Exec(`
UPDATE answer_sheet_template
SET page_count=3,layout=$3::jsonb,content_hash=$4,updated_at=now()
WHERE tenant_id=$1::uuid AND id=$2::uuid
`, fixture.TenantID, fixture.TemplateID, layout, "sha256:story060-three-page-"+suffix); err != nil {
		t.Fatalf("prepare three-page locked template: %v", err)
	}

	var studentID, classID string
	if err := db.QueryRow(`
SELECT st.id::text,st.class_id::text
FROM exam_class ec
JOIN student st
  ON st.tenant_id=ec.tenant_id AND st.class_id=ec.class_id
WHERE ec.tenant_id=$1::uuid AND ec.exam_id=$2::uuid
  AND ec.deleted_at IS NULL AND st.status='active' AND st.deleted_at IS NULL
ORDER BY st.created_at
LIMIT 1
`, fixture.TenantID, fixture.ExamID).Scan(&studentID, &classID); err != nil {
		t.Fatalf("lookup roster student: %v", err)
	}
	missingStudent := e2ePostJSON(t, router, http.MethodPost, "/api/v1/students", adminToken, story056JSON(t, map[string]any{
		"school_id":  fixture.SchoolID,
		"class_id":   classID,
		"student_no": "S060-MISSING-" + suffix,
		"name":       "STORY-060 Missing Page Student",
	}), http.StatusCreated)["student"].(map[string]any)
	missingStudentID := e2eString(t, missingStudent, "id")

	issue := func(label, targetStudentID string) (string, string, map[int]string) {
		t.Helper()
		issued := e2ePostJSON(
			t, router, http.MethodPost,
			"/api/v1/answer-sheet-templates/"+fixture.TemplateID+"/student-barcodes",
			adminToken,
			story056JSON(t, map[string]any{
				"student_ids":     []string{targetStudentID},
				"idempotency_key": "story060-" + label + "-" + suffix,
			}),
			http.StatusOK,
		)["barcodes"].(map[string]any)
		student := issued["students"].([]any)[0].(map[string]any)
		pageValues := map[int]string{}
		for _, raw := range student["pages"].([]any) {
			page := raw.(map[string]any)
			pageValues[int(e2eFloat(t, page, "page_no"))] = e2eString(t, page, "value")
		}
		if len(pageValues) != 3 {
			t.Fatalf("expected three immutable print pages: %#v", issued)
		}
		return e2eString(t, issued, "print_batch_id"), e2eString(t, student, "sheet_serial"), pageValues
	}

	printBatchID, insertedSheetSerial, insertedBarcodes := issue("inserted", studentID)
	printContext := e2eGetJSON(
		t, router,
		"/api/v1/answer-sheet-templates/"+fixture.TemplateID+"/print-context",
		adminToken, http.StatusOK,
	)["print_context"].(map[string]any)
	if e2eString(t, printContext, "template_id") != fixture.TemplateID ||
		e2eString(t, printContext, "exam_id") != fixture.ExamID {
		t.Fatalf("print context must stay scoped to the locked template: %#v", printContext)
	}
	var activeCandidateFound bool
	for _, raw := range printContext["candidates"].([]any) {
		candidate := raw.(map[string]any)
		if e2eString(t, candidate, "student_id") == studentID {
			activeCandidateFound = candidate["has_active_sheet"] == true &&
				e2eString(t, candidate, "active_print_batch_id") == printBatchID
		}
	}
	if !activeCandidateFound {
		t.Fatalf("issued student must be protected from accidental duplicate Web issuance: %#v", printContext["candidates"])
	}
	batches := printContext["batches"].([]any)
	if len(batches) == 0 ||
		e2eString(t, batches[0].(map[string]any), "print_batch_id") != printBatchID ||
		batches[0].(map[string]any)["downloadable"] != true {
		t.Fatalf("freshly issued print package must be recoverable after refresh: %#v", batches)
	}
	download := func() *httptest.ResponseRecorder {
		t.Helper()
		request := httptest.NewRequest(http.MethodGet, "/api/v1/answer-sheet-print-batches/"+printBatchID+"/package.pdf", nil)
		request.Header.Set("Authorization", "Bearer "+adminToken)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("download print package expected 200, got %d: %s", response.Code, response.Body.String())
		}
		return response
	}
	firstPackage := download()
	secondPackage := download()
	if firstPackage.Header().Get("Content-Type") != "application/pdf" ||
		firstPackage.Header().Get("X-EduGrade-SHA256") == "" ||
		!bytes.HasPrefix(firstPackage.Body.Bytes(), []byte("%PDF-")) ||
		!bytes.Equal(firstPackage.Body.Bytes(), secondPackage.Body.Bytes()) {
		t.Fatalf("print package must be a deterministic auditable PDF: headers=%v", firstPackage.Header())
	}

	store := capture.NewPostgresStoreWithBarcodeKeyring(db, e2eBarcodeKeyring())
	createCapture := func(label string, barcodes []string) string {
		t.Helper()
		assetID := e2eUploadSyntheticPDF(
			t, router, adminToken, "story060-"+label+"-"+suffix+".pdf",
			"%PDF-1.4\n% STORY-060 "+label+" "+suffix+"\n",
		)
		batch := e2ePostJSON(t, router, http.MethodPost, "/api/v1/exams/"+fixture.ExamID+"/capture-batches", adminToken, story056JSON(t, map[string]any{
			"name":            "STORY-060 " + label,
			"source_type":     "web_upload",
			"idempotency_key": "story060-capture-" + label + "-" + suffix,
		}), http.StatusCreated)["batch"].(map[string]any)
		file := e2ePostJSON(t, router, http.MethodPost, "/api/v1/capture-batches/"+e2eString(t, batch, "id")+"/files", adminToken, story056JSON(t, map[string]any{
			"file_asset_id":   assetID,
			"idempotency_key": "story060-file-" + label + "-" + suffix,
		}), http.StatusCreated)["file"].(map[string]any)
		e2ePostJSON(t, router, http.MethodPost, "/api/v1/capture-batches/"+e2eString(t, batch, "id")+"/process", adminToken, `{}`, http.StatusOK)
		var hash string
		if err := db.QueryRow(`
SELECT hash_sha256 FROM file_asset
WHERE tenant_id=$1::uuid AND id=$2::uuid AND deleted_at IS NULL
`, fixture.TenantID, assetID).Scan(&hash); err != nil {
			t.Fatalf("lookup synthetic page hash: %v", err)
		}
		decoded := make([]capture.DecodedPageInput, 0, len(barcodes))
		for index, value := range barcodes {
			page := capture.DecodedPageInput{
				SourceIndex: index + 1, FileAssetID: assetID, SHA256: hash, Width: 1000, Height: 1400,
			}
			if value != "" {
				page.Barcodes = []capture.BarcodeObservation{{
					Format: "QR_CODE", Text: value,
					Polygon: []map[string]int{
						{"x": 10, "y": 10}, {"x": 110, "y": 10},
						{"x": 110, "y": 110}, {"x": 10, "y": 110},
					},
				}}
			}
			decoded = append(decoded, page)
		}
		if _, err := store.ApplyFileResult(context.Background(), fixture.TenantID, e2eString(t, file, "id"), decoded); err != nil {
			t.Fatalf("apply %s multi-page result: %v", label, err)
		}
		return e2eString(t, file, "id")
	}

	insertedFileID := createCapture("inserted-page", []string{
		insertedBarcodes[1], "", insertedBarcodes[2], insertedBarcodes[3],
	})
	var insertedStudentID, insertedIdentityStatus string
	var insertedExpected, insertedActual int
	if err := db.QueryRow(`
SELECT s.student_id::text,s.identity_status,s.expected_page_count,s.actual_page_count
FROM submission s
JOIN capture_page cp ON cp.tenant_id=s.tenant_id AND cp.submission_id=s.id
WHERE cp.tenant_id=$1::uuid AND cp.capture_file_id=$2::uuid
LIMIT 1
`, fixture.TenantID, insertedFileID).Scan(
		&insertedStudentID, &insertedIdentityStatus, &insertedExpected, &insertedActual,
	); err != nil {
		t.Fatalf("query inserted-page submission: %v", err)
	}
	if insertedStudentID != studentID || insertedIdentityStatus != "matched" ||
		insertedExpected != 3 || insertedActual != 3 {
		t.Fatalf("controlled pages must match the student without counting the insertion: student=%s status=%s expected=%d actual=%d",
			insertedStudentID, insertedIdentityStatus, insertedExpected, insertedActual)
	}
	rows, err := db.Query(`
SELECT cp.source_index,cp.assigned_page_no,sp.page_no,cp.status,
       COALESCE(cp.page_identity->>'barcode_conflict_code','')
FROM capture_page cp
JOIN submission_page sp
  ON sp.tenant_id=cp.tenant_id AND sp.id=cp.submission_page_id
WHERE cp.tenant_id=$1::uuid AND cp.capture_file_id=$2::uuid
ORDER BY cp.source_index
`, fixture.TenantID, insertedFileID)
	if err != nil {
		t.Fatalf("query inserted-page ordering: %v", err)
	}
	defer rows.Close()
	semanticPages := []int{}
	physicalPages := []int{}
	for rows.Next() {
		var sourceIndex, semanticPage, physicalPage int
		var status, code string
		if err = rows.Scan(&sourceIndex, &semanticPage, &physicalPage, &status, &code); err != nil {
			t.Fatalf("scan inserted-page ordering: %v", err)
		}
		semanticPages = append(semanticPages, semanticPage)
		physicalPages = append(physicalPages, physicalPage)
		if sourceIndex == 2 && (status != "needs_review" || code != "unbound_page_in_controlled_sheet") {
			t.Fatalf("inserted uncontrolled page must be isolated for review: status=%s code=%s", status, code)
		}
	}
	if !slices.Equal(semanticPages, []int{1, 2, 2, 3}) ||
		!slices.Equal(physicalPages, []int{1, 2, 3, 4}) {
		t.Fatalf("inserted page shifted controlled semantics: semantic=%v physical=%v", semanticPages, physicalPages)
	}
	e2eExpectStatus(
		t, router, http.MethodGet,
		"/api/v1/answer-sheet-print-batches/"+printBatchID+"/package.pdf",
		adminToken, "", http.StatusConflict,
	)
	printContext = e2eGetJSON(
		t, router,
		"/api/v1/answer-sheet-templates/"+fixture.TemplateID+"/print-context",
		adminToken, http.StatusOK,
	)["print_context"].(map[string]any)
	batches = printContext["batches"].([]any)
	var observedBatchFound bool
	for _, raw := range batches {
		batch := raw.(map[string]any)
		if e2eString(t, batch, "print_batch_id") == printBatchID {
			observedBatchFound = batch["downloadable"] == false &&
				e2eFloat(t, batch, "observed_count") > 0
		}
	}
	if !observedBatchFound {
		t.Fatalf("observed package must become non-downloadable in the Web context: %#v", batches)
	}

	_, missingSheetSerial, missingBarcodes := issue("missing", missingStudentID)
	missingFileID := createCapture("missing-page", []string{missingBarcodes[1], missingBarcodes[3]})
	var missingExpected, missingActual int
	var assignedPages []int
	if err := db.QueryRow(`
SELECT s.expected_page_count,s.actual_page_count
FROM submission s
JOIN capture_page cp ON cp.tenant_id=s.tenant_id AND cp.submission_id=s.id
WHERE cp.tenant_id=$1::uuid AND cp.capture_file_id=$2::uuid
LIMIT 1
`, fixture.TenantID, missingFileID).Scan(&missingExpected, &missingActual); err != nil {
		t.Fatalf("query missing-page counts: %v", err)
	}
	missingRows, err := db.Query(`
SELECT assigned_page_no
FROM capture_page
WHERE tenant_id=$1::uuid AND capture_file_id=$2::uuid
ORDER BY source_index
`, fixture.TenantID, missingFileID)
	if err != nil {
		t.Fatalf("query missing-page assignments: %v", err)
	}
	defer missingRows.Close()
	for missingRows.Next() {
		var pageNo int
		if err = missingRows.Scan(&pageNo); err != nil {
			t.Fatalf("scan missing-page assignment: %v", err)
		}
		assignedPages = append(assignedPages, pageNo)
	}
	if missingExpected != 3 || missingActual != 2 || !slices.Equal(assignedPages, []int{1, 3}) {
		t.Fatalf("missing page must remain explicit without shifting page 3: expected=%d actual=%d pages=%v",
			missingExpected, missingActual, assignedPages)
	}

	_, repeatedSheetSerial, repeatedBarcodes := issue("repeated-student", studentID)
	repeatedFileID := createCapture("repeated-student", []string{repeatedBarcodes[1]})
	var repeatedConflicts int
	if err := db.QueryRow(`
SELECT count(DISTINCT s.id)
FROM submission s
WHERE s.tenant_id=$1::uuid
  AND s.identity_status='conflict'
  AND (
    s.student_id=$2::uuid
    OR s.id IN (
      SELECT cp.submission_id
      FROM capture_page cp
      WHERE cp.tenant_id=$1::uuid AND cp.capture_file_id=$3::uuid
    )
  )
`, fixture.TenantID, studentID, repeatedFileID).Scan(&repeatedConflicts); err != nil {
		t.Fatalf("query repeated-student conflicts: %v", err)
	}
	if repeatedConflicts != 1 {
		t.Fatalf("a second active submission for one student must become a review conflict without replacing the original, got %d", repeatedConflicts)
	}

	var insertedSheetCount, missingSheetCount, repeatedSheetCount int
	if err := db.QueryRow(`
SELECT
  (SELECT count(*) FROM answer_sheet_print_sheet WHERE tenant_id=$1::uuid AND id=$2::uuid),
  (SELECT count(*) FROM answer_sheet_print_sheet WHERE tenant_id=$1::uuid AND id=$3::uuid),
  (SELECT count(*) FROM answer_sheet_print_sheet WHERE tenant_id=$1::uuid AND id=$4::uuid)
`, fixture.TenantID, insertedSheetSerial, missingSheetSerial, repeatedSheetSerial).Scan(
		&insertedSheetCount, &missingSheetCount, &repeatedSheetCount,
	); err != nil {
		t.Fatalf("query multi-page sheet ledgers: %v", err)
	}
	if insertedSheetCount != 1 || missingSheetCount != 1 || repeatedSheetCount != 1 {
		t.Fatal("multi-page captures lost their immutable sheet ledgers")
	}
}

func TestStory060StudentSheetIdentityE2EWithPostgresTestDatabase(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("EDUGRADE_E2E_DATABASE_URL"))
	if dsn == "" {
		t.Skip("EDUGRADE_E2E_DATABASE_URL is not set; skipping STORY-060 sheet identity PostgreSQL acceptance workflow")
	}
	db := e2eOpenPostgresTestDB(t, dsn)
	e2eApplyPostgresMigrations(t, db)
	e2eActivatePostgresDemoUsers(t, db, []string{"tenant_admin"})
	router := e2ePostgresRouter(db)
	adminToken := e2eLoginWithTenant(t, router, "demo", "tenant_admin", "ChangeMe123!")
	suffix := strings.ReplaceAll(time.Now().UTC().Format("20060102150405.000000000"), ".", "")
	fixture := e2eCreateStory056AcceptanceFixture(t, db, router, adminToken, suffix)

	var rosterStudentID, gradeID string
	if err := db.QueryRow(`
SELECT st.id::text,cls.grade_id::text
FROM exam_class ec
JOIN school_class cls
  ON cls.tenant_id=ec.tenant_id AND cls.id=ec.class_id
JOIN student st
  ON st.tenant_id=ec.tenant_id AND st.class_id=ec.class_id
WHERE ec.tenant_id=$1::uuid AND ec.exam_id=$2::uuid
  AND ec.deleted_at IS NULL AND st.status='active' AND st.deleted_at IS NULL
ORDER BY st.created_at
LIMIT 1
`, fixture.TenantID, fixture.ExamID).Scan(&rosterStudentID, &gradeID); err != nil {
		t.Fatalf("lookup STORY-060 roster student: %v", err)
	}

	outsideClass := e2ePostJSON(t, router, http.MethodPost, "/api/v1/classes", adminToken, story056JSON(t, map[string]any{
		"school_id": fixture.SchoolID,
		"grade_id":  gradeID,
		"name":      "STORY-060 非应考班级",
		"code":      "story060-outside-" + suffix,
	}), http.StatusCreated)["class"].(map[string]any)
	outsideStudent := e2ePostJSON(t, router, http.MethodPost, "/api/v1/students", adminToken, story056JSON(t, map[string]any{
		"school_id":  fixture.SchoolID,
		"class_id":   e2eString(t, outsideClass, "id"),
		"student_no": "S060-OUTSIDE-" + suffix,
		"name":       "非本场应考学生",
	}), http.StatusCreated)["student"].(map[string]any)
	outsideStudentID := e2eString(t, outsideStudent, "id")

	e2eExpectStatus(
		t,
		router,
		http.MethodPost,
		"/api/v1/answer-sheet-templates/"+fixture.TemplateID+"/student-barcodes",
		adminToken,
		story056JSON(t, map[string]any{
			"student_ids":     []string{rosterStudentID, outsideStudentID},
			"idempotency_key": "story060-reject-outside-" + suffix,
		}),
		http.StatusBadRequest,
	)

	issueBody := story056JSON(t, map[string]any{
		"student_ids":     []string{rosterStudentID},
		"idempotency_key": "story060-issue-" + suffix,
	})
	issued := e2ePostJSON(
		t,
		router,
		http.MethodPost,
		"/api/v1/answer-sheet-templates/"+fixture.TemplateID+"/student-barcodes",
		adminToken,
		issueBody,
		http.StatusOK,
	)["barcodes"].(map[string]any)
	replayed := e2ePostJSON(
		t,
		router,
		http.MethodPost,
		"/api/v1/answer-sheet-templates/"+fixture.TemplateID+"/student-barcodes",
		adminToken,
		issueBody,
		http.StatusOK,
	)["barcodes"].(map[string]any)
	printBatchID := e2eString(t, issued, "print_batch_id")
	if e2eString(t, replayed, "print_batch_id") != printBatchID {
		t.Fatalf("idempotent issuance created a second print batch: first=%#v replay=%#v", issued, replayed)
	}
	issuedStudent := issued["students"].([]any)[0].(map[string]any)
	replayedStudent := replayed["students"].([]any)[0].(map[string]any)
	sheetSerial := e2eString(t, issuedStudent, "sheet_serial")
	if e2eString(t, replayedStudent, "sheet_serial") != sheetSerial {
		t.Fatalf("idempotent issuance changed the physical sheet serial: first=%#v replay=%#v", issuedStudent, replayedStudent)
	}
	barcodeValue := e2eString(t, issuedStudent["pages"].([]any)[0].(map[string]any), "value")
	if e2eString(t, replayedStudent["pages"].([]any)[0].(map[string]any), "value") != barcodeValue {
		t.Fatal("idempotent issuance must return the exact persisted barcode value")
	}
	e2eExpectStatus(
		t,
		router,
		http.MethodPost,
		"/api/v1/answer-sheet-templates/"+fixture.TemplateID+"/student-barcodes",
		adminToken,
		story056JSON(t, map[string]any{
			"student_ids":     []string{outsideStudentID},
			"idempotency_key": "story060-issue-" + suffix,
		}),
		http.StatusConflict,
	)
	var printBatchCount, printSheetCount, printPageCount int
	if err := db.QueryRow(`
SELECT
  (SELECT count(*) FROM answer_sheet_print_batch WHERE tenant_id=$1::uuid AND id=$2::uuid),
  (SELECT count(*) FROM answer_sheet_print_sheet WHERE tenant_id=$1::uuid AND print_batch_id=$2::uuid),
  (SELECT count(*) FROM answer_sheet_print_page p
     JOIN answer_sheet_print_sheet s ON s.tenant_id=p.tenant_id AND s.id=p.print_sheet_id
   WHERE s.tenant_id=$1::uuid AND s.print_batch_id=$2::uuid)
`, fixture.TenantID, printBatchID).Scan(&printBatchCount, &printSheetCount, &printPageCount); err != nil {
		t.Fatalf("query persisted print ledger: %v", err)
	}
	if printBatchCount != 1 || printSheetCount != 1 || printPageCount != 1 {
		t.Fatalf("unexpected print ledger counts: batch=%d sheet=%d page=%d", printBatchCount, printSheetCount, printPageCount)
	}

	firstAssetID := e2eUploadSyntheticPDF(
		t, router, adminToken, "story060-sheet-first-"+suffix+".pdf",
		"%PDF-1.4\n% STORY-060 first physical page "+suffix+"\n",
	)
	secondAssetID := e2eUploadSyntheticPDF(
		t, router, adminToken, "story060-sheet-duplicate-"+suffix+".pdf",
		"%PDF-1.4\n% STORY-060 duplicate physical page "+suffix+"\n",
	)
	batch := e2ePostJSON(t, router, http.MethodPost, "/api/v1/exams/"+fixture.ExamID+"/capture-batches", adminToken, story056JSON(t, map[string]any{
		"name":            "STORY-060 答题卡序列冲突验收",
		"source_type":     "web_upload",
		"idempotency_key": "story060-capture-" + suffix,
	}), http.StatusCreated)["batch"].(map[string]any)
	batchID := e2eString(t, batch, "id")
	firstFile := e2ePostJSON(t, router, http.MethodPost, "/api/v1/capture-batches/"+batchID+"/files", adminToken, story056JSON(t, map[string]any{
		"file_asset_id":   firstAssetID,
		"idempotency_key": "story060-file-first-" + suffix,
	}), http.StatusCreated)["file"].(map[string]any)
	secondFile := e2ePostJSON(t, router, http.MethodPost, "/api/v1/capture-batches/"+batchID+"/files", adminToken, story056JSON(t, map[string]any{
		"file_asset_id":   secondAssetID,
		"idempotency_key": "story060-file-duplicate-" + suffix,
	}), http.StatusCreated)["file"].(map[string]any)
	e2ePostJSON(t, router, http.MethodPost, "/api/v1/capture-batches/"+batchID+"/process", adminToken, `{}`, http.StatusOK)

	store := capture.NewPostgresStoreWithBarcodeKeyring(db, e2eBarcodeKeyring())
	applyPage := func(label string, file map[string]any, assetID string) {
		t.Helper()
		var hash string
		if err := db.QueryRow(`
SELECT hash_sha256
FROM file_asset
WHERE tenant_id=$1::uuid AND id=$2::uuid AND deleted_at IS NULL
`, fixture.TenantID, assetID).Scan(&hash); err != nil {
			t.Fatalf("lookup capture asset hash: %v", err)
		}
		_, err := store.ApplyFileResult(context.Background(), fixture.TenantID, e2eString(t, file, "id"), []capture.DecodedPageInput{{
			SourceIndex: 1,
			FileAssetID: assetID,
			SHA256:      hash,
			Width:       1000,
			Height:      1400,
			Barcodes: []capture.BarcodeObservation{{
				Format: "QR_CODE",
				Text:   barcodeValue,
				Polygon: []map[string]int{
					{"x": 10, "y": 10},
					{"x": 110, "y": 10},
					{"x": 110, "y": 110},
					{"x": 10, "y": 110},
				},
			}},
		}})
		if err != nil {
			t.Fatalf("apply %s captured student sheet page: %v", label, err)
		}
	}
	applyPage("first", firstFile, firstAssetID)
	applyPage("duplicate", secondFile, secondAssetID)

	pages := e2eGetJSON(t, router, "/api/v1/capture-batches/"+batchID+"/pages", adminToken, http.StatusOK)["pages"].([]any)
	if len(pages) != 2 {
		t.Fatalf("expected both repeated physical pages to remain visible for review: %#v", pages)
	}
	firstPage := pages[0].(map[string]any)
	secondPage := pages[1].(map[string]any)
	firstPageID := e2eString(t, firstPage, "id")
	if firstPage["status"] != "needs_review" || secondPage["status"] != "needs_review" {
		t.Fatalf("both sides of a sheet serial conflict must enter review: %#v", pages)
	}
	if e2eString(t, firstPage, "sheet_serial") != sheetSerial ||
		e2eString(t, secondPage, "sheet_serial") != sheetSerial ||
		e2eString(t, secondPage, "duplicate_of_page_id") != firstPageID {
		t.Fatalf("typed serial ownership or duplicate link missing: %#v", pages)
	}
	for _, rawPage := range pages {
		page := rawPage.(map[string]any)
		identity := page["page_identity"].(map[string]any)
		if identity["barcode_status"] != "needs_review" || identity["barcode_conflict_code"] != "sheet_page_duplicate" {
			t.Fatalf("page conflict evidence is incomplete: %#v", page)
		}
	}
	var sheetStatus string
	var conflictingSubmissions int
	if err := db.QueryRow(`
SELECT status
FROM answer_sheet_print_sheet
WHERE tenant_id=$1::uuid AND id=$2::uuid
`, fixture.TenantID, sheetSerial).Scan(&sheetStatus); err != nil {
		t.Fatalf("query conflicted print sheet: %v", err)
	}
	if err := db.QueryRow(`
SELECT count(DISTINCT s.id)
FROM capture_page p
JOIN submission s ON s.tenant_id=p.tenant_id AND s.id=p.submission_id
WHERE p.tenant_id=$1::uuid AND p.sheet_serial=$2::uuid
  AND p.deleted_at IS NULL AND s.identity_status='conflict'
`, fixture.TenantID, sheetSerial).Scan(&conflictingSubmissions); err != nil {
		t.Fatalf("query conflicting submissions: %v", err)
	}
	if sheetStatus != "conflict" || conflictingSubmissions != 2 {
		t.Fatalf("duplicate scan must persist a sheet and submission conflict: sheet=%s submissions=%d", sheetStatus, conflictingSubmissions)
	}

	deletedPage := e2ePostJSON(
		t,
		router,
		http.MethodPost,
		"/api/v1/capture-pages/"+e2eString(t, secondPage, "id")+"/delete",
		adminToken,
		story056JSON(t, map[string]any{
			"revision": e2eFloat(t, secondPage, "revision"),
			"reason":   "STORY-060 confirmed duplicate physical page",
		}),
		http.StatusOK,
	)["page"].(map[string]any)
	pages = e2eGetJSON(t, router, "/api/v1/capture-batches/"+batchID+"/pages", adminToken, http.StatusOK)["pages"].([]any)
	firstPage = pages[0].(map[string]any)
	secondPage = pages[1].(map[string]any)
	firstIdentity := firstPage["page_identity"].(map[string]any)
	if firstPage["status"] != "quality_checking" ||
		firstIdentity["barcode_status"] != "verified" ||
		firstIdentity["barcode_conflict_code"] != nil ||
		secondPage["status"] != "deleted" {
		t.Fatalf("deleting the confirmed duplicate must resolve the remaining serial conflict: %#v", pages)
	}
	var unassignedSubmissions int
	if err := db.QueryRow(`
SELECT count(DISTINCT s.id)
FROM capture_page p
JOIN submission s ON s.tenant_id=p.tenant_id AND s.id=p.submission_id
WHERE p.tenant_id=$1::uuid AND p.sheet_serial=$2::uuid
  AND p.deleted_at IS NULL AND s.identity_status='unassigned'
`, fixture.TenantID, sheetSerial).Scan(&unassignedSubmissions); err != nil {
		t.Fatalf("query reconciled submissions: %v", err)
	}
	if err := db.QueryRow(`
SELECT status
FROM answer_sheet_print_sheet
WHERE tenant_id=$1::uuid AND id=$2::uuid
`, fixture.TenantID, sheetSerial).Scan(&sheetStatus); err != nil {
		t.Fatalf("query reconciled sheet: %v", err)
	}
	if sheetStatus != "observed" || unassignedSubmissions != 2 {
		t.Fatalf("resolved duplicate must restore observed/unassigned state: sheet=%s submissions=%d", sheetStatus, unassignedSubmissions)
	}

	e2ePostJSON(
		t,
		router,
		http.MethodPost,
		"/api/v1/capture-pages/"+e2eString(t, deletedPage, "id")+"/restore",
		adminToken,
		story056JSON(t, map[string]any{
			"revision": e2eFloat(t, deletedPage, "revision"),
			"reason":   "STORY-060 restore verifies conflict recalculation",
		}),
		http.StatusOK,
	)
	pages = e2eGetJSON(t, router, "/api/v1/capture-batches/"+batchID+"/pages", adminToken, http.StatusOK)["pages"].([]any)
	for _, rawPage := range pages {
		page := rawPage.(map[string]any)
		identity := page["page_identity"].(map[string]any)
		if page["status"] != "needs_review" || identity["barcode_conflict_code"] != "sheet_page_duplicate" {
			t.Fatalf("restoring the duplicate must restore both sides of the conflict: %#v", pages)
		}
	}

	revoked := e2ePostJSON(
		t,
		router,
		http.MethodPost,
		"/api/v1/answer-sheet-print-sheets/"+sheetSerial+"/revoke",
		adminToken,
		`{"reason":"STORY-060 physical sheet damaged"}`,
		http.StatusOK,
	)["sheet"].(map[string]any)
	if revoked["status"] != "revoked" || revoked["revoke_reason"] != "STORY-060 physical sheet damaged" {
		t.Fatalf("sheet revocation metadata is incomplete: %#v", revoked)
	}
	pages = e2eGetJSON(t, router, "/api/v1/capture-batches/"+batchID+"/pages", adminToken, http.StatusOK)["pages"].([]any)
	for _, rawPage := range pages {
		page := rawPage.(map[string]any)
		identity := page["page_identity"].(map[string]any)
		if page["status"] != "needs_review" || identity["barcode_conflict_code"] != "sheet_revoked" {
			t.Fatalf("revocation must invalidate every observed page of the old serial: %#v", pages)
		}
	}

	reprintBody := story056JSON(t, map[string]any{
		"reason":          "STORY-060 replace damaged physical sheet",
		"idempotency_key": "story060-reprint-" + suffix,
	})
	reprinted := e2ePostJSON(
		t,
		router,
		http.MethodPost,
		"/api/v1/answer-sheet-print-sheets/"+sheetSerial+"/reprint",
		adminToken,
		reprintBody,
		http.StatusOK,
	)["barcodes"].(map[string]any)
	replayedReprint := e2ePostJSON(
		t,
		router,
		http.MethodPost,
		"/api/v1/answer-sheet-print-sheets/"+sheetSerial+"/reprint",
		adminToken,
		reprintBody,
		http.StatusOK,
	)["barcodes"].(map[string]any)
	reprintedStudent := reprinted["students"].([]any)[0].(map[string]any)
	reprintedSerial := e2eString(t, reprintedStudent, "sheet_serial")
	if reprintedSerial == sheetSerial ||
		e2eString(t, replayedReprint, "print_batch_id") != e2eString(t, reprinted, "print_batch_id") ||
		e2eString(t, replayedReprint["students"].([]any)[0].(map[string]any), "sheet_serial") != reprintedSerial {
		t.Fatalf("reprint must create one new immutable serial and replay it idempotently: first=%#v replay=%#v", reprinted, replayedReprint)
	}
	e2eExpectStatus(
		t,
		router,
		http.MethodPost,
		"/api/v1/answer-sheet-print-sheets/"+sheetSerial+"/reprint",
		adminToken,
		story056JSON(t, map[string]any{
			"reason":          "attempt a second replacement",
			"idempotency_key": "story060-second-reprint-" + suffix,
		}),
		http.StatusConflict,
	)
	var supersedesSerial, operation, reprintReason string
	if err := db.QueryRow(`
SELECT s.supersedes_sheet_id::text,b.operation,b.reason
FROM answer_sheet_print_sheet s
JOIN answer_sheet_print_batch b
  ON b.tenant_id=s.tenant_id AND b.id=s.print_batch_id
WHERE s.tenant_id=$1::uuid AND s.id=$2::uuid
`, fixture.TenantID, reprintedSerial).Scan(&supersedesSerial, &operation, &reprintReason); err != nil {
		t.Fatalf("query replacement lineage: %v", err)
	}
	if supersedesSerial != sheetSerial || operation != "reprint" ||
		reprintReason != "STORY-060 replace damaged physical sheet" {
		t.Fatalf("replacement lineage is incomplete: supersedes=%s operation=%s reason=%s", supersedesSerial, operation, reprintReason)
	}
}
