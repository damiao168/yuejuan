package seedquality

import (
	"context"
	"edugrade-enterprise/services/api-gateway/internal/commandreceipt"
	"strings"
	"testing"
)

func TestSubmissionRecoveryKeepsBlindReceipt(t *testing.T) {
	store := NewMemoryStore()
	task := Task{ID: "task", AssignedTo: "grader", Status: "in_progress", Revision: 1, GoldPaperID: "secret-gold", ReferenceScore: 5}
	store.tasks[taskKey("tenant", task.ID)] = memoryTask{tenantID: "tenant", task: task}
	ctx := commandreceipt.WithID(context.Background(), "blind-command")
	input := SubmitInput{Score: 4, ExpectedRevision: 1}
	completed, _, err := store.CompleteTask(ctx, "tenant", task.ID, "grader", input, Observation{ReferenceScore: 5, SubmittedScore: 4, GoldPaperID: "secret-gold"})
	if err != nil {
		t.Fatal(err)
	}
	again, _, err := store.CompleteTask(ctx, "tenant", task.ID, "grader", input, Observation{})
	if err != nil || completed.Revision != again.Revision {
		t.Fatalf("blind replay: %+v %v", again, err)
	}
	receipt, err := store.RecoverCommand(ctx, "tenant", "grader", "blind-command")
	if err != nil || receipt.Status != "succeeded" {
		t.Fatalf("receipt: %+v %v", receipt, err)
	}
	if strings.Contains(string(receipt.Result), "gold") || strings.Contains(string(receipt.Result), "reference") || strings.Contains(string(receipt.Result), "observation") {
		t.Fatalf("blind metadata leak: %s", receipt.Result)
	}
}
