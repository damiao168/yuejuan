package idempotency

import (
	"context"
	"errors"
	"time"
)

var (
	ErrKeyConflict = errors.New("idempotency key reused with a different request")
	ErrInProgress  = errors.New("idempotent operation is in progress")
)

type BeginInput struct {
	TenantID      string
	ActorID       string
	Method        string
	Route         string
	Key           string
	RequestHash   string
	ExpiresAt     time.Time
	AllowTakeover bool
	StaleBefore   time.Time
}

type Record struct {
	State           string
	RequestHash     string
	ResponseStatus  int
	ResponseHeaders map[string]string
	ResponseBody    []byte
	UpdatedAt       time.Time
}

type Store interface {
	Begin(ctx context.Context, input BeginInput) (Record, bool, error)
	Complete(ctx context.Context, input BeginInput, status int, headers map[string]string, body []byte) error
	Abort(ctx context.Context, input BeginInput) error
}
