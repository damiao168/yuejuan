package auth

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestRedisLoginLimiterReportsDegradedState(t *testing.T) {
	client := redis.NewClient(&redis.Options{
		Addr:        "127.0.0.1:1",
		DialTimeout: 20 * time.Millisecond,
		ReadTimeout: 20 * time.Millisecond,
	})
	defer client.Close()
	guard := NewRedisLoginAttemptGuard(client, 2, time.Minute)

	guard.Check(context.Background(), LoginAttempt{TenantCode: "demo", Identifier: "teacher", IPAddress: "127.0.0.1"}, time.Now())

	if !guard.Degraded() {
		t.Fatal("Redis failure must mark distributed login limiting as degraded")
	}
}

func TestLayeredLoginLimiterBlocksDistributedAttackAgainstAccount(t *testing.T) {
	guard := NewLayeredLoginAttemptGuard(
		NewLoginFailureLimiter(2, time.Hour),
		NewLoginFailureLimiter(100, time.Hour),
		NewLoginFailureLimiter(100, time.Hour),
	)
	now := time.Now().UTC()
	first := LoginAttempt{TenantCode: "demo", Identifier: "teacher", IPAddress: "203.0.113.1"}
	second := LoginAttempt{TenantCode: "demo", Identifier: "teacher", IPAddress: "203.0.113.2"}
	if _, blocked := guard.RegisterFailure(context.Background(), first, now); blocked {
		t.Fatal("first failure should not block")
	}
	limit, blocked := guard.RegisterFailure(context.Background(), second, now)
	if !blocked || limit.Bucket != "account" {
		t.Fatalf("distributed failures must block account bucket, got %#v blocked=%v", limit, blocked)
	}
}

func TestLayeredLoginLimiterBlocksCredentialStuffingSource(t *testing.T) {
	guard := NewLayeredLoginAttemptGuard(
		NewLoginFailureLimiter(100, time.Hour),
		NewLoginFailureLimiter(2, time.Hour),
		NewLoginFailureLimiter(100, time.Hour),
	)
	now := time.Now().UTC()
	first := LoginAttempt{TenantCode: "demo", Identifier: "teacher-1", IPAddress: "203.0.113.9"}
	second := LoginAttempt{TenantCode: "demo", Identifier: "teacher-2", IPAddress: "203.0.113.9"}
	guard.RegisterFailure(context.Background(), first, now)
	limit, blocked := guard.RegisterFailure(context.Background(), second, now)
	if !blocked || limit.Bucket != "source" {
		t.Fatalf("multi-account failures must block source bucket, got %#v blocked=%v", limit, blocked)
	}
}

func TestSuccessfulLoginDoesNotClearSourceHistory(t *testing.T) {
	source := NewLoginFailureLimiter(2, time.Hour)
	guard := NewLayeredLoginAttemptGuard(NewLoginFailureLimiter(10, time.Hour), source, NewLoginFailureLimiter(10, time.Hour))
	now := time.Now().UTC()
	first := LoginAttempt{TenantCode: "demo", Identifier: "teacher-1", IPAddress: "203.0.113.9"}
	second := LoginAttempt{TenantCode: "demo", Identifier: "teacher-2", IPAddress: "203.0.113.9"}
	guard.RegisterFailure(context.Background(), first, now)
	guard.RegisterSuccess(context.Background(), first)
	limit, blocked := guard.RegisterFailure(context.Background(), second, now)
	if !blocked || limit.Bucket != "source" {
		t.Fatalf("account success must not erase source history, got %#v blocked=%v", limit, blocked)
	}
}

func TestLoginLimiterCanonicalizesPhoneAndIPAddressForms(t *testing.T) {
	if left, right := LoginAccountFailureKey("DEMO", "138 0013 8000"), LoginAccountFailureKey("demo", "+86 13800138000"); left != right {
		t.Fatalf("equivalent phone identifiers must share an account bucket: %q != %q", left, right)
	}
	if left, right := LoginSourceFailureKey("2001:0db8:0:0:0:0:0:1"), LoginSourceFailureKey("2001:db8::1"); left != right {
		t.Fatalf("equivalent IP forms must share a source bucket: %q != %q", left, right)
	}
}

func TestLoginLimiterSharesResolvedAccountBucketAcrossAliases(t *testing.T) {
	guard := NewMemoryLoginAttemptGuard(2, time.Hour)
	now := time.Now().UTC()
	first := LoginAttempt{TenantCode: "demo", Identifier: "teacher", AccountID: "user-1", IPAddress: "203.0.113.1"}
	second := LoginAttempt{TenantCode: "demo", Identifier: "13800138000", AccountID: "user-1", IPAddress: "203.0.113.2"}
	third := LoginAttempt{TenantCode: "demo", Identifier: "EMP-001", AccountID: "user-1", IPAddress: "203.0.113.3"}
	guard.RegisterFailure(context.Background(), first, now)
	guard.RegisterFailure(context.Background(), second, now)
	if limit, blocked := guard.Check(context.Background(), third, now); !blocked || limit.Bucket != "account" {
		t.Fatalf("employee number must share the username and phone account lock: %#v blocked=%v", limit, blocked)
	}
	third.AccountID = "user-2"
	if _, blocked := guard.Check(context.Background(), third, now); blocked {
		t.Fatal("another account must not inherit the resolved account lock")
	}
}
