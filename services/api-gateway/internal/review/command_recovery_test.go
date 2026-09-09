package review

import (
	"context"
	"edugrade-enterprise/services/api-gateway/internal/commandreceipt"
	"errors"
	"testing"
)

func TestSubmitCommandSurvivesTerminalStateAndDeletion(t *testing.T) {
	store := NewMemoryStore()
	store.AddContext(tenantID, "segment-1", reviewContext())
	task, err := store.CreateTask(context.Background(), tenantID, "manager", CreateTaskInput{AnswerSegmentID: "segment-1", Source: "manual_sample", AssignedTo: "reviewer-1"})
	if err != nil {
		t.Fatal(err)
	}
	ctx := commandreceipt.WithID(context.Background(), "submit-command")
	input := SubmitGradeInput{ExpectedRevision: task.Revision, Score: 4, RubricSelections: []RubricSelection{{PointID: "p1", Score: 4}}}
	first, err := store.SubmitGrade(ctx, tenantID, task.ID, "reviewer-1", input)
	if err != nil {
		t.Fatal(err)
	}
	delete(store.tasks, key(tenantID, task.ID))
	replay, err := store.SubmitGrade(ctx, tenantID, task.ID, "reviewer-1", input)
	if err != nil || replay.Grade.ID != first.Grade.ID {
		t.Fatalf("deleted replay: %+v %v", replay, err)
	}
	input.Score = 3
	if _, err = store.SubmitGrade(ctx, tenantID, task.ID, "reviewer-1", input); !errors.Is(err, commandreceipt.ErrConflict) {
		t.Fatalf("changed payload: %v", err)
	}
	receipt, err := store.RecoverCommand(ctx, tenantID, "reviewer-2", "submit-command")
	if err != nil || receipt.Status != "not_accepted" {
		t.Fatalf("actor scope: %+v %v", receipt, err)
	}
}
