package auth

import (
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestHashPasswordUsesArgon2idAndSupportsUnicodePassphrases(t *testing.T) {
	password := "这是只有张老师知道的一句很长的密码短语"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(hash, "$argon2id$") {
		t.Fatalf("expected Argon2id PHC hash, got %q", hash)
	}
	valid, needsRehash := VerifyPassword(hash, password)
	if !valid || needsRehash {
		t.Fatalf("new hash verification valid=%v needsRehash=%v", valid, needsRehash)
	}
	if CheckPassword(hash, password+"错误") {
		t.Fatal("wrong passphrase must not verify")
	}
}

func TestLegacyBcryptPasswordRequestsTransparentRehash(t *testing.T) {
	legacy, err := bcrypt.GenerateFromPassword([]byte("LegacyTeacher123!"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatal(err)
	}
	valid, needsRehash := VerifyPassword(string(legacy), "LegacyTeacher123!")
	if !valid || !needsRehash {
		t.Fatalf("legacy bcrypt valid=%v needsRehash=%v", valid, needsRehash)
	}
}

func TestArgon2ParserRejectsExcessiveParameters(t *testing.T) {
	malicious := "$argon2id$v=19$m=999999999,t=2,p=1$c2FsdHNhbHRzYWx0$AAAAAAAAAAAAAAAAAAAAAA"
	if valid, _ := VerifyPassword(malicious, "anything"); valid {
		t.Fatal("out-of-bounds Argon2 parameters must be rejected")
	}
}
