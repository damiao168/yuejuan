package org

import (
	"os"
	"strings"
	"testing"
)

func TestPostgresOrganizationWritesCheckParentTenant(t *testing.T) {
	raw, err := os.ReadFile("store_postgres.go")
	if err != nil {
		t.Fatalf("read postgres store: %v", err)
	}
	sqlText := compactSQL(string(raw))
	for _, want := range []string{
		"FROM school WHERE tenant_id = $1 AND id::text = $2 AND deleted_at IS NULL",
		"FROM school WHERE tenant_id = $1 AND id::text = $2 AND deleted_at IS NULL",
		"FROM grade WHERE tenant_id = $1 AND id::text = $3 AND school_id::text = $2 AND deleted_at IS NULL",
		"FROM school_class WHERE tenant_id = $1 AND id::text = $3 AND school_id::text = $2 AND deleted_at IS NULL",
	} {
		if !strings.Contains(sqlText, want) {
			t.Fatalf("Postgres organization writes must check parent tenant with %q", want)
		}
	}
}

func compactSQL(value string) string {
	return strings.Join(strings.Fields(value), " ")
}
