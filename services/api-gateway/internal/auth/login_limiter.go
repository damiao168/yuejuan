package auth

import (
	"context"
	"strings"
	"sync"
	"time"
)

type LoginLimiter interface {
	IsBlocked(ctx context.Context, key string, now time.Time) (time.Duration, bool)
	RegisterFailure(ctx context.Context, key string, now time.Time) (int, time.Duration, bool)
	Clear(ctx context.Context, key string)
}

type LoginFailureLimiter struct {
	mu      sync.Mutex
	limit   int
	window  time.Duration
	entries map[string]loginFailureEntry
}

type loginFailureEntry struct {
	Count        int
	FirstFailure time.Time
	BlockedUntil time.Time
}

func NewLoginFailureLimiter(limit int, window time.Duration) *LoginFailureLimiter {
	if window <= 0 {
		window = 15 * time.Minute
	}
	return &LoginFailureLimiter{
		limit:   limit,
		window:  window,
		entries: map[string]loginFailureEntry{},
	}
}

func LoginFailureKey(tenantCode string, username string, ipAddress string) string {
	return strings.ToLower(strings.TrimSpace(tenantCode)) + "|" + strings.ToLower(strings.TrimSpace(username)) + "|" + strings.TrimSpace(ipAddress)
}

func (l *LoginFailureLimiter) IsBlocked(_ context.Context, key string, now time.Time) (time.Duration, bool) {
	if l == nil || l.limit <= 0 {
		return 0, false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	entry, ok := l.entries[key]
	if !ok {
		return 0, false
	}
	if entry.BlockedUntil.After(now) {
		return entry.BlockedUntil.Sub(now), true
	}
	if !entry.BlockedUntil.IsZero() || now.Sub(entry.FirstFailure) > l.window {
		delete(l.entries, key)
	}
	return 0, false
}

func (l *LoginFailureLimiter) RegisterFailure(_ context.Context, key string, now time.Time) (int, time.Duration, bool) {
	if l == nil || l.limit <= 0 {
		return 0, 0, false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	entry := l.entries[key]
	if entry.FirstFailure.IsZero() || now.Sub(entry.FirstFailure) > l.window {
		entry = loginFailureEntry{FirstFailure: now}
	}
	entry.Count++
	blocked := entry.Count >= l.limit
	var retryAfter time.Duration
	if blocked {
		entry.BlockedUntil = now.Add(l.window)
		retryAfter = l.window
	}
	l.entries[key] = entry
	return entry.Count, retryAfter, blocked
}

func (l *LoginFailureLimiter) Clear(_ context.Context, key string) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.entries, key)
}
