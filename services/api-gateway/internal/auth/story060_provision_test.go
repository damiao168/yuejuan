package auth

import (
	"context"
	"errors"
	"testing"
)

type story060ProvisionRecorder struct{ roles []string }

func (s *story060ProvisionRecorder) ProvisionStory060User(_ context.Context, _, _, roleCode, _ string) error {
	s.roles = append(s.roles, roleCode)
	return nil
}

func TestProvisionStory060UsersUsesDedicatedOrdinaryRoles(t *testing.T) {
	store := &story060ProvisionRecorder{}
	if err := ProvisionStory060Users(context.Background(), store, "School-Admin!2026", "Worker-Secret!2026"); err != nil {
		t.Fatal(err)
	}
	if len(store.roles) != 2 || store.roles[0] != "school_admin" || store.roles[1] != "subjective_grading_worker" {
		t.Fatalf("unexpected provisioned roles: %v", store.roles)
	}
}

func TestProvisionStory060UsersRejectsWeakPasswords(t *testing.T) {
	err := ProvisionStory060Users(context.Background(), &story060ProvisionRecorder{}, "weak", "Worker-Secret!2026")
	if !errors.Is(err, ErrStory060ProvisioningInput) {
		t.Fatalf("expected provisioning input rejection, got %v", err)
	}
}
