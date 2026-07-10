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
	sessions map[string]memorySession
	audits   []AuditRecord
	auditSeq int
}

type memorySession struct {
	TenantID  string
	UserID    string
	ExpiresAt time.Time
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		users:    map[string]UserWithPassword{},
		sessions: map[string]memorySession{},
		audits:   []AuditRecord{},
	}
}

func (s *MemoryStore) AddUser(user UserWithPassword) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.users[user.TenantCode+"|"+user.Username] = user
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
