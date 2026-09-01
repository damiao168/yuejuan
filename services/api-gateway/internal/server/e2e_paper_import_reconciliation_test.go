package server

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"edugrade-enterprise/services/api-gateway/internal/paper"
)

func TestPaperImportApplyGateE2EWithPostgresTestDatabase(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("EDUGRADE_E2E_DATABASE_URL"))
	if dsn == "" {
		t.Skip("EDUGRADE_E2E_DATABASE_URL is not set; skipping paper import PostgreSQL apply gate test")
	}
	db := e2eOpenPostgresTestDB(t, dsn)
	e2eApplyPostgresMigrations(t, db)
	ctx := context.Background()
	var tenantID, userID, examID, paperID, paperFileID, answerFileID, questionID string
	err := db.QueryRowContext(ctx, `
WITH fixture AS (
  SELECT t.id AS tenant_id,(MAX(u.id::text) FILTER (WHERE u.username='teacher'))::uuid AS user_id
  FROM tenant t JOIN app_user u ON u.tenant_id=t.id
  WHERE t.code='demo' AND u.username='teacher'
  GROUP BY t.id
), school_row AS (
  INSERT INTO school(tenant_id,name,code,status)
  SELECT tenant_id,'Paper import gate school','paper-import-gate','active' FROM fixture
  RETURNING id,tenant_id
), exam_row AS (
  INSERT INTO exam(tenant_id,school_id,name,subject,exam_type,total_score,status,grading_mode,appeal_enabled,publish_policy,created_by)
  SELECT f.tenant_id,s.id,'Paper import gate exam','mathematics','quiz',10,'draft','ai_assisted',true,'manual_after_confirmation',f.user_id
  FROM fixture f JOIN school_row s ON s.tenant_id=f.tenant_id
  RETURNING id,tenant_id,created_by
), paper_file AS (
  INSERT INTO file_asset(tenant_id,exam_id,owner_type,owner_id,original_name,content_type,size_bytes,hash_sha256,storage_bucket,storage_key,visibility,uploaded_by)
  SELECT tenant_id,id,'exam',id,'paper.pdf','application/pdf',1,repeat('b',64),'test','paper-import-gate-paper','tenant',created_by FROM exam_row
  RETURNING id,tenant_id,exam_id,uploaded_by
), answer_file AS (
  INSERT INTO file_asset(tenant_id,exam_id,owner_type,owner_id,original_name,content_type,size_bytes,hash_sha256,storage_bucket,storage_key,visibility,uploaded_by)
  SELECT tenant_id,id,'exam',id,'answer.pdf','application/pdf',1,repeat('c',64),'test','paper-import-gate-answer','tenant',created_by FROM exam_row
  RETURNING id,tenant_id,exam_id
), paper_row AS (
  INSERT INTO exam_paper(tenant_id,exam_id,file_asset_id,version_no,status,uploaded_by)
  SELECT tenant_id,exam_id,id,1,'uploaded',uploaded_by FROM paper_file
  RETURNING id,tenant_id,exam_id
), question_row AS (
  INSERT INTO question(tenant_id,exam_id,exam_paper_id,question_no,question_type,score,knowledge_points,answer_area,sort_order,status)
  SELECT tenant_id,exam_id,id,'1','short_answer',10,'[]','{}',1,'active' FROM paper_row
  RETURNING id,tenant_id,exam_id
)
SELECT f.tenant_id::text,f.user_id::text,e.id::text,p.id::text,pf.id::text,af.id::text,q.id::text
FROM fixture f
JOIN exam_row e ON e.tenant_id=f.tenant_id
JOIN paper_row p ON p.exam_id=e.id
JOIN paper_file pf ON pf.exam_id=e.id
JOIN answer_file af ON af.exam_id=e.id
JOIN question_row q ON q.exam_id=e.id
`).Scan(&tenantID, &userID, &examID, &paperID, &paperFileID, &answerFileID, &questionID)
	if err != nil {
		t.Fatalf("seed paper import fixture: %v", err)
	}

	store := paper.NewPostgresStore(db)
	job, err := store.CreatePaperImport(ctx, tenantID, examID, userID, paper.CreatePaperImportInput{
		ExamPaperID: paperID, PaperFileAssetID: paperFileID, AnswerFileAssetID: answerFileID, Subject: "mathematics",
	})
	if err != nil {
		t.Fatalf("create paper import: %v", err)
	}
	invalid := postgresPaperImportDraft("1", "essay", 10)
	if _, err := store.CompletePaperImport(ctx, tenantID, job.ID, []paper.PaperImportDraftQuestion{invalid}, nil); err != nil {
		t.Fatalf("preview invalid candidate: %v", err)
	}
	if _, err := store.ApplyPaperImport(ctx, tenantID, job.ID, userID); !errors.Is(err, paper.ErrInvalidInput) {
		t.Fatalf("PostgreSQL apply gate accepted invalid reconciliation: %v", err)
	}
	var stem string
	if err := db.QueryRowContext(ctx, `SELECT COALESCE(stem,'') FROM question WHERE tenant_id=$1::uuid AND id=$2::uuid`, tenantID, questionID).Scan(&stem); err != nil || stem != "" {
		t.Fatalf("blocked PostgreSQL import mutated canonical question: stem=%q err=%v", stem, err)
	}

	valid := postgresPaperImportDraft("1.", "short_answer", 10)
	valid.Solution = &paper.SolutionInput{RawText: "先列式，再计算", Steps: []paper.SolutionStep{{StepNo: 1, Content: "列式"}}}
	if _, err := store.CompletePaperImport(ctx, tenantID, job.ID, []paper.PaperImportDraftQuestion{valid}, nil); err != nil {
		t.Fatalf("preview corrected candidate: %v", err)
	}
	if _, err := store.ApplyPaperImport(ctx, tenantID, job.ID, userID); err != nil {
		t.Fatalf("corrected PostgreSQL import did not apply: %v", err)
	}
	if err := db.QueryRowContext(ctx, `SELECT COALESCE(stem,'') FROM question WHERE tenant_id=$1::uuid AND id=$2::uuid`, tenantID, questionID).Scan(&stem); err != nil || stem != "Imported 1." {
		t.Fatalf("corrected PostgreSQL import was not persisted: stem=%q err=%v", stem, err)
	}
	var appliedImportID, solutionText string
	if err := db.QueryRowContext(ctx, `SELECT COALESCE(q.paper_import_id::text,''),s.raw_text FROM question q JOIN question_solution s ON s.tenant_id=q.tenant_id AND s.question_id=q.id AND s.deleted_at IS NULL WHERE q.tenant_id=$1::uuid AND q.id=$2::uuid`, tenantID, questionID).Scan(&appliedImportID, &solutionText); err != nil || appliedImportID != job.ID || solutionText != "先列式，再计算" {
		t.Fatalf("formal provenance/solution missing: import=%q solution=%q err=%v", appliedImportID, solutionText, err)
	}

	reorderJob, err := store.CreatePaperImport(ctx, tenantID, examID, userID, paper.CreatePaperImportInput{Subject: "mathematics", Sources: []paper.CreatePaperImportSourceInput{
		{FileAssetID: paperFileID, DocumentIndex: 0, RoleHint: "auto"},
		{FileAssetID: answerFileID, DocumentIndex: 1, RoleHint: "auto"},
	}})
	if err != nil {
		t.Fatalf("create source reorder fixture: %v", err)
	}
	if _, err = store.CompletePaperImportCandidates(ctx, tenantID, reorderJob.ID, nil, nil, []paper.AnswerCandidate{{CandidateID: "a1", QuestionNoHint: "1", StandardAnswer: "42"}}, nil, nil, nil); err != nil {
		t.Fatalf("complete source reorder fixture: %v", err)
	}
	reordered, err := store.ReplacePaperImportSources(ctx, tenantID, reorderJob.ID, userID, paper.ReplacePaperImportSourcesInput{Sources: []paper.ReplacePaperImportSourceInput{
		{ID: reorderJob.Sources[1].ID, DocumentIndex: 0, RoleHint: "answer"},
		{ID: reorderJob.Sources[0].ID, DocumentIndex: 1, RoleHint: "question"},
	}})
	if err != nil {
		t.Fatalf("replace ordered sources against partial unique index: %v", err)
	}
	if len(reordered.Sources) != 2 || reordered.Sources[0].FileAssetID != answerFileID || reordered.Sources[1].FileAssetID != paperFileID {
		t.Fatalf("PostgreSQL source order was not preserved: %#v", reordered.Sources)
	}
	var errorCode string
	if err := db.QueryRowContext(ctx, `SELECT error_code FROM paper_import_job WHERE tenant_id=$1::uuid AND id=$2::uuid`, tenantID, reorderJob.ID).Scan(&errorCode); err != nil || errorCode != "" {
		t.Fatalf("replacing sources must clear error_code to empty string: %q (%v)", errorCode, err)
	}
}

func postgresPaperImportDraft(number, kind string, score float64) paper.PaperImportDraftQuestion {
	return paper.PaperImportDraftQuestion{
		QuestionNo: number, QuestionType: kind, Score: score, Stem: "Imported " + number,
		AnswerKey: &paper.AnswerKeyInput{StandardAnswer: "42", EquivalentAnswers: []any{"42"}, Tolerance: map[string]any{}},
		Rubric: &paper.RubricInput{Status: "draft", MaxScore: score, Points: []paper.RubricPoint{
			{ID: "p1", Description: "correct", Score: score, Required: true},
		}},
		Confidence: .99, Issues: []string{},
	}
}
