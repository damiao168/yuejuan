package capture

import (
	"errors"
	"strings"
	"testing"
)

func TestControlledBarcodeRoundTripAndTamperRejection(t *testing.T) {
	keyring := BarcodeKeyring{ActiveKeyID: "2026-01", Keys: map[string][]byte{"2026-01": []byte(strings.Repeat("k", 32))}}
	input := BarcodeClaims{TenantID: "11111111-1111-4111-8111-111111111111", ExamID: "22222222-2222-4222-8222-222222222222", TemplateID: "33333333-3333-4333-8333-333333333333", TemplateContentHash: "sha256:template", PageNo: 2, Nonce: "random-128-bit"}
	value, err := keyring.Sign(input)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := keyring.Verify(value)
	if err != nil || claims.PageNo != 2 || claims.KeyID != "2026-01" {
		t.Fatalf("verify returned %#v, %v", claims, err)
	}
	tampered := strings.Replace(value, "EG1.", "EG1.A", 1)
	if _, err = keyring.Verify(tampered); !errors.Is(err, ErrBarcodeInvalid) {
		t.Fatalf("tampered value should be rejected, got %v", err)
	}
}

func TestControlledBarcodeRejectsUnknownKeyAndInvalidClaims(t *testing.T) {
	keyring := BarcodeKeyring{ActiveKeyID: "active", Keys: map[string][]byte{"active": []byte(strings.Repeat("a", 32))}}
	_, err := keyring.Sign(BarcodeClaims{TenantID: "bad", ExamID: "bad", TemplateID: "bad", PageNo: 0})
	if !errors.Is(err, ErrBarcodeInvalid) {
		t.Fatalf("invalid claims should be rejected, got %v", err)
	}
}
