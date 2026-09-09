package exam

import (
	"context"
	"errors"
	"testing"

	"edugrade-enterprise/services/api-gateway/internal/auth"
)

func TestExamSessionCommandReplayAndRecovery(t *testing.T) {
	store := NewMemoryStore()
	scope := auth.AccessScope{TenantID: "tenant", TenantWide: true}
	input := CreateSessionInput{SchoolID: "school", GradeID: "grade", Name: "期中", ExamType: "formal_exam", GradingMode: "ai_assisted", PublishPolicy: "after_admin_approval", CommandID: "command-1234", Subjects: []SessionSubjectInput{{Subject: "math", TotalScore: 100}}}
	first, err := store.CreateExamSession(context.Background(), scope, "user", input)
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.CreateExamSession(context.Background(), scope, "user", input)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID || len(first.Exams) != len(second.Exams) || first.Exams[0].ID != second.Exams[0].ID {
		t.Fatalf("command replay created different resources: %#v %#v", first, second)
	}
	recovered, err := store.RecoverExamSessionCommand(context.Background(), scope, "user", input.CommandID)
	if err != nil || recovered.Status != "succeeded" || recovered.Session == nil || recovered.Session.ID != first.ID {
		t.Fatalf("recovery = %#v, %v", recovered, err)
	}
}

func TestExamSessionCommandRejectsChangedRequest(t *testing.T) {
	store := NewMemoryStore()
	scope := auth.AccessScope{TenantID: "tenant", TenantWide: true}
	input := CreateSessionInput{SchoolID: "school", GradeID: "grade", Name: "期中", ExamType: "formal_exam", GradingMode: "ai_assisted", PublishPolicy: "after_admin_approval", CommandID: "command-1234", Subjects: []SessionSubjectInput{{Subject: "math", TotalScore: 100}}}
	if _, err := store.CreateExamSession(context.Background(), scope, "user", input); err != nil {
		t.Fatal(err)
	}
	input.Name = "changed"
	if _, err := store.CreateExamSession(context.Background(), scope, "user", input); !errors.Is(err, ErrCommandConflict) {
		t.Fatalf("changed command error = %v", err)
	}
}

func TestExamSessionCommandReportsNotAccepted(t *testing.T) {
	store := NewMemoryStore()
	scope := auth.AccessScope{TenantID: "tenant", TenantWide: true}
	result, err := store.RecoverExamSessionCommand(context.Background(), scope, "user", "command-missing")
	if err != nil || result.Status != "not_accepted" || result.CommandID != "command-missing" {
		t.Fatalf("missing command recovery = %#v, %v", result, err)
	}
}
