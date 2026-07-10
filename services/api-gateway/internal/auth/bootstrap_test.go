package auth_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"edugrade-enterprise/services/api-gateway/internal/auth"
)

func TestBootstrapInitialAdminCreatesFirstPlatformAdmin(t *testing.T) {
	store := &fakeBootstrapStore{}

	result, err := auth.BootstrapInitialAdmin(context.Background(), store, auth.BootstrapAdminInput{
		Username:    "ops_admin",
		DisplayName: "Ops Admin",
		Password:    "StrongPass123!",
	})
	if err != nil {
		t.Fatalf("bootstrap returned error: %v", err)
	}
	if store.checkedTenantCode != "platform" || store.checkedRoleCode != "platform_admin" {
		t.Fatalf("bootstrap must default to platform/platform_admin, got %s/%s", store.checkedTenantCode, store.checkedRoleCode)
	}
	if store.upsertInput.Username != "ops_admin" || store.upsertInput.DisplayName != "Ops Admin" {
		t.Fatalf("unexpected upsert input: %#v", store.upsertInput)
	}
	if store.passwordHash == "" || store.passwordHash == "StrongPass123!" || !strings.HasPrefix(store.passwordHash, "$2") {
		t.Fatalf("bootstrap must pass bcrypt password hash, got %q", store.passwordHash)
	}
	if result.Username != "ops_admin" || result.RoleCode != "platform_admin" {
		t.Fatalf("unexpected bootstrap result: %#v", result)
	}
}

func TestBootstrapInitialAdminRejectsExistingActiveAdmin(t *testing.T) {
	store := &fakeBootstrapStore{activeAdminExists: true}

	_, err := auth.BootstrapInitialAdmin(context.Background(), store, auth.BootstrapAdminInput{
		Username: "ops_admin",
		Password: "StrongPass123!",
	})
	if !errors.Is(err, auth.ErrBootstrapAlreadyCompleted) {
		t.Fatalf("expected ErrBootstrapAlreadyCompleted, got %v", err)
	}
	if store.upsertCalled {
		t.Fatal("bootstrap must not upsert when an active admin already exists")
	}
}

func TestBootstrapInitialAdminRejectsWeakPassword(t *testing.T) {
	store := &fakeBootstrapStore{}

	_, err := auth.BootstrapInitialAdmin(context.Background(), store, auth.BootstrapAdminInput{
		Username: "ops_admin",
		Password: "short",
	})
	if !errors.Is(err, auth.ErrWeakBootstrapPassword) {
		t.Fatalf("expected ErrWeakBootstrapPassword, got %v", err)
	}
	if store.checkedTenantCode != "" || store.upsertCalled {
		t.Fatalf("weak password must fail before store calls, got check=%s upsert=%v", store.checkedTenantCode, store.upsertCalled)
	}
}

type fakeBootstrapStore struct {
	activeAdminExists bool
	checkedTenantCode string
	checkedRoleCode   string
	upsertCalled      bool
	upsertInput       auth.BootstrapAdminInput
	passwordHash      string
}

func (s *fakeBootstrapStore) ActiveAdminExists(ctx context.Context, tenantCode string, roleCode string) (bool, error) {
	s.checkedTenantCode = tenantCode
	s.checkedRoleCode = roleCode
	return s.activeAdminExists, nil
}

func (s *fakeBootstrapStore) UpsertBootstrapAdmin(ctx context.Context, input auth.BootstrapAdminInput, passwordHash string) (auth.BootstrapAdminResult, error) {
	s.upsertCalled = true
	s.upsertInput = input
	s.passwordHash = passwordHash
	return auth.BootstrapAdminResult{
		TenantID:   "tenant-platform",
		TenantCode: input.TenantCode,
		UserID:     "user-ops-admin",
		Username:   input.Username,
		RoleCode:   input.RoleCode,
	}, nil
}
