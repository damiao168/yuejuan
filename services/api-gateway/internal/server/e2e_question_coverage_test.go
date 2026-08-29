package server

import (
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"edugrade-enterprise/services/api-gateway/internal/scorerelease"
)

func TestReleaseGateBlocksSubmissionWithoutCompleteQuestionCoverageE2EWithPostgresTestDatabase(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("EDUGRADE_E2E_DATABASE_URL"))
	if dsn == "" {
		t.Skip("EDUGRADE_E2E_DATABASE_URL is not set; skipping question coverage PostgreSQL acceptance workflow")
	}
	db := e2eOpenPostgresTestDB(t, dsn)
	e2eApplyPostgresMigrations(t, db)
	e2eActivatePostgresDemoUsers(t, db, []string{"tenant_admin"})
	router := e2ePostgresRouter(db)
	adminToken := e2eLoginWithTenant(t, router, "demo", "tenant_admin", "ChangeMe123!")
	suffix := strings.ReplaceAll(time.Now().UTC().Format("20060102150405.000000000"), ".", "")
	fixture := e2eCreateStory056AcceptanceFixture(t, db, router, adminToken, suffix)

	var studentID string
	if err := db.QueryRow(`
SELECT student_id::text
FROM exam_candidate_snapshot
WHERE tenant_id=$1::uuid AND exam_id=$2::uuid
ORDER BY captured_at
LIMIT 1
`, fixture.TenantID, fixture.ExamID).Scan(&studentID); err != nil {
		t.Fatalf("lookup frozen candidate: %v", err)
	}
	submission := e2ePostJSON(t, router, http.MethodPost, "/api/v1/exams/"+fixture.ExamID+"/submissions", adminToken,
		story056JSON(t, map[string]any{
			"student_id": studentID, "candidate_no": "COVERAGE-" + suffix,
			"source_type": "pdf_upload", "expected_page_count": 0,
		}), http.StatusCreated)["submission"].(map[string]any)
	if _, err := db.Exec(`
INSERT INTO submission_grade (
  tenant_id,exam_id,submission_id,student_id,anonymous_code,total_score,max_score,
  status,locked,confirmed_by,confirmed_at,created_by
)
VALUES ($1::uuid,$2::uuid,$3::uuid,$4::uuid,$5,0,3,'confirmed',false,$6::uuid,now(),$6::uuid)
`, fixture.TenantID, fixture.ExamID, e2eString(t, submission, "id"), studentID, "COVERAGE-"+suffix, fixture.AdminID); err != nil {
		t.Fatalf("seed a grade whose missing question coverage must be rejected: %v", err)
	}

	gate, err := scorerelease.NewPostgresStore(db, nil).Gate(t.Context(), fixture.TenantID, fixture.ExamID)
	if err != nil {
		t.Fatalf("calculate PostgreSQL release gate: %v", err)
	}
	for _, issue := range gate.Blocking {
		if issue.Code == "question_coverage_incomplete" {
			if issue.Count != 1 {
				t.Fatalf("question coverage blocker count mismatch: %#v", issue)
			}
			return
		}
	}
	t.Fatalf("question coverage blocker missing: %#v", gate)
}
