package db

import (
	"os"
	"strings"
	"testing"
)

func TestMFANotificationMigrationRetainsPendingIntentsAndIsolation(t *testing.T) {
	raw, err := os.ReadFile("../../migrations/000150_auth_mfa_security_notification_intents.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(raw)
	for _, required := range []string{"auth_security_notification_intent", "event_id UUID PRIMARY KEY", "UNIQUE (tenant_id, id)", "REFERENCES event_outbox(tenant_id, id)", "REFERENCES app_user(tenant_id, id)", "CHECK (status = 'awaiting_channel')", "ENABLE ROW LEVEL SECURITY", "FORCE ROW LEVEL SECURITY", "WITH CHECK (edugrade_tenant_matches(tenant_id))", "aggregate_type <> 'auth_mfa' OR edugrade_tenant_matches(tenant_id)", "guard_auth_mfa_outbox_fact", "current_setting('edugrade.tenant_id', true) IS DISTINCT FROM 'maintenance'", "BEFORE UPDATE OR DELETE ON event_outbox", "REVOKE UPDATE, DELETE"} {
		if !strings.Contains(sql, required) {
			t.Fatalf("MFA notification migration missing %q", required)
		}
	}
	for _, forbidden := range []string{"secret_ciphertext", "code_hash", "session_hash", "user_agent", "phone TEXT", "email TEXT", "EXECUTE FUNCTION enqueue_business_mutation_outbox"} {
		if strings.Contains(sql, forbidden) {
			t.Fatalf("secret/contact data or generic secret-table trigger introduced: %s", forbidden)
		}
	}
}
