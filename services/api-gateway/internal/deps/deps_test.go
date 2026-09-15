package deps

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"edugrade-enterprise/services/api-gateway/internal/config"
)

func TestNewRedisCheckerConfiguresACLAndTLS(t *testing.T) {
	caFile := filepath.Join(t.TempDir(), "ca.crt")
	if err := os.WriteFile(caFile, []byte("not-a-certificate"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := NewRedisChecker(config.RedisConfig{
		Addr:       "redis:6379",
		TLSEnabled: true,
		TLSCAFile:  caFile,
	}); err == nil || !strings.Contains(err.Error(), "contains no valid certificates") {
		t.Fatalf("invalid Redis CA must fail startup, got %v", err)
	}

	checker, closeRedis, err := NewRedisChecker(config.RedisConfig{
		Addr:     "redis:6379",
		Username: "edugrade-api",
		Password: "test-password",
		DB:       2,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer closeRedis()
	options := checker.client.Options()
	if options.Username != "edugrade-api" || options.Password != "test-password" || options.DB != 2 {
		t.Fatalf("unexpected Redis ACL options: %#v", options)
	}
}

func TestCheckAllOnlyBlocksReadinessForRequiredDependencies(t *testing.T) {
	results, ready := CheckAll(context.Background(), time.Second, []Checker{
		StaticChecker{CheckerName: "postgres"},
		NotConfiguredChecker{CheckerName: "ai_service", DetailText: "optional"},
	})
	if !ready {
		t.Fatal("optional dependency must not block readiness")
	}
	if len(results) != 2 || !results[0].Required || results[1].Required || results[1].Status != "not_configured" {
		t.Fatalf("unexpected dependency results: %#v", results)
	}

	_, ready = CheckAll(context.Background(), time.Second, []Checker{
		StaticChecker{CheckerName: "postgres", Err: errors.New("offline")},
	})
	if ready {
		t.Fatal("required dependency failure must block readiness")
	}
}

func TestAuthRateLimiterDegradationBlocksReadiness(t *testing.T) {
	checker := NewAuthRateLimiterChecker(func() bool { return true })
	if err := checker.Check(context.Background()); err == nil {
		t.Fatal("degraded authentication rate limiter must block readiness")
	}
}

func TestAuthRateLimiterReadinessProbeCanRecoverState(t *testing.T) {
	degraded := true
	checker := NewAuthRateLimiterChecker(func() bool { return degraded }).WithProbe(func(context.Context) error {
		degraded = false
		return nil
	})
	if err := checker.Check(context.Background()); err != nil {
		t.Fatalf("successful probe must clear stale degraded state: %v", err)
	}

	probeFailure := errors.New("limiter write unavailable")
	checker = NewAuthRateLimiterChecker(func() bool { return false }).WithProbe(func(context.Context) error { return probeFailure })
	if err := checker.Check(context.Background()); !errors.Is(err, probeFailure) {
		t.Fatalf("probe failure must block readiness, got %v", err)
	}
}

func TestMinIOCheckerRequiresConfiguredBucketToExist(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/edugrade-files/" && r.URL.Query().Has("location") {
			w.Header().Set("Content-Type", "application/xml")
			_, _ = w.Write([]byte(`<LocationConstraint xmlns="http://s3.amazonaws.com/doc/2006-03-01/">us-east-1</LocationConstraint>`))
			return
		}
		if r.Method != http.MethodHead || r.URL.Path != "/edugrade-files/" {
			t.Fatalf("unexpected MinIO readiness request: %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	checker, err := NewMinIOChecker(config.MinIOConfig{
		Endpoint:  strings.TrimPrefix(server.URL, "http://"),
		AccessKey: "test-app-access",
		SecretKey: "test-app-secret",
	}, "edugrade-files")
	if err != nil {
		t.Fatal(err)
	}
	if err := checker.Check(context.Background()); err == nil || !strings.Contains(err.Error(), "restore it") {
		t.Fatalf("missing bucket must fail readiness with recovery guidance, got %v", err)
	}
}

func TestPostgresCheckerDoesNotOverrideConfiguredPoolLimits(t *testing.T) {
	database, err := sql.Open("pgx", "postgres://test:test@127.0.0.1:5432/test?sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	database.SetMaxOpenConns(17)

	_ = NewPostgresChecker(database)
	if database.Stats().MaxOpenConnections != 17 {
		t.Fatalf("readiness checker changed configured pool ceiling: %d", database.Stats().MaxOpenConnections)
	}
}
