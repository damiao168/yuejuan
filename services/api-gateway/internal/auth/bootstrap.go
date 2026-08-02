package auth

import (
	"context"
	"errors"
	"strings"
	"unicode"
)

var (
	ErrBootstrapAlreadyCompleted = errors.New("bootstrap already completed")
	ErrWeakBootstrapPassword     = errors.New("bootstrap password does not meet strength requirements")
	ErrInvalidBootstrapInput     = errors.New("invalid bootstrap input")
)

type BootstrapAdminInput struct {
	TenantCode  string
	RoleCode    string
	Username    string
	DisplayName string
	Password    string
}

type BootstrapAdminResult struct {
	TenantID   string `json:"tenant_id"`
	TenantCode string `json:"tenant_code"`
	UserID     string `json:"user_id"`
	Username   string `json:"username"`
	RoleCode   string `json:"role_code"`
}

type BootstrapStore interface {
	ActiveAdminExists(ctx context.Context, tenantCode string, roleCode string) (bool, error)
	UpsertBootstrapAdmin(ctx context.Context, input BootstrapAdminInput, passwordHash string) (BootstrapAdminResult, error)
}

func BootstrapInitialAdmin(ctx context.Context, store BootstrapStore, input BootstrapAdminInput) (BootstrapAdminResult, error) {
	input = normalizeBootstrapAdminInput(input)
	if input.Username == "" {
		return BootstrapAdminResult{}, ErrInvalidBootstrapInput
	}
	if !bootstrapFieldsWithinLimits(input) {
		return BootstrapAdminResult{}, ErrInvalidBootstrapInput
	}
	if !strongBootstrapPassword(input.Password) {
		return BootstrapAdminResult{}, ErrWeakBootstrapPassword
	}
	exists, err := store.ActiveAdminExists(ctx, input.TenantCode, input.RoleCode)
	if err != nil {
		return BootstrapAdminResult{}, err
	}
	if exists {
		return BootstrapAdminResult{}, ErrBootstrapAlreadyCompleted
	}
	hash, err := HashPassword(input.Password)
	if err != nil {
		return BootstrapAdminResult{}, err
	}
	return store.UpsertBootstrapAdmin(ctx, input, hash)
}

func normalizeBootstrapAdminInput(input BootstrapAdminInput) BootstrapAdminInput {
	input.TenantCode = strings.TrimSpace(input.TenantCode)
	if input.TenantCode == "" {
		input.TenantCode = "platform"
	}
	input.RoleCode = strings.TrimSpace(input.RoleCode)
	if input.RoleCode == "" {
		input.RoleCode = "platform_admin"
	}
	input.Username = strings.TrimSpace(input.Username)
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	if input.DisplayName == "" {
		input.DisplayName = input.Username
	}
	return input
}

func strongBootstrapPassword(password string) bool {
	return StrongPassword(password)
}

func StrongPassword(password string) bool {
	if len([]rune(password)) < 12 {
		return false
	}
	var hasUpper, hasLower, hasDigit, hasSymbol bool
	for _, ch := range password {
		switch {
		case unicode.IsUpper(ch):
			hasUpper = true
		case unicode.IsLower(ch):
			hasLower = true
		case unicode.IsDigit(ch):
			hasDigit = true
		case unicode.IsPunct(ch) || unicode.IsSymbol(ch):
			hasSymbol = true
		}
	}
	return hasUpper && hasLower && hasDigit && hasSymbol
}
