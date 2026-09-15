package auth

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestNormalizedDeviceNameTruncatesOnRuneBoundary(t *testing.T) {
	value := strings.Repeat("阅", 129)
	got := normalizedDeviceName(value, SessionTypeDesktopDevice)

	if !utf8.ValidString(got) {
		t.Fatalf("device name must remain valid UTF-8: %q", got)
	}
	if count := utf8.RuneCountInString(got); count != 128 {
		t.Fatalf("device name rune count=%d want 128", count)
	}
}
