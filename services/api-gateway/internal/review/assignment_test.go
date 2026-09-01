package review

import (
	"context"
	"fmt"
	"testing"
)

func TestMemoryStoreHasActiveAssignmentIsExactAndUnpaged(t *testing.T) {
	store := NewMemoryStore()
	for index := 0; index < 501; index++ {
		id := fmt.Sprintf("task-%03d", index)
		segmentID := fmt.Sprintf("segment-%03d", index)
		store.tasks[key("tenant-a", id)] = ReviewTask{
			ID: id, TenantID: "tenant-a", AssignedTo: "reviewer-a",
			AnswerSegmentID: segmentID, Status: "assigned",
		}
	}

	allowed, err := store.HasActiveAssignment(context.Background(), "tenant-a", "reviewer-a", "segment-500")
	if err != nil || !allowed {
		t.Fatalf("assignment beyond the first 500 tasks was not found: allowed=%t err=%v", allowed, err)
	}
	for _, check := range []struct {
		name, tenantID, reviewerID, segmentID string
	}{
		{"foreign reviewer", "tenant-a", "reviewer-b", "segment-500"},
		{"foreign tenant", "tenant-b", "reviewer-a", "segment-500"},
		{"foreign segment", "tenant-a", "reviewer-a", "segment-missing"},
	} {
		t.Run(check.name, func(t *testing.T) {
			allowed, err := store.HasActiveAssignment(context.Background(), check.tenantID, check.reviewerID, check.segmentID)
			if err != nil || allowed {
				t.Fatalf("assignment boundary leaked: allowed=%t err=%v", allowed, err)
			}
		})
	}
}

func TestMemoryStoreHasActiveAssignmentPreservesStatusSemantics(t *testing.T) {
	for _, testCase := range []struct {
		status  string
		allowed bool
	}{
		{"pending", true},
		{"assigned", true},
		{"in_progress", true},
		{"submitted", true},
		{"returned", true},
		{"completed", false},
		{"cancelled", false},
	} {
		t.Run(testCase.status, func(t *testing.T) {
			store := NewMemoryStore()
			store.tasks[key("tenant-a", "task-a")] = ReviewTask{
				ID: "task-a", TenantID: "tenant-a", AssignedTo: "reviewer-a",
				AnswerSegmentID: "segment-a", Status: testCase.status,
			}
			allowed, err := store.HasActiveAssignment(context.Background(), "tenant-a", "reviewer-a", "segment-a")
			if err != nil || allowed != testCase.allowed {
				t.Fatalf("status %s: allowed=%t err=%v", testCase.status, allowed, err)
			}
		})
	}
}
