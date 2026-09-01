package auth

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

type MemoryStore struct {
	mu             sync.RWMutex
	users          map[string]UserWithPassword
	roles          map[string]map[string]AssignableRole
	tenantStatuses map[string]string
	sessions       map[string]memorySession
	boundaries     map[string]ResourceBoundary
	audits         []AuditRecord
	auditSeq       int
	userSeq        int
	sessionSeq     int
}

type memorySession struct {
	ID          string
	TenantID    string
	UserID      string
	SessionType string
	DeviceName  string
	CreatedAt   time.Time
	LastSeenAt  time.Time
	ExpiresAt   time.Time
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		users:          map[string]UserWithPassword{},
		roles:          map[string]map[string]AssignableRole{},
		tenantStatuses: map[string]string{},
		sessions:       map[string]memorySession{},
		boundaries:     map[string]ResourceBoundary{},
		audits:         []AuditRecord{},
	}
}

func (s *MemoryStore) AddResourceBoundary(resourceType, resourceID string, boundary ResourceBoundary) {
	s.mu.Lock()
	defer s.mu.Unlock()
	boundary.ResourceType = resourceType
	boundary.ResourceID = resourceID
	s.boundaries[resourceType+"|"+resourceID] = boundary
}

func (s *MemoryStore) ResolveResourceBoundary(_ context.Context, scope AccessScope, resourceType, resourceID string) (ResourceBoundary, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if boundary, ok := s.boundaries[resourceType+"|"+resourceID]; ok {
		return boundary, nil
	}
	boundary := ResourceBoundary{ResourceType: resourceType, ResourceID: resourceID, TenantID: scope.TenantID}
	if scope.syntheticUnbounded {
		return boundary, nil
	}
	switch resourceType {
	case "exam":
		if scope.AllowsExam(resourceID) || scope.TenantWide {
			boundary.ExamID = resourceID
			return boundary, nil
		}
	case "submission":
		if scope.AllowsSubmission(resourceID) || scope.TenantWide {
			boundary.SubmissionID = resourceID
			return boundary, nil
		}
	case "file_asset":
		if scope.AllowsFile(resourceID) || scope.TenantWide {
			return boundary, nil
		}
	case "review_task":
		if scope.AllowsReviewTask(resourceID) || scope.TenantWide {
			boundary.AssignedTo = scope.ActorID
			return boundary, nil
		}
	case "arbitration_task":
		if scope.AllowsArbitrationTask(resourceID) || scope.TenantWide {
			boundary.AssignedTo = scope.ActorID
			return boundary, nil
		}
	case "student":
		if scope.AllowsStudent(resourceID) || scope.TenantWide {
			boundary.StudentID = resourceID
			return boundary, nil
		}
	default:
		if scope.TenantWide {
			return boundary, nil
		}
	}
	return ResourceBoundary{}, ErrResourceBoundaryNotFound
}

func (s *MemoryStore) AddUser(user UserWithPassword) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.users[user.TenantCode+"|"+user.Username] = user
	if _, exists := s.tenantStatuses[user.TenantID]; !exists {
		s.tenantStatuses[user.TenantID] = "active"
	}
	if s.roles[user.TenantID] == nil {
		s.roles[user.TenantID] = map[string]AssignableRole{}
	}
	for _, code := range user.Roles {
		if _, exists := s.roles[user.TenantID][code]; !exists {
			s.roles[user.TenantID][code] = AssignableRole{Code: code, Name: code, ScopeType: RolePolicy(code).CanonicalScope}
		}
	}
}

// SetTenantStatus supports tenant lifecycle checks in the in-memory runtime
// and tests. Users and sessions remain stored, but inactive tenants cannot
// authenticate or use an existing session.
func (s *MemoryStore) SetTenantStatus(tenantID, status string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tenantStatuses[tenantID] = status
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
	if !ok || user.Status != "active" || s.tenantStatuses[user.TenantID] != "active" {
		return UserWithPassword{}, ErrInvalidCredentials
	}
	return user, nil
}

func (s *MemoryStore) ResolveAccessScope(_ context.Context, user User) (AccessScope, error) {
	scope, err := ResolveDeclaredAccessScope(user)
	if err != nil {
		return AccessScope{}, err
	}
	// Synthetic unit fixtures may explicitly request a tenant-wide in-memory
	// boundary. Production scopes never receive this test-only expansion.
	if scopeDeclaresSynthetic(user.DataScope) {
		scope.syntheticUnbounded = true
	}
	return scope, nil
}

func (s *MemoryStore) CreateSession(_ context.Context, input CreateSessionInput) (DeviceSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessionSeq++
	now := time.Now().UTC()
	record := memorySession{
		ID: fmt.Sprintf("memory-session-%d", s.sessionSeq), TenantID: input.TenantID, UserID: input.UserID,
		SessionType: input.SessionType, DeviceName: input.DeviceName,
		CreatedAt: now, LastSeenAt: now, ExpiresAt: input.ExpiresAt,
	}
	s.sessions[input.TokenHash] = record
	return deviceSessionFromMemory(record, true), nil
}

func (s *MemoryStore) FindUserBySession(_ context.Context, tokenHash string, now time.Time) (User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	session, ok := s.sessions[tokenHash]
	if !ok || !session.ExpiresAt.After(now) {
		return User{}, ErrUnauthenticated
	}
	if now.Sub(session.LastSeenAt) >= 5*time.Minute {
		session.LastSeenAt = now
		s.sessions[tokenHash] = session
	}
	for _, user := range s.users {
		if user.ID == session.UserID && user.TenantID == session.TenantID && user.Status == "active" && s.tenantStatuses[user.TenantID] == "active" {
			return user.User, nil
		}
	}
	return User{}, ErrUnauthenticated
}

func (s *MemoryStore) DeleteSession(_ context.Context, tokenHash string, _ string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, tokenHash)
	return nil
}

func (s *MemoryStore) ListSessions(_ context.Context, tenantID string, userID string, currentTokenHash string, now time.Time) ([]DeviceSession, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []DeviceSession{}
	for tokenHash, record := range s.sessions {
		if record.TenantID != tenantID || record.UserID != userID || !record.ExpiresAt.After(now) {
			continue
		}
		out = append(out, deviceSessionFromMemory(record, tokenHash == currentTokenHash))
	}
	return out, nil
}

func (s *MemoryStore) RevokeSession(_ context.Context, tenantID string, userID string, sessionID string, _ string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for tokenHash, record := range s.sessions {
		if record.ID == sessionID && record.TenantID == tenantID && record.UserID == userID {
			delete(s.sessions, tokenHash)
			return true, nil
		}
	}
	return false, nil
}

func (s *MemoryStore) RevokeAllSessions(_ context.Context, tenantID string, userID string, _ string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	revoked := 0
	for tokenHash, record := range s.sessions {
		if record.TenantID == tenantID && record.UserID == userID {
			delete(s.sessions, tokenHash)
			revoked++
		}
	}
	return revoked, nil
}

func (s *MemoryStore) UpdatePasswordAndRevokeSessions(_ context.Context, tenantID, userID, expectedPasswordHash, newPasswordHash string) (bool, int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	found := false
	for key, user := range s.users {
		if user.TenantID == tenantID && user.ID == userID && user.PasswordHash == expectedPasswordHash {
			user.PasswordHash = newPasswordHash
			s.users[key] = user
			found = true
			break
		}
	}
	if !found {
		return false, 0, nil
	}
	revoked := 0
	for tokenHash, record := range s.sessions {
		if record.TenantID == tenantID && record.UserID == userID {
			delete(s.sessions, tokenHash)
			revoked++
		}
	}
	return true, revoked, nil
}

func deviceSessionFromMemory(record memorySession, current bool) DeviceSession {
	return DeviceSession{
		ID: record.ID, SessionType: record.SessionType, DeviceName: record.DeviceName,
		CreatedAt: record.CreatedAt, LastSeenAt: record.LastSeenAt, ExpiresAt: record.ExpiresAt, Current: current,
	}
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
	if filter.ScopeMode != "" && filter.ScopeMode != "tenant" && filter.ScopeMode != "platform" {
		return []AuditRecord{}, nil
	}
	if filter.ScopeMode != "" && filter.ScopeMode != "tenant" && filter.ScopeMode != "platform" {
		return []AuditRecord{}, nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []AuditRecord{}
	for _, record := range s.audits {
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
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].ID > out[j].ID
		}
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	if filter.CursorID != "" {
		start := 0
		for start < len(out) {
			item := out[start]
			if item.CreatedAt.Before(filter.CursorCreatedAt) ||
				(item.CreatedAt.Equal(filter.CursorCreatedAt) && item.ID < filter.CursorID) {
				break
			}
			start++
		}
		out = out[start:]
	}
	if limit := normalizedAuditLimit(filter.Limit); len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (s *MemoryStore) ListManagedUsers(_ context.Context, tenantID string, filter ManagedUserFilter) ([]ManagedUser, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []ManagedUser{}
	for _, user := range s.users {
		if user.TenantID != tenantID || (filter.UserID != "" && user.ID != filter.UserID) {
			continue
		}
		if filter.RestrictSchools && !containsID(filter.SchoolIDs, firstScopeID(user.DataScope, "school_id")) {
			continue
		}
		query := strings.ToLower(filter.Query)
		if query != "" && !strings.Contains(strings.ToLower(user.Username), query) && !strings.Contains(strings.ToLower(user.DisplayName), query) {
			continue
		}
		if filter.Role != "" {
			matched := false
			for _, role := range user.Roles {
				if role == filter.Role {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}
		out = append(out, ManagedUser{
			ID: user.ID, Username: user.Username, DisplayName: user.DisplayName,
			Status: user.Status, Roles: append([]string(nil), user.Roles...), SchoolID: firstScopeID(user.DataScope, "school_id"),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].ID > out[j].ID
		}
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	if filter.CursorID != "" {
		start := 0
		for start < len(out) {
			item := out[start]
			if item.CreatedAt.Before(filter.CursorCreatedAt) || (item.CreatedAt.Equal(filter.CursorCreatedAt) && item.ID < filter.CursorID) {
				break
			}
			start++
		}
		out = out[start:]
	}
	if filter.Limit > 0 && len(out) > filter.Limit {
		out = out[:filter.Limit]
	}
	return out, nil
}

func (s *MemoryStore) ListAssignableRoles(_ context.Context, actor User) ([]AssignableRole, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []AssignableRole{}
	for _, role := range s.roles[actor.TenantID] {
		if policy := RolePolicy(role.Code); CanAssignManagedRole(actor, role.Code) && role.ScopeType == policy.CanonicalScope {
			out = append(out, role)
		}
	}
	return out, nil
}

func (s *MemoryStore) CreateManagedUser(_ context.Context, actor User, actorScope AccessScope, input CreateManagedUserInput, passwordHash string) (ManagedUser, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := actor.TenantCode + "|" + input.Username
	if existing, exists := s.users[key]; exists && existing.User.Status != "deleted" {
		return ManagedUser{}, ErrUsernameExists
	}
	role, exists := s.roles[actor.TenantID][input.RoleCode]
	if !exists {
		return ManagedUser{}, ErrRoleNotFound
	}
	policy := RolePolicy(role.Code)
	if !CanAssignManagedRole(actor, role.Code) || role.ScopeType != policy.CanonicalScope {
		return ManagedUser{}, ErrRoleAssignment
	}
	if len(input.ClassIDs) > 0 && !policy.ClassBinding {
		return ManagedUser{}, ErrInvalidRoleBinding
	}
	schoolID := strings.TrimSpace(input.SchoolID)
	if schoolID == "" && HasRole(actor, "school_admin") && len(actorScope.SchoolIDs) == 1 {
		schoolID = actorScope.SchoolIDs[0]
	}
	if policy.SchoolRequired && schoolID == "" {
		return ManagedUser{}, ErrInvalidRoleBinding
	}
	if schoolID != "" && !actorScope.AllowsSchool(schoolID) {
		return ManagedUser{}, ErrOrganizationScope
	}
	s.userSeq++
	user := UserWithPassword{
		User: User{
			ID: fmt.Sprintf("managed-user-%d", s.userSeq), TenantID: actor.TenantID, TenantCode: actor.TenantCode,
			Username: input.Username, DisplayName: input.DisplayName, Status: "active",
			Roles: []string{input.RoleCode}, Permissions: []string{}, DataScope: map[string]any{input.RoleCode: canonicalRoleDataScope(input.RoleCode, schoolID)},
		},
		PasswordHash: passwordHash,
	}
	s.users[key] = user
	return ManagedUser{ID: user.ID, Username: user.Username, DisplayName: user.DisplayName, Status: user.Status, Roles: user.Roles, SchoolID: schoolID}, nil
}

func scopeDeclaresSynthetic(scope map[string]any) bool {
	for key, raw := range scope {
		if key == "synthetic" {
			value, _ := raw.(bool)
			if value {
				return true
			}
		}
		if nested, ok := raw.(map[string]any); ok && scopeDeclaresSynthetic(nested) {
			return true
		}
	}
	return false
}

func firstScopeID(scope map[string]any, key string) string {
	if value, ok := scope[key].(string); ok {
		return value
	}
	for _, raw := range scope {
		if nested, ok := raw.(map[string]any); ok {
			if value := firstScopeID(nested, key); value != "" {
				return value
			}
		}
	}
	return ""
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
