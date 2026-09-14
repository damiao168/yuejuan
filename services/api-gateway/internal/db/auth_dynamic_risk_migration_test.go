package db

import (
	"os"
	"strings"
	"testing"
)

func TestDynamicRiskTablesAreForcedTenantIsolated(t *testing.T) {
	raw, err := os.ReadFile("../../migrations/000147_auth_dynamic_risk_shadow.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(raw)
	for _, table := range []string{"auth_trusted_device", "auth_risk_event"} {
		for _, clause := range []string{
			"ALTER TABLE " + table + " ENABLE ROW LEVEL SECURITY",
			"ALTER TABLE " + table + " FORCE ROW LEVEL SECURITY",
			"CREATE POLICY edugrade_tenant_isolation ON " + table,
		} {
			if !strings.Contains(sql, clause) {
				t.Fatalf("dynamic risk migration is missing %q", clause)
			}
		}
	}
	if !strings.Contains(sql, "WITH CHECK (edugrade_tenant_matches(tenant_id))") {
		t.Fatal("dynamic risk writes must be constrained by the authenticated tenant")
	}
}
