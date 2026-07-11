package auth

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type MemoryStore struct {
	mu       sync.RWMutex
	users    map[string]UserWithPassword
	roles    map[string]map[string]AssignableRole
	sessions map[string]memorySession
	audits   []AuditRecord
	auditSeq int
	userSeq  int
}

type memorySession struct {
	TenantID  string
	UserID    string
	ExpiresAt time.Time
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		users:    map[string]UserWithPassword{},
		roles:    map[string]map[string]AssignableRole{},
		sessions: map[string]memorySession{},
		audits:   []AuditRecord{},
	}
}

func (s *MemoryStore) AddUser(user UserWithPassword) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.users[user.TenantCode+"|"+user.Username] = user
	if s.roles[user.TenantID] == nil {
		s.roles[user.TenantID] = map[string]AssignableRole{}
	}
	for _, code := range user.Roles {
		if _, exists := s.roles[user.TenantID][code]; !exists {
			s.roles[user.TenantID][code] = AssignableRole{Code: code, Name: code, ScopeType: "tenant"}
		}
	}
}

func (s *MemoryStore) AddRole(tenantID string, role AssignableRole) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.roles[tenantID] == nil {
		s.roles[tenantID] = map[string]AssignableRole{}
	}
	s.roles[tenantID][role.Code] = role
}

func (s *MemoryStore) FindUserByLogin(_ context.Context, tenantCode string, username string) (UserWithPassword, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	user, ok := s.users[tenantCode+"|"+username]
	if !ok || user.Status != "active" {
		return UserWithPassword{}, ErrInvalidCredentials
	}
	return user, nil
}

func (s *MemoryStore) CreateSession(_ context.Context, tenantID string, userID string, tokenHash string, expiresAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[tokenHash] = memorySession{TenantID: tenantID, UserID: userID, ExpiresAt: expiresAt}
	return nil
}

func (s *MemoryStore) FindUserBySession(_ context.Context, tokenHash string, now time.Time) (User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	session, ok := s.sessions[tokenHash]
	if !ok || !session.ExpiresAt.After(now) {
		return User{}, ErrUnauthenticated
	}
	for _, user := range s.users {
		if user.ID == session.UserID && user.TenantID == session.TenantID && user.Status == "active" {
			return user.User, nil
		}
	}
	return User{}, ErrUnauthenticated
}

func (s *MemoryStore) DeleteSession(_ context.Context, tokenHash string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, tokenHash)
	return nil
}

func (s *MemoryStore) Audit(_ context.Context, event AuditEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.auditSeq++
	s.audits = append(s.audits, AuditRecord{
		ID:          fmt.Sprintf("memory-audit-%d", s.auditSeq),
		TenantID:    event.TenantID,
		ActorID:     event.ActorID,
		Action:      event.Action,
		TargetType:  event.TargetType,
		TargetID:    event.TargetID,
		BeforeValue: cloneAuditMap(event.BeforeValue),
		AfterValue:  cloneAuditMap(event.AfterValue),
		Reason:      event.Reason,
		IPAddress:   event.IPAddress,
		UserAgent:   event.UserAgent,
		RequestID:   event.RequestID,
		CreatedAt:   time.Now().UTC(),
	})
	return nil
}

func (s *MemoryStore) Audits() []AuditEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]AuditEvent, len(s.audits))
	for i, record := range s.audits {
		out[i] = AuditEvent{
			TenantID:   record.TenantID,
			ActorID:    record.ActorID,
			Action:     record.Action,
			TargetType: record.TargetType,
			TargetID:   record.TargetID,
			Reason:     record.Reason,
			IPAddress:  record.IPAddress,
			UserAgent:  record.UserAgent,
			RequestID:  record.RequestID,
		}
	}
	return out
}

func (s *MemoryStore) ListAudits(_ context.Context, tenantID string, filter AuditFilter) ([]AuditRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	limit := normalizedAuditLimit(filter.Limit)
	out := []AuditRecord{}
	for i := len(s.audits) - 1; i >= 0 && len(out) < limit; i-- {
		record := s.audits[i]
		if record.TenantID != tenantID {
			continue
		}
		if filter.Action != "" && record.Action != filter.Action {
			continue
		}
		if filter.ActorID != "" && record.ActorID != filter.ActorID {
			continue
		}
		if filter.TargetType != "" && record.TargetType != filter.TargetType {
			continue
		}
		if filter.TargetID != "" && record.TargetID != filter.TargetID {
			continue
		}
		if filter.ExamID != "" && record.TargetID != filter.ExamID {
			continue
		}
		if filter.IPAddress != "" && record.IPAddress != filter.IPAddress {
			continue
		}
		if !filter.CreatedFrom.IsZero() && record.CreatedAt.Before(filter.CreatedFrom) {
			continue
		}
		if !filter.CreatedTo.IsZero() && record.CreatedAt.After(filter.CreatedTo) {
			continue
		}
		out = append(out, record)
	}
	return out, nil
}

func (s *MemoryStore) ListManagedUsers(_ context.Context, tenantID string) ([]ManagedUser, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []ManagedUser{}
	for _, user := range s.users {
		if user.TenantID != tenantID {
			continue
		}
		out = append(out, ManagedUser{
			ID: user.ID, Username: user.Username, DisplayName: user.DisplayName,
			Status: user.Status, Roles: append([]string(nil), user.Roles...),
		})
	}
	return out, nil
}

func (s *MemoryStore) ListAssignableRoles(_ context.Context, tenantID string) ([]AssignableRole, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []AssignableRole{}
	for _, role := range s.roles[tenantID] {
		if role.Code == "platform_admin" || role.Code == "tenant_admin" {
			continue
		}
		out = append(out, role)
	}
	return out, nil
}

func (s *MemoryStore) CreateManagedUser(_ context.Context, tenantID string, tenantCode string, input CreateManagedUserInput, passwordHash string) (ManagedUser, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := tenantCode + "|" + input.Username
	if existing, exists := s.users[key]; exists && existing.User.Status != "deleted" {
		return ManagedUser{}, ErrUsernameExists
	}
	role, exists := s.roles[tenantID][input.RoleCode]
	if !exists || role.Code == "platform_admin" || role.Code == "tenant_admin" {
		return ManagedUser{}, ErrRoleNotFound
	}
	s.userSeq++
	user := UserWithPassword{
		User: User{
			ID: fmt.Sprintf("managed-user-%d", s.userSeq), TenantID: tenantID, TenantCode: tenantCode,
			Username: input.Username, DisplayName: input.DisplayName, Status: "active",
			Roles: []string{input.RoleCode}, Permissions: []string{}, DataScope: map[string]any{input.RoleCode: map[string]any{"scope": "tenant"}},
		},
		PasswordHash: passwordHash,
	}
	s.users[key] = user
	return ManagedUser{ID: user.ID, Username: user.Username, DisplayName: user.DisplayName, Status: user.Status, Roles: user.Roles}, nil
}

func cloneAuditMap(value map[string]any) map[string]any {
	if len(value) == 0 {
		return nil
	}
	out := make(map[string]any, len(value))
	for key, item := range value {
		out[key] = item
	}
	return out
}
