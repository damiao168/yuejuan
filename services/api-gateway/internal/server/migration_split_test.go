package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestE2ESplitSQLStatementsKeepsDollarQuotedBlocksTogether(t *testing.T) {
	sqlText := `
CREATE TABLE sample (id INT);
DO $$
BEGIN
  CREATE OR REPLACE FUNCTION pg_temp.sample_fn()
  RETURNS VOID
  LANGUAGE plpgsql
  AS $fn$
  BEGIN
    PERFORM 1;
  END;
  $fn$;

  PERFORM pg_temp.sample_fn();
END $$;
CREATE INDEX idx_sample_id ON sample (id);
`

	statements := e2eSplitSQLStatements(sqlText)

	if len(statements) != 3 {
		t.Fatalf("expected 3 SQL statements, got %d: %#v", len(statements), statements)
	}
	if statements[1][0:5] != "DO $$" {
		t.Fatalf("second statement should be the full dollar-quoted DO block: %q", statements[1])
	}
}

func TestE2ESplitSQLStatementsIgnoresSemicolonsInLineComments(t *testing.T) {
	statements := e2eSplitSQLStatements("-- first clause; second clause\nCREATE TABLE sample (id INT);\n-- trailing; note\nCREATE INDEX idx_sample_id ON sample (id);")
	if len(statements) != 2 {
		t.Fatalf("expected 2 SQL statements, got %d: %#v", len(statements), statements)
	}
	if !strings.Contains(statements[0], "CREATE TABLE") || !strings.Contains(statements[1], "CREATE INDEX") {
		t.Fatalf("line comments split executable SQL: %#v", statements)
	}
}

func TestStory050ImageQualityMigrationContainsRequiredSchema(t *testing.T) {
	path := filepath.Join("..", "..", "migrations", "000022_story050_image_quality_run.sql")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	text := strings.ToLower(string(raw))
	for _, want := range []string{
		"submission_page_quality_run",
		"latest_quality_run_id",
		"normalized_file_asset_id",
		"quality_override",
		"quality_status in ('unchecked', 'passed', 'review', 'failed')",
		"processing_status in ('pending', 'processing', 'completed', 'retryable_error', 'terminal_error')",
		"normalization_transform jsonb",
		"source_sha256",
		"lease_token",
		"lease_expires_at",
		"result_version",
		"profile_config_hash",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("story050 migration missing %q", want)
		}
	}
}

func TestExamTemplateRoutingMigrationContainsBindingAndEvidence(t *testing.T) {
	path := filepath.Join("..", "..", "migrations", "000131_exam_answer_sheet_template_binding.sql")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	text := strings.ToLower(string(raw))
	for _, want := range []string{
		"exam_answer_sheet_template_binding",
		"locked_with_guard",
		"bound_auto",
		"template_content_hash",
		"routing_mode",
		"guard_report",
		"page_template_match_run",
		"source_page_revision",
		"candidates jsonb",
		"decision in ('matched', 'ambiguous', 'unknown', 'conflict')",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("template routing migration missing %q", want)
		}
	}
}
