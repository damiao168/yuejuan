package pagination

import (
	"testing"
	"time"
)

func TestCursorRoundTripAndLimits(t *testing.T) {
	now := time.Date(2026, 8, 2, 12, 0, 0, 123, time.UTC)
	decoded, err := Decode(Encode(now, "item-1"))
	if err != nil || !decoded.CreatedAt.Equal(now) || decoded.ID != "item-1" {
		t.Fatalf("unexpected cursor: %#v %v", decoded, err)
	}
	if _, err := Decode("not-base64"); err == nil {
		t.Fatal("malformed cursor must be rejected")
	}
	if _, err := Limit("201", 50, 200); err == nil {
		t.Fatal("oversized page must be rejected")
	}
	parts, err := DecodeParts(EncodeParts("9", now.Format(time.RFC3339Nano), "task-1"), 3)
	if err != nil || len(parts) != 3 || parts[0] != "9" || parts[2] != "task-1" {
		t.Fatalf("unexpected composite cursor: %#v %v", parts, err)
	}
	if _, err := DecodeParts(EncodeParts("only", "two"), 3); err == nil {
		t.Fatal("composite cursor with wrong arity must be rejected")
	}
}
