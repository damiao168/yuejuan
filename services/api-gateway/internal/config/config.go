package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Service       ServiceConfig
	Auth          AuthConfig
	Security      SecurityConfig
	Postgres      PostgresConfig
	Redis         RedisConfig
	MinIO         MinIOConfig
	Qdrant        QdrantConfig
	AIService     AIServiceConfig
	Files         FileConfig
	Observability ObservabilityConfig
	Barcode       BarcodeConfig
}

type ServiceConfig struct {
	Name              string
	Environment       string
	Host              string
	Port              int
	LogLevel          string
	ReadinessTimeout  time.Duration
	ShutdownTimeout   time.Duration
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
}

func (c ServiceConfig) Addr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

type PostgresConfig struct {
	DSN string
}

type AuthConfig struct {
	SessionTTL          time.Duration
	LoginFailureLimit   int
	LoginFailureWindow  time.Duration
	SessionCookieName   string
	SessionCookieSecure bool
}

type SecurityConfig struct {
	MaxHeaderBytes      int
	MaxRequestBodyBytes int64
	CORSAllowedOrigins  []string
	CORSAllowedMethods  []string
	CORSAllowedHeaders  []string
}

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

type MinIOConfig struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	UseSSL    bool
}

type QdrantConfig struct {
	URL    string
	APIKey string
}

type AIServiceConfig struct {
	URL           string
	Token         string
	Timeout       time.Duration
	MaxRetries    int
	ModelVersion  string
	PromptVersion string
	MinConfidence float64
}

type FileConfig struct {
	Bucket            string
	MaxUploadBytes    int64
	AllowedExtensions []string
}

type ObservabilityConfig struct {
	SlowRequestThreshold time.Duration
}

type BarcodeConfig struct {
	ActiveKeyID string
	HMACKeys    map[string][]byte
}

func Load(envFile string) (Config, error) {
	if envFile != "" {
		if err := loadDotEnv(envFile); err != nil {
			return Config{}, err
		}
	}

	environment := getEnv("EDUGRADE_ENV", "development")
	sessionCookieSecure := defaultSessionCookieSecure(environment)
	if raw, ok := os.LookupEnv("EDUGRADE_SESSION_COOKIE_SECURE"); ok && strings.TrimSpace(raw) != "" {
		if parsed, err := strconv.ParseBool(raw); err == nil {
			sessionCookieSecure = parsed
		}
	}

	cfg := Config{
		Service: ServiceConfig{
			Name:              getEnv("EDUGRADE_SERVICE_NAME", "api-gateway"),
			Environment:       environment,
			Host:              getEnv("EDUGRADE_HTTP_HOST", "127.0.0.1"),
			Port:              getEnvInt("EDUGRADE_HTTP_PORT", 8080),
			LogLevel:          getEnv("EDUGRADE_LOG_LEVEL", "info"),
			ReadinessTimeout:  getEnvDuration("EDUGRADE_READINESS_TIMEOUT", 2*time.Second),
			ShutdownTimeout:   getEnvDuration("EDUGRADE_SHUTDOWN_TIMEOUT", 10*time.Second),
			ReadHeaderTimeout: getEnvDuration("EDUGRADE_HTTP_READ_HEADER_TIMEOUT", 5*time.Second),
			ReadTimeout:       getEnvDuration("EDUGRADE_HTTP_READ_TIMEOUT", 15*time.Second),
			WriteTimeout:      getEnvDuration("EDUGRADE_HTTP_WRITE_TIMEOUT", 300*time.Second),
			IdleTimeout:       getEnvDuration("EDUGRADE_HTTP_IDLE_TIMEOUT", 60*time.Second),
		},
		Auth: AuthConfig{
			SessionTTL:          getEnvDuration("EDUGRADE_SESSION_TTL", 8*time.Hour),
			LoginFailureLimit:   getEnvInt("EDUGRADE_LOGIN_FAILURE_LIMIT", 5),
			LoginFailureWindow:  getEnvDuration("EDUGRADE_LOGIN_FAILURE_WINDOW", 15*time.Minute),
			SessionCookieName:   getEnv("EDUGRADE_SESSION_COOKIE_NAME", "edugrade_session"),
			SessionCookieSecure: sessionCookieSecure,
		},
		Security: SecurityConfig{
			MaxHeaderBytes:      getEnvInt("EDUGRADE_HTTP_MAX_HEADER_BYTES", 1<<20),
			MaxRequestBodyBytes: int64(getEnvInt("EDUGRADE_MAX_REQUEST_BODY_BYTES", 2*1024*1024)),
			CORSAllowedOrigins:  splitCSV(getEnv("EDUGRADE_CORS_ALLOWED_ORIGINS", "http://127.0.0.1:5173,http://127.0.0.1:5174,http://127.0.0.1:5180,http://localhost:5173,http://localhost:5174,http://localhost:5180")),
			CORSAllowedMethods:  splitCSV(getEnv("EDUGRADE_CORS_ALLOWED_METHODS", "GET,POST,PUT,PATCH,DELETE,OPTIONS")),
			CORSAllowedHeaders:  splitCSV(getEnv("EDUGRADE_CORS_ALLOWED_HEADERS", "Authorization,Content-Type,X-Request-ID,X-Trace-ID")),
		},
		Postgres: PostgresConfig{
			DSN: getEnv("EDUGRADE_POSTGRES_DSN", "postgres://edugrade:edugrade_dev@127.0.0.1:5432/edugrade?sslmode=disable"),
		},
		Redis: RedisConfig{
			Addr:     getEnv("EDUGRADE_REDIS_ADDR", "127.0.0.1:6379"),
			Password: getEnv("EDUGRADE_REDIS_PASSWORD", ""),
			DB:       getEnvInt("EDUGRADE_REDIS_DB", 0),
		},
		MinIO: MinIOConfig{
			Endpoint:  getEnv("EDUGRADE_MINIO_ENDPOINT", "127.0.0.1:9000"),
			AccessKey: getEnv("EDUGRADE_MINIO_ACCESS_KEY", "edugrade"),
			SecretKey: getEnv("EDUGRADE_MINIO_SECRET_KEY", "edugrade_dev_secret"),
			UseSSL:    getEnvBool("EDUGRADE_MINIO_USE_SSL", false),
		},
		Qdrant: QdrantConfig{
			URL:    getEnv("EDUGRADE_QDRANT_URL", ""),
			APIKey: getEnv("EDUGRADE_QDRANT_API_KEY", ""),
		},
		AIService: AIServiceConfig{
			URL:           getEnv("EDUGRADE_AI_SERVICE_URL", ""),
			Token:         getEnv("EDUGRADE_AI_SERVICE_TOKEN", ""),
			Timeout:       getEnvDuration("EDUGRADE_AI_SERVICE_TIMEOUT", 250*time.Second),
			MaxRetries:    getEnvInt("EDUGRADE_AI_SERVICE_MAX_RETRIES", 1),
			ModelVersion:  getEnv("EDUGRADE_AI_MODEL_VERSION", "Qwen/Qwen3-4B-GGUF:Q4_K_M"),
			PromptVersion: getEnv("EDUGRADE_AI_PROMPT_VERSION", "subjective-local-structured-v2"),
			MinConfidence: getEnvFloat("EDUGRADE_AI_MIN_CONFIDENCE", 0.8),
		},
		Files: FileConfig{
			Bucket:            getEnv("EDUGRADE_FILE_BUCKET", "edugrade-files"),
			MaxUploadBytes:    int64(getEnvInt("EDUGRADE_FILE_MAX_UPLOAD_BYTES", 104857600)),
			AllowedExtensions: splitCSV(getEnv("EDUGRADE_FILE_ALLOWED_EXTENSIONS", ".pdf,.png,.jpg,.jpeg,.csv,.docx")),
		},
		Observability: ObservabilityConfig{
			SlowRequestThreshold: getEnvDuration("EDUGRADE_SLOW_REQUEST_THRESHOLD", 2*time.Second),
		},
		Barcode: BarcodeConfig{
			ActiveKeyID: getEnv("EDUGRADE_BARCODE_ACTIVE_KEY_ID", "local-v1"),
			HMACKeys:    parseBarcodeKeys(getEnv("EDUGRADE_BARCODE_HMAC_KEYS", "")),
		},
	}
	if err := validateProductionConfig(cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func parseBarcodeKeys(raw string) map[string][]byte {
	keys := map[string][]byte{}
	for _, item := range strings.Split(raw, ",") {
		parts := strings.SplitN(strings.TrimSpace(item), ":", 2)
		if len(parts) == 2 && parts[0] != "" && len(parts[1]) >= 32 {
			keys[parts[0]] = []byte(parts[1])
		}
	}
	return keys
}

func validateProductionConfig(cfg Config) error {
	if strings.TrimSpace(cfg.AIService.URL) != "" {
		if len(cfg.AIService.Token) < 32 {
			return fmt.Errorf("unsafe AI service configuration: EDUGRADE_AI_SERVICE_TOKEN must contain at least 32 characters")
		}
		if cfg.AIService.Timeout <= 0 || cfg.AIService.MaxRetries < 0 || cfg.AIService.MaxRetries > 1 || cfg.AIService.MinConfidence < 0 || cfg.AIService.MinConfidence > 1 {
			return fmt.Errorf("invalid AI service timeout, retry, or confidence configuration")
		}
		if cfg.Service.WriteTimeout <= cfg.AIService.Timeout {
			return fmt.Errorf("EDUGRADE_HTTP_WRITE_TIMEOUT must exceed EDUGRADE_AI_SERVICE_TIMEOUT")
		}
	}
	if !isProductionLike(cfg.Service.Environment) {
		return nil
	}
	var problems []string
	if strings.TrimSpace(cfg.AIService.URL) == "" {
		problems = append(problems, "EDUGRADE_AI_SERVICE_URL must be configured for production-like environments")
	}
	if !cfg.Auth.SessionCookieSecure {
		problems = append(problems, "EDUGRADE_SESSION_COOKIE_SECURE must be true")
	}
	if strings.Contains(cfg.Postgres.DSN, "edugrade_dev") || strings.Contains(cfg.Postgres.DSN, "sslmode=disable") {
		problems = append(problems, "EDUGRADE_POSTGRES_DSN must not use development credentials or disabled TLS")
	}
	if cfg.MinIO.AccessKey == "edugrade" || cfg.MinIO.SecretKey == "edugrade_dev_secret" {
		problems = append(problems, "MinIO development credentials must be replaced")
	}
	if len(cfg.Barcode.HMACKeys[cfg.Barcode.ActiveKeyID]) < 32 {
		problems = append(problems, "EDUGRADE_BARCODE_HMAC_KEYS must contain the active key with at least 32 characters")
	}
	for _, origin := range cfg.Security.CORSAllowedOrigins {
		if strings.Contains(origin, "localhost") || strings.Contains(origin, "127.0.0.1") {
			problems = append(problems, "EDUGRADE_CORS_ALLOWED_ORIGINS must not contain local development origins")
			break
		}
	}
	if len(problems) > 0 {
		return fmt.Errorf("unsafe production configuration: %s", strings.Join(problems, "; "))
	}
	return nil
}

func isProductionLike(environment string) bool {
	switch strings.ToLower(strings.TrimSpace(environment)) {
	case "", "development", "dev", "test", "local":
		return false
	default:
		return true
	}
}

func defaultSessionCookieSecure(environment string) bool {
	return isProductionLike(environment)
}

func loadDotEnv(path string) error {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return fmt.Errorf("invalid env line: %q", line)
		}
		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), "\"'")
		if _, exists := os.LookupEnv(key); !exists {
			if err := os.Setenv(key, value); err != nil {
				return err
			}
		}
	}
	return scanner.Err()
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	value, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func getEnvBool(key string, fallback bool) bool {
	value, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	value, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func getEnvFloat(key string, fallback float64) float64 {
	value, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
