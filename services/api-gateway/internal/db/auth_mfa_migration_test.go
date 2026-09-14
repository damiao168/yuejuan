package db

import (
	"os"
	"strings"
	"testing"
)

func TestScopedMFAMigrationProtectsSecretsAndTenantBoundaries(t *testing.T) {
	raw, err := os.ReadFile("../../migrations/000149_auth_totp_scoped_challenges.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(raw)
	for _, required := range []string{"auth_totp", "auth_mfa_recovery_code", "auth_mfa_challenge", "secret_ciphertext BYTEA", "secret_nonce BYTEA", "code_hash TEXT", "token_hash TEXT", "session_hash TEXT", "security_epoch BIGINT", "UNIQUE (tenant_id, user_id)", "REFERENCES auth_totp(tenant_id, user_id, id) ON DELETE CASCADE", "ENABLE ROW LEVEL SECURITY", "FORCE ROW LEVEL SECURITY", "WITH CHECK (edugrade_tenant_matches(tenant_id))", "attempts BETWEEN 0 AND 5", "mfa.disable", "mfa.recovery.rotate", "TO edugrade_tenant_runtime"} {
		if !strings.Contains(sql, required) {
			t.Fatalf("MFA migration missing %q", required)
		}
	}
	for _, forbidden := range []string{"secret TEXT", "recovery_code TEXT", "challenge_token TEXT"} {
		if strings.Contains(sql, forbidden) {
			t.Fatal("MFA plaintext secret column introduced")
		}
	}
}
