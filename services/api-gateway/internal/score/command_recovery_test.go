package score

import (
	"context"
	"edugrade-enterprise/services/api-gateway/internal/commandreceipt"
	"errors"
	"testing"
)

func TestConfirmationCommandReplaysOriginalResult(t *testing.T) {
	store := seededScoreStore()
	ctx := context.Background()
	if _, err := store.FinalizeExam(ctx, tenantID, "exam-1", "manager-1"); err != nil {
		t.Fatal(err)
	}
	ctx = commandreceipt.WithID(ctx, "confirm-one")
	input := ConfirmInput{Reason: "original"}
	first, err := store.ConfirmGrades(ctx, tenantID, "exam-1", "manager-1", input)
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.ConfirmGrades(ctx, tenantID, "exam-1", "manager-1", input)
	if err != nil || first[0].Revision != second[0].Revision {
		t.Fatalf("confirmation replay: %+v %v", second, err)
	}
	if _, err = store.ConfirmGrades(ctx, tenantID, "exam-1", "manager-1", ConfirmInput{Reason: "changed"}); !errors.Is(err, commandreceipt.ErrConflict) {
		t.Fatalf("changed confirmation: %v", err)
	}
}
