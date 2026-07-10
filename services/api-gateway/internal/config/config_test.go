package config

import "testing"

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

func setSecureProductionEnvironment(t *testing.T) {
	t.Helper()
	t.Setenv("EDUGRADE_POSTGRES_DSN", "postgres://edugrade:strong-password@db.internal:5432/edugrade?sslmode=require")
	t.Setenv("EDUGRADE_MINIO_ACCESS_KEY", "production-access")
	t.Setenv("EDUGRADE_MINIO_SECRET_KEY", "production-secret")
	t.Setenv("EDUGRADE_CORS_ALLOWED_ORIGINS", "https://grading.example.edu")
}
