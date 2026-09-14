package config

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestOptionalMFARequiresIndependentKey(t *testing.T) {
	t.Setenv("EDUGRADE_ENV", "test")
	t.Setenv("EDUGRADE_MFA_ENABLED", "false")
	t.Setenv("EDUGRADE_MFA_MASTER_KEY", "")
	t.Setenv("EDUGRADE_MFA_MASTER_KEY_FILE", "")
	cfg, err := Load("")
	if err != nil || cfg.Auth.MFAEnabled || cfg.Auth.MFAMasterKey != "" {
		t.Fatal("MFA must default off without fallback key")
	}
	t.Setenv("EDUGRADE_MFA_ENABLED", "true")
	for _, key := range []string{"", "invalid", base64.StdEncoding.EncodeToString([]byte("short"))} {
		t.Setenv("EDUGRADE_MFA_MASTER_KEY", key)
		if _, err := Load(""); err == nil || !strings.Contains(err.Error(), "EDUGRADE_MFA_MASTER_KEY") {
			t.Fatal("enabled MFA accepted invalid key")
		}
	}
	key := base64.StdEncoding.EncodeToString([]byte("01234567890123456789012345678901"))
	t.Setenv("EDUGRADE_MFA_MASTER_KEY", key)
	cfg, err = Load("")
	if err != nil || !cfg.Auth.MFAEnabled || cfg.Auth.MFAMasterKey != key {
		t.Fatal("valid key rejected")
	}
	t.Setenv("EDUGRADE_MODEL_CREDENTIAL_MASTER_KEY", key)
	if _, err := Load(""); err == nil {
		t.Fatal("cross-domain key reuse accepted")
	}
}
