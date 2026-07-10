package auth

import (
	"context"
	"errors"
	"time"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUnauthenticated    = errors.New("unauthenticated")
	ErrForbidden          = errors.New("forbidden")
)

const PlatformTenantID = "00000000-0000-0000-0000-000000000001"

type User struct {
	ID          string         `json:"id"`
	TenantID    string         `json:"tenant_id"`
	TenantCode  string         `json:"tenant_code"`
	Username    string         `json:"username"`
	DisplayName string         `json:"display_name"`
	Status      string         `json:"status"`
	Roles       []string       `json:"roles"`
	Permissions []string       `json:"permissions"`
	DataScope   map[string]any `json:"data_scope"`
}

type UserWithPassword struct {
	User
	PasswordHash string
}

type Session struct {
	Token     string    `json:"access_token"`
	TokenHash string    `json:"-"`
	ExpiresAt time.Time `json:"expires_at"`
	User      User      `json:"user"`
}

type AuditEvent struct {
	TenantID    string
	ActorID     string
	Action      string
	TargetType  string
	TargetID    string
	BeforeValue map[string]any
	AfterValue  map[string]any
	Reason      string
	IPAddress   string
	UserAgent   string
	RequestID   string
}

type AuditRecord struct {
	ID          string         `json:"id"`
	TenantID    string         `json:"tenant_id"`
	ActorID     string         `json:"actor_id,omitempty"`
	Action      string         `json:"action"`
	TargetType  string         `json:"target_type"`
	TargetID    string         `json:"target_id,omitempty"`
	BeforeValue map[string]any `json:"before_value,omitempty"`
	AfterValue  map[string]any `json:"after_value,omitempty"`
	Reason      string         `json:"reason,omitempty"`
	IPAddress   string         `json:"ip_address,omitempty"`
	UserAgent   string         `json:"user_agent,omitempty"`
	RequestID   string         `json:"request_id,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
}

type AuditFilter struct {
	Action      string
	ActorID     string
	TargetType  string
	TargetID    string
	ExamID      string
	IPAddress   string
	CreatedFrom time.Time
	CreatedTo   time.Time
	Limit       int
}

type Store interface {
	FindUserByLogin(ctx context.Context, tenantCode string, username string) (UserWithPassword, error)
	CreateSession(ctx context.Context, tenantID string, userID string, tokenHash string, expiresAt time.Time) error
	FindUserBySession(ctx context.Context, tokenHash string, now time.Time) (User, error)
	DeleteSession(ctx context.Context, tokenHash string) error
	Audit(ctx context.Context, event AuditEvent) error
	ListAudits(ctx context.Context, tenantID string, filter AuditFilter) ([]AuditRecord, error)
}
