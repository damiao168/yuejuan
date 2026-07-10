package deps

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"edugrade-enterprise/services/api-gateway/internal/config"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/redis/go-redis/v9"
)

type Checker interface {
	Name() string
	Check(ctx context.Context) error
}

type CheckResult struct {
	Name       string `json:"name"`
	Status     string `json:"status"`
	Error      string `json:"error,omitempty"`
	Detail     string `json:"detail,omitempty"`
	DurationMS int64  `json:"duration_ms"`
	CheckedAt  string `json:"checked_at"`
}

var ErrNotConfigured = errors.New("dependency not configured")

type PostgresChecker struct {
	db *sql.DB
}

func NewPostgresChecker(db *sql.DB) *PostgresChecker {
	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(30 * time.Minute)
	return &PostgresChecker{db: db}
}

func (c *PostgresChecker) Name() string {
	return "postgres"
}

func (c *PostgresChecker) Check(ctx context.Context) error {
	return c.db.PingContext(ctx)
}

type RedisChecker struct {
	client *redis.Client
}

func NewRedisChecker(cfg config.RedisConfig) (*RedisChecker, func() error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})
	return &RedisChecker{client: client}, client.Close
}

func (c *RedisChecker) Name() string {
	return "redis"
}

func (c *RedisChecker) Check(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
}

type MinIOChecker struct {
	client *minio.Client
}

func NewMinIOChecker(cfg config.MinIOConfig) (*MinIOChecker, error) {
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, err
	}
	return &MinIOChecker{client: client}, nil
}

func (c *MinIOChecker) Name() string {
	return "minio"
}

func (c *MinIOChecker) Check(ctx context.Context) error {
	_, err := c.client.ListBuckets(ctx)
	return err
}

type HTTPChecker struct {
	checkerName string
	url         string
	headers     map[string]string
	detail      string
}

func NewQdrantChecker(cfg config.QdrantConfig) Checker {
	url := strings.TrimSpace(cfg.URL)
	if url == "" {
		return NotConfiguredChecker{CheckerName: "qdrant", DetailText: "EDUGRADE_QDRANT_URL 未配置/待接入"}
	}
	headers := map[string]string{}
	if cfg.APIKey != "" {
		headers["api-key"] = cfg.APIKey
	}
	return HTTPChecker{
		checkerName: "qdrant",
		url:         strings.TrimRight(url, "/") + "/healthz",
		headers:     headers,
		detail:      "Qdrant /healthz",
	}
}

func NewAIServiceChecker(cfg config.AIServiceConfig) Checker {
	url := strings.TrimSpace(cfg.URL)
	if url == "" {
		return NotConfiguredChecker{CheckerName: "ai_service", DetailText: "EDUGRADE_AI_SERVICE_URL 未配置/待接入"}
	}
	return HTTPChecker{
		checkerName: "ai_service",
		url:         strings.TrimRight(url, "/") + "/health",
		detail:      "AI service /health",
	}
}

func (c HTTPChecker) Name() string {
	return c.checkerName
}

func (c HTTPChecker) Detail() string {
	return c.detail
}

func (c HTTPChecker) Check(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url, nil)
	if err != nil {
		return err
	}
	for key, value := range c.headers {
		req.Header.Set(key, value)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("%s health endpoint returned %d", c.checkerName, resp.StatusCode)
	}
	return nil
}

type DetailedChecker interface {
	Detail() string
}

func CheckAll(ctx context.Context, timeout time.Duration, checkers []Checker) ([]CheckResult, bool) {
	results := make([]CheckResult, 0, len(checkers))
	allHealthy := true
	for _, checker := range checkers {
		checkCtx, cancel := context.WithTimeout(ctx, timeout)
		start := time.Now()
		err := checker.Check(checkCtx)
		duration := time.Since(start)
		cancel()
		result := CheckResult{
			Name:       checker.Name(),
			Status:     "ok",
			DurationMS: duration.Milliseconds(),
			CheckedAt:  time.Now().UTC().Format(time.RFC3339),
		}
		if detailed, ok := checker.(DetailedChecker); ok {
			result.Detail = detailed.Detail()
		}
		if err != nil {
			if errors.Is(err, ErrNotConfigured) {
				result.Status = "not_configured"
				if result.Detail == "" {
					result.Detail = "未配置/待接入"
				}
			} else {
				result.Status = "error"
				result.Error = err.Error()
			}
			allHealthy = false
		}
		results = append(results, result)
	}
	return results, allHealthy
}

type StaticChecker struct {
	CheckerName string
	Err         error
}

func (c StaticChecker) Name() string {
	return c.CheckerName
}

func (c StaticChecker) Check(context.Context) error {
	if c.Err != nil {
		return fmt.Errorf("%s: %w", c.CheckerName, c.Err)
	}
	return nil
}

type NotConfiguredChecker struct {
	CheckerName string
	DetailText  string
}

func (c NotConfiguredChecker) Name() string {
	return c.CheckerName
}

func (c NotConfiguredChecker) Detail() string {
	if c.DetailText == "" {
		return "未配置/待接入"
	}
	return c.DetailText
}

func (c NotConfiguredChecker) Check(context.Context) error {
	return fmt.Errorf("%s: %w", c.CheckerName, ErrNotConfigured)
}
