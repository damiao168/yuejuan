package server

import (
	"context"
	"os"
	"strings"
	"testing"

	"edugrade-enterprise/services/api-gateway/internal/review"
)

func TestReviewAssignmentLookupE2EWithPostgresTestDatabase(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("EDUGRADE_E2E_DATABASE_URL"))
	if dsn == "" {
		t.Skip("EDUGRADE_E2E_DATABASE_URL is not set; skipping exact review assignment PostgreSQL test")
	}
	db := e2eOpenPostgresTestDB(t, dsn)
	e2eApplyPostgresMigrations(t, db)
	ctx := context.Background()
	var tenantID, reviewerID, otherReviewerID, segmentID, taskID string
	err := db.QueryRowContext(ctx, `
WITH fixture AS (
  SELECT t.id AS tenant_id,
         (MAX(u.id::text) FILTER (WHERE u.username='grader'))::uuid AS reviewer_id,
         (MAX(u.id::text) FILTER (WHERE u.username='teacher'))::uuid AS other_reviewer_id
  FROM tenant t JOIN app_user u ON u.tenant_id=t.id
  WHERE t.code='demo' AND u.username IN ('grader','teacher')
  GROUP BY t.id
), school_row AS (
  INSERT INTO school(tenant_id,name,code,status)
  SELECT tenant_id,'Assignment lookup school','assignment-lookup','active' FROM fixture
  RETURNING id,tenant_id
), exam_row AS (
  INSERT INTO exam(tenant_id,school_id,name,subject,exam_type,total_score,status,grading_mode,appeal_enabled,publish_policy,created_by)
  SELECT f.tenant_id,s.id,'Assignment lookup exam','mathematics','quiz',10,'draft','ai_assisted',true,'manual_after_confirmation',f.other_reviewer_id
  FROM fixture f JOIN school_row s ON s.tenant_id=f.tenant_id
  RETURNING id,tenant_id,created_by
), file_row AS (
  INSERT INTO file_asset(tenant_id,exam_id,owner_type,owner_id,original_name,content_type,size_bytes,hash_sha256,storage_bucket,storage_key,visibility,uploaded_by)
  SELECT tenant_id,id,'exam',id,'assignment.pdf','application/pdf',1,repeat('a',64),'test','assignment','tenant',created_by FROM exam_row
  RETURNING id,tenant_id,exam_id,uploaded_by
), paper_row AS (
  INSERT INTO exam_paper(tenant_id,exam_id,file_asset_id,version_no,status,uploaded_by)
  SELECT tenant_id,exam_id,id,1,'uploaded',uploaded_by FROM file_row
  RETURNING id,tenant_id,exam_id,uploaded_by
), question_row AS (
  INSERT INTO question(tenant_id,exam_id,exam_paper_id,question_no,question_type,score,knowledge_points,answer_area,sort_order,status)
  SELECT tenant_id,exam_id,id,'1','short_answer',10,'[]','{}',1,'active' FROM paper_row
  RETURNING id,tenant_id,exam_id
), submission_row AS (
  INSERT INTO submission(tenant_id,exam_id,candidate_no,source_type,status,collected_by)
  SELECT e.tenant_id,e.id,'assignment-candidate','pdf_upload','pages_uploaded',e.created_by FROM exam_row e
  RETURNING id,tenant_id,exam_id,collected_by
), page_row AS (
  INSERT INTO submission_page(tenant_id,submission_id,file_asset_id,page_no,status)
  SELECT s.tenant_id,s.id,f.id,1,'accepted' FROM submission_row s JOIN file_row f ON f.tenant_id=s.tenant_id
  RETURNING id,tenant_id,submission_id
), segment_row AS (
  INSERT INTO answer_segment(tenant_id,submission_id,submission_page_id,question_id,question_no,bbox,source,status)
  SELECT s.tenant_id,s.id,p.id,q.id,'1','{"x":0,"y":0,"width":1,"height":1}','manual','accepted'
  FROM submission_row s JOIN page_row p ON p.submission_id=s.id JOIN question_row q ON q.exam_id=s.exam_id
  RETURNING id,tenant_id,submission_id,question_id
), task_row AS (
  INSERT INTO review_task(tenant_id,exam_id,question_id,question_no,answer_segment_id,submission_id,anonymous_code,source,status,assigned_to,created_by)
  SELECT f.tenant_id,e.id,s.question_id,'1',s.id,s.submission_id,'anonymous','manual_sample','assigned',f.reviewer_id,f.other_reviewer_id
  FROM fixture f JOIN exam_row e ON e.tenant_id=f.tenant_id JOIN segment_row s ON s.tenant_id=f.tenant_id
  RETURNING id,tenant_id,answer_segment_id,assigned_to
)
SELECT t.tenant_id::text,t.assigned_to::text,f.other_reviewer_id::text,t.answer_segment_id::text,t.id::text
FROM task_row t JOIN fixture f ON f.tenant_id=t.tenant_id
`).Scan(&tenantID, &reviewerID, &otherReviewerID, &segmentID, &taskID)
	if err != nil {
		t.Fatalf("seed exact assignment fixture: %v", err)
	}

	store := review.NewPostgresStore(db)
	assertAssignment := func(name, tenant, reviewer, segment string, expected bool) {
		t.Helper()
		actual, err := store.HasActiveAssignment(ctx, tenant, reviewer, segment)
		if err != nil || actual != expected {
			t.Fatalf("%s: actual=%t expected=%t err=%v", name, actual, expected, err)
		}
	}
	assertAssignment("assigned", tenantID, reviewerID, segmentID, true)
	assertAssignment("other reviewer", tenantID, otherReviewerID, segmentID, false)
	assertAssignment("other tenant", "00000000-0000-0000-0000-000000000001", reviewerID, segmentID, false)

	for _, status := range []struct {
		value   string
		allowed bool
	}{{"submitted", true}, {"completed", false}, {"cancelled", false}} {
		if _, err := db.ExecContext(ctx, `UPDATE review_task SET status=$1 WHERE tenant_id=$2::uuid AND id=$3::uuid`, status.value, tenantID, taskID); err != nil {
			t.Fatalf("set review task status %s: %v", status.value, err)
		}
		assertAssignment(status.value, tenantID, reviewerID, segmentID, status.allowed)
	}
}
