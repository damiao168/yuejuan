package auth

import "testing"

func TestNormalizePhoneRejectsInvalidCountryCodeAndNonASCIIDigits(t *testing.T) {
	for _, value := range []string{"+01234567890", "+８６１３８００１３８０００", "12345", "+"} {
		if _, err := NormalizePhone(value); err == nil {
			t.Fatalf("invalid phone %q must be rejected", value)
		}
	}
	for _, value := range []string{"13800138000", "86 138-0013-8000", "0086(138)00138000", "+86 13800138000"} {
		if got, err := NormalizePhone(value); err != nil || got != "+8613800138000" {
			t.Fatalf("phone %q normalized to %q: %v", value, got, err)
		}
	}
}
