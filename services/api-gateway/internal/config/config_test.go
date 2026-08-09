package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestGetEnvOrFileReadsSecretAndRejectsAmbiguousSources(t *testing.T) {
	key := "EDUGRADE_TEST_SECRET_SOURCE"
	path := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(path, []byte("from-file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(key+"_FILE", path)
	value, err := getEnvOrFile(key, "fallback")
	if err != nil || value != "from-file" {
		t.Fatalf("unexpected file secret: %q %v", value, err)
	}
	t.Setenv(key, "from-env")
	if _, err := getEnvOrFile(key, "fallback"); err == nil {
		t.Fatal("direct and file secret sources must not be accepted together")
	}
}

func TestLoadUsesDefaults(t *testing.T) {
	t.Setenv("EDUGRADE_HTTP_PORT", "")
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Service.Name != "api-gateway" {
		t.Fatalf("unexpected service name: %s", cfg.Service.Name)
	}
	if cfg.Service.Port == 0 {
		t.Fatal("expected non-zero default port")
	}
}

func TestLoadReadsEnvironment(t *testing.T) {
	t.Setenv("EDUGRADE_SERVICE_NAME", "test-service")
	t.Setenv("EDUGRADE_HTTP_PORT", "18080")
	t.Setenv("EDUGRADE_MINIO_USE_SSL", "true")
	t.Setenv("EDUGRADE_WORKER_HEARTBEAT_STALE_AFTER", "45s")
	t.Setenv("EDUGRADE_POSTGRES_MAX_OPEN_CONNS", "24")
	t.Setenv("EDUGRADE_POSTGRES_MAX_IDLE_CONNS", "8")
	t.Setenv("EDUGRADE_POSTGRES_CONN_MAX_LIFETIME", "45m")
	t.Setenv("EDUGRADE_POSTGRES_CONN_MAX_IDLE_TIME", "4m")
	t.Setenv("EDUGRADE_POSTGRES_STATEMENT_TIMEOUT", "40s")
	t.Setenv("EDUGRADE_POSTGRES_LOCK_TIMEOUT", "3s")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Service.Name != "test-service" {
		t.Fatalf("unexpected service name: %s", cfg.Service.Name)
	}
	if cfg.Service.Port != 18080 {
		t.Fatalf("unexpected port: %d", cfg.Service.Port)
	}
	if !cfg.MinIO.UseSSL {
		t.Fatal("expected MinIO SSL to be true")
	}
	if cfg.Observability.WorkerHeartbeatStaleAfter != 45*time.Second {
		t.Fatalf("unexpected worker heartbeat stale threshold: %s", cfg.Observability.WorkerHeartbeatStaleAfter)
	}
	if cfg.Postgres.MaxOpenConns != 24 || cfg.Postgres.MaxIdleConns != 8 ||
		cfg.Postgres.ConnMaxLifetime != 45*time.Minute || cfg.Postgres.ConnMaxIdleTime != 4*time.Minute ||
		cfg.Postgres.StatementTimeout != 40*time.Second || cfg.Postgres.LockTimeout != 3*time.Second {
		t.Fatalf("unexpected PostgreSQL capacity configuration: %#v", cfg.Postgres)
	}
}

func TestLoadRejectsUnsafePostgresCapacityConfiguration(t *testing.T) {
	t.Setenv("EDUGRADE_POSTGRES_MAX_OPEN_CONNS", "5")
	t.Setenv("EDUGRADE_POSTGRES_MAX_IDLE_CONNS", "6")
	if _, err := Load(""); err == nil {
		t.Fatal("idle connections must not exceed the open connection ceiling")
	}

	t.Setenv("EDUGRADE_POSTGRES_MAX_IDLE_CONNS", "2")
	t.Setenv("EDUGRADE_POSTGRES_LOCK_TIMEOUT", "2m")
	t.Setenv("EDUGRADE_POSTGRES_STATEMENT_TIMEOUT", "1m")
	if _, err := Load(""); err == nil {
		t.Fatal("lock timeout must not exceed the statement timeout")
	}
}

func TestLoadReadsGovernedAIServiceConfiguration(t *testing.T) {
	t.Setenv("EDUGRADE_AI_GRADING_ENABLED", "true")
	t.Setenv("EDUGRADE_AI_SERVICE_URL", "http://grading-agent:8100")
	t.Setenv("EDUGRADE_AI_SERVICE_TOKEN", "test-service-token-with-at-least-32-characters")
	t.Setenv("EDUGRADE_AI_SERVICE_TIMEOUT", "240s")
	t.Setenv("EDUGRADE_AI_SERVICE_MAX_RETRIES", "0")
	t.Setenv("EDUGRADE_AI_MODEL_VERSION", "model-v1")
	t.Setenv("EDUGRADE_AI_PROMPT_VERSION", "prompt-v2")
	t.Setenv("EDUGRADE_AI_MIN_CONFIDENCE", "0.75")
	t.Setenv("EDUGRADE_AI_PROVIDER_KEY", "local")
	t.Setenv("EDUGRADE_AI_DEPLOYMENT_KEY", "local-test-v1")
	t.Setenv("EDUGRADE_AI_ADAPTER_TYPE", "local_llama_cpp")
	t.Setenv("EDUGRADE_AI_DEPLOYMENT_REGION", "on_premise")
	t.Setenv("EDUGRADE_AI_CAPABILITY_PROFILE", "local-pilot-v1")

	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AIService.URL != "http://grading-agent:8100" ||
		cfg.AIService.Timeout != 240*time.Second ||
		cfg.AIService.MaxRetries != 0 ||
		cfg.AIService.ModelVersion != "model-v1" ||
		cfg.AIService.PromptVersion != "prompt-v2" ||
		cfg.AIService.MinConfidence != 0.75 ||
		cfg.AIService.ProviderKey != "local" ||
		cfg.AIService.DeploymentKey != "local-test-v1" ||
		cfg.AIService.AdapterType != "local_llama_cpp" ||
		cfg.AIService.DeploymentRegion != "on_premise" ||
		cfg.AIService.CapabilityProfile != "local-pilot-v1" {
		t.Fatalf("unexpected AI service configuration: %#v", cfg.AIService)
	}
}

func TestLoadRejectsIncompleteAIServiceIdentity(t *testing.T) {
	t.Setenv("EDUGRADE_AI_GRADING_ENABLED", "true")
	t.Setenv("EDUGRADE_AI_SERVICE_URL", "http://grading-agent:8100")
	t.Setenv("EDUGRADE_AI_SERVICE_TOKEN", "test-service-token-with-at-least-32-characters")
	t.Setenv("EDUGRADE_AI_PROVIDER_KEY", "contains whitespace")
	if _, err := Load(""); err == nil {
		t.Fatal("configured AI service must use bounded provider identity values")
	}
}

func TestLoadRejectsAIServiceWithoutStrongServiceToken(t *testing.T) {
	t.Setenv("EDUGRADE_AI_GRADING_ENABLED", "true")
	t.Setenv("EDUGRADE_AI_SERVICE_URL", "http://grading-agent:8100")
	t.Setenv("EDUGRADE_AI_SERVICE_TOKEN", "short")
	if _, err := Load(""); err == nil {
		t.Fatal("configured AI service must require a strong service token")
	}
}

func TestLoadRejectsHTTPWriteTimeoutShorterThanAIRequest(t *testing.T) {
	t.Setenv("EDUGRADE_AI_GRADING_ENABLED", "true")
	t.Setenv("EDUGRADE_AI_SERVICE_URL", "http://grading-agent:8100")
	t.Setenv("EDUGRADE_AI_SERVICE_TOKEN", "test-service-token-with-at-least-32-characters")
	t.Setenv("EDUGRADE_AI_SERVICE_TIMEOUT", "250s")
	t.Setenv("EDUGRADE_HTTP_WRITE_TIMEOUT", "15s")
	if _, err := Load(""); err == nil {
		t.Fatal("HTTP write timeout must outlive the synchronous grading request")
	}
}

func TestLoadParsesRotatableBarcodeKeys(t *testing.T) {
	t.Setenv("EDUGRADE_BARCODE_ACTIVE_KEY_ID", "v2")
	t.Setenv("EDUGRADE_BARCODE_HMAC_KEYS", "v1:11111111111111111111111111111111,v2:22222222222222222222222222222222")
	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Barcode.ActiveKeyID != "v2" || len(cfg.Barcode.HMACKeys) != 2 || len(cfg.Barcode.HMACKeys["v1"]) != 32 {
		t.Fatalf("unexpected barcode key configuration: active=%q keys=%d", cfg.Barcode.ActiveKeyID, len(cfg.Barcode.HMACKeys))
	}
}

func TestLoadDefaultsSessionCookieSecureForProduction(t *testing.T) {
	t.Setenv("EDUGRADE_ENV", "production")
	t.Setenv("EDUGRADE_SESSION_COOKIE_SECURE", "")
	setSecureProductionEnvironment(t)

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if !cfg.Auth.SessionCookieSecure {
		t.Fatal("production environment must default session cookie to Secure")
	}
}

func TestLoadRejectsInsecureProductionCookieOverride(t *testing.T) {
	t.Setenv("EDUGRADE_ENV", "production")
	t.Setenv("EDUGRADE_SESSION_COOKIE_SECURE", "false")
	setSecureProductionEnvironment(t)

	if _, err := Load(""); err == nil {
		t.Fatal("insecure production cookie override must be rejected")
	}
}

func TestLoadRejectsDevelopmentCredentialsInProduction(t *testing.T) {
	t.Setenv("EDUGRADE_ENV", "production")
	t.Setenv("EDUGRADE_SESSION_COOKIE_SECURE", "true")
	t.Setenv("EDUGRADE_POSTGRES_DSN", "postgres://edugrade:edugrade_dev@db.internal:5432/edugrade?sslmode=disable")
	t.Setenv("EDUGRADE_MINIO_ACCESS_KEY", "edugrade")
	t.Setenv("EDUGRADE_MINIO_SECRET_KEY", "edugrade_dev_secret")
	t.Setenv("EDUGRADE_CORS_ALLOWED_ORIGINS", "http://localhost:5173")
	if _, err := Load(""); err == nil {
		t.Fatal("development credentials must be rejected in production")
	}
}

func TestLoadRejectsMissingAIServiceInProduction(t *testing.T) {
	t.Setenv("EDUGRADE_ENV", "production")
	t.Setenv("EDUGRADE_AI_GRADING_ENABLED", "true")
	t.Setenv("EDUGRADE_SESSION_COOKIE_SECURE", "true")
	setSecureProductionEnvironment(t)
	t.Setenv("EDUGRADE_AI_SERVICE_URL", "")

	if _, err := Load(""); err == nil {
		t.Fatal("enabled production AI grading must require a real service")
	}
}

func TestLoadAllowsProductionWithAIGradingDisabled(t *testing.T) {
	t.Setenv("EDUGRADE_ENV", "production")
	t.Setenv("EDUGRADE_AI_GRADING_ENABLED", "false")
	t.Setenv("EDUGRADE_ALLOW_MOCK_AI", "false")
	setSecureProductionEnvironment(t)
	t.Setenv("EDUGRADE_AI_SERVICE_URL", "")
	t.Setenv("EDUGRADE_AI_SERVICE_TOKEN", "")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("disabled production AI grading should start safely: %v", err)
	}
	if cfg.AIService.Enabled {
		t.Fatal("AI grading must remain disabled")
	}
}

func TestLoadRejectsProductionMockAI(t *testing.T) {
	t.Setenv("EDUGRADE_ENV", "production")
	t.Setenv("EDUGRADE_AI_GRADING_ENABLED", "false")
	t.Setenv("EDUGRADE_ALLOW_MOCK_AI", "true")
	setSecureProductionEnvironment(t)

	if _, err := Load(""); err == nil {
		t.Fatal("production must reject mock AI even while grading is disabled")
	}
}

func TestLoadRequiresExplicitDemoMock(t *testing.T) {
	t.Setenv("EDUGRADE_ENV", "demo")
	t.Setenv("EDUGRADE_AI_GRADING_ENABLED", "true")
	t.Setenv("EDUGRADE_ALLOW_MOCK_AI", "false")
	t.Setenv("EDUGRADE_AI_SERVICE_URL", "")

	if _, err := Load(""); err == nil {
		t.Fatal("demo mock AI must be explicitly enabled")
	}
}

func setSecureProductionEnvironment(t *testing.T) {
	t.Helper()
	t.Setenv("EDUGRADE_AI_SERVICE_URL", "https://grading-agent.internal")
	t.Setenv("EDUGRADE_AI_SERVICE_TOKEN", "production-service-token-with-at-least-32-characters")
	t.Setenv("EDUGRADE_POSTGRES_DSN", "postgres://edugrade:strong-password@db.internal:5432/edugrade?sslmode=require")
	t.Setenv("EDUGRADE_MINIO_ACCESS_KEY", "production-access")
	t.Setenv("EDUGRADE_MINIO_SECRET_KEY", "production-secret")
	t.Setenv("EDUGRADE_CORS_ALLOWED_ORIGINS", "https://grading.example.edu")
	t.Setenv("EDUGRADE_BARCODE_ACTIVE_KEY_ID", "production-v1")
	t.Setenv("EDUGRADE_BARCODE_HMAC_KEYS", "production-v1:0123456789abcdef0123456789abcdef")
}
