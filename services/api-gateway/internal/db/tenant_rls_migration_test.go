package db

import (
	"os"
	"strings"
	"testing"
)

func TestHighRiskTenantRLSMigrationIsForcedAndComplete(t *testing.T) {
	raw, err := os.ReadFile("../../migrations/000139_high_risk_tenant_rls.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(raw)
	for _, table := range []string{"student", "submission", "submission_page", "answer_segment", "ocr_result", "human_grade", "final_grade", "appeal", "file_asset"} {
		if !strings.Contains(sql, "'"+table+"'") {
			t.Fatalf("high-risk RLS migration does not protect %s", table)
		}
	}
	for _, required := range []string{"ENABLE ROW LEVEL SECURITY", "FORCE ROW LEVEL SECURITY", "WITH CHECK", "edugrade_tenant_matches", "NOBYPASSRLS", "edugrade_tenant_runtime"} {
		if !strings.Contains(sql, required) {
			t.Fatalf("high-risk RLS migration is missing %q", required)
		}
	}
}
