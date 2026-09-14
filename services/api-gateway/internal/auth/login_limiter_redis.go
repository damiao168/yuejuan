package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisLoginFailureLimiter struct {
	client   redis.UniversalClient
	limit    int
	window   time.Duration
	fallback *LoginFailureLimiter
}

func NewRedisLoginFailureLimiter(client redis.UniversalClient, limit int, window time.Duration) *RedisLoginFailureLimiter {
	return &RedisLoginFailureLimiter{client: client, limit: limit, window: window, fallback: NewLoginFailureLimiter(limit, window)}
}

func NewRedisLoginAttemptGuard(client redis.UniversalClient, limit int, window time.Duration) *LayeredLoginAttemptGuard {
	if limit <= 0 {
		limit = 5
	}
	sourceLimit := limit * 10
	if sourceLimit < 50 {
		sourceLimit = 50
	}
	return NewLayeredLoginAttemptGuard(
		NewRedisLoginFailureLimiter(client, limit, window),
		NewRedisLoginFailureLimiter(client, sourceLimit, window),
		NewRedisLoginFailureLimiter(client, limit, window),
	)
}

func (l *RedisLoginFailureLimiter) IsBlocked(ctx context.Context, key string, now time.Time) (time.Duration, bool) {
	if l == nil || l.client == nil || l.limit <= 0 {
		return 0, false
	}
	ctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	values, err := l.client.Pipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.Get(ctx, l.redisKey(key))
		pipe.PTTL(ctx, l.redisKey(key))
		return nil
	})
	if err != nil && err != redis.Nil {
		return l.fallback.IsBlocked(ctx, key, now)
	}
	if len(values) != 2 {
		return 0, false
	}
	count, countErr := values[0].(*redis.StringCmd).Int()
	ttl, ttlErr := values[1].(*redis.DurationCmd).Result()
	if countErr != nil || ttlErr != nil || count < l.limit || ttl <= 0 {
		return 0, false
	}
	return ttl, true
}

func (l *RedisLoginFailureLimiter) RegisterFailure(ctx context.Context, key string, now time.Time) (int, time.Duration, bool) {
	if l == nil || l.client == nil || l.limit <= 0 {
		return 0, 0, false
	}
	ctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	result, err := redisFailureScript.Run(ctx, l.client, []string{l.redisKey(key)}, l.window.Milliseconds()).Int64Slice()
	if err != nil || len(result) != 2 {
		return l.fallback.RegisterFailure(ctx, key, now)
	}
	count := int(result[0])
	retry := time.Duration(result[1]) * time.Millisecond
	return count, retry, count >= l.limit
}

func (l *RedisLoginFailureLimiter) Clear(ctx context.Context, key string) {
	if l == nil || l.client == nil {
		return
	}
	// The fallback may contain failures recorded during an earlier Redis
	// outage. Clear it even when Redis has recovered so a later outage cannot
	// resurrect stale lockout state after a successful login.
	l.fallback.Clear(ctx, key)
	ctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	_ = l.client.Del(ctx, l.redisKey(key)).Err()
}

func (l *RedisLoginFailureLimiter) redisKey(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return "edugrade:auth:login-fail:" + hex.EncodeToString(sum[:])
}

var redisFailureScript = redis.NewScript(`
local count = redis.call('INCR', KEYS[1])
if count == 1 then
  redis.call('PEXPIRE', KEYS[1], ARGV[1])
end
local ttl = redis.call('PTTL', KEYS[1])
return {count, ttl}
`)
