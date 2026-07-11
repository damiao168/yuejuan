package exam

import "testing"

func TestCanTransition(t *testing.T) {
	if !CanTransition("draft", "configured") {
		t.Fatal("expected draft -> configured to be allowed")
	}
	if CanTransition("draft", "published") {
		t.Fatal("expected draft -> published to be rejected")
	}
	if CanTransition("configured", "collecting") || CanTransition("ready", "collecting") {
		t.Fatal("collection must only start through the readiness gate")
	}
	if !CanTransition("published", "archived") {
		t.Fatal("expected published -> archived to be allowed")
	}
}
