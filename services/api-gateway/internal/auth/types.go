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
	ErrUsernameExists     = errors.New("username already exists")
	ErrRoleNotFound       = errors.New("role not found")
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

type ManagedUser struct {
	ID          string    `json:"id"`
	Username    string    `json:"username"`
	DisplayName string    `json:"display_name"`
	Status      string    `json:"status"`
	Roles       []string  `json:"roles"`
	CreatedAt   time.Time `json:"created_at,omitempty"`
}

type ManagedUserFilter struct {
	Query           string
	Role            string
	UserID          string
	Limit           int
	CursorCreatedAt time.Time
	CursorID        string
}

type AssignableRole struct {
	Code        string `json:"code"`
	Name        string `json:"name"`
	ScopeType   string `json:"scope_type"`
	Description string `json:"description,omitempty"`
}

type CreateManagedUserInput struct {
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Password    string `json:"password"`
	RoleCode    string `json:"role_code"`
}

type Session struct {
	Token     string    `json:"access_token"`
	TokenHash string    `json:"-"`
	ExpiresAt time.Time `json:"expires_at"`
	User      User      `json:"user"`
}

const (
	SessionTypeStandard         = "standard"
	SessionTypeRememberedDevice = "remembered_device"
	SessionTypeDesktopDevice    = "desktop_device"
	SessionTypeService          = "service"
)

type CreateSessionInput struct {
	TenantID      string
	UserID        string
	TokenHash     string
	SessionType   string
	DeviceID      string
	DeviceName    string
	UserAgentHash string
	IPPrefix      string
	ExpiresAt     time.Time
}

type DeviceSession struct {
	ID          string    `json:"id"`
	SessionType string    `json:"session_type"`
	DeviceName  string    `json:"device_name"`
	CreatedAt   time.Time `json:"created_at"`
	LastSeenAt  time.Time `json:"last_seen_at"`
	ExpiresAt   time.Time `json:"expires_at"`
	Current     bool      `json:"current"`
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
	Action          string
	ActorID         string
	TargetType      string
	TargetID        string
	ExamID          string
	IPAddress       string
	CreatedFrom     time.Time
	CreatedTo       time.Time
	Limit           int
	CursorCreatedAt time.Time
	CursorID        string
}

type Store interface {
	FindUserByLogin(ctx context.Context, tenantCode string, username string) (UserWithPassword, error)
	CreateSession(ctx context.Context, input CreateSessionInput) (DeviceSession, error)
	FindUserBySession(ctx context.Context, tokenHash string, now time.Time) (User, error)
	ResolveAccessScope(ctx context.Context, user User) (AccessScope, error)
	DeleteSession(ctx context.Context, tokenHash string, reason string) error
	ListSessions(ctx context.Context, tenantID string, userID string, currentTokenHash string, now time.Time) ([]DeviceSession, error)
	RevokeSession(ctx context.Context, tenantID string, userID string, sessionID string, reason string) (bool, error)
	RevokeAllSessions(ctx context.Context, tenantID string, userID string, reason string) (int, error)
	UpdatePasswordAndRevokeSessions(ctx context.Context, tenantID string, userID string, expectedPasswordHash string, newPasswordHash string) (bool, int, error)
	Audit(ctx context.Context, event AuditEvent) error
	ListAudits(ctx context.Context, tenantID string, filter AuditFilter) ([]AuditRecord, error)
	ListManagedUsers(ctx context.Context, tenantID string, filter ManagedUserFilter) ([]ManagedUser, error)
	ListAssignableRoles(ctx context.Context, tenantID string) ([]AssignableRole, error)
	CreateManagedUser(ctx context.Context, tenantID string, tenantCode string, input CreateManagedUserInput, passwordHash string) (ManagedUser, error)
}
