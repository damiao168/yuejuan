package config

import (
	"bufio"
	"fmt"
	"net"
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
	DSN              string
	MaxOpenConns     int
	MaxIdleConns     int
	ConnMaxLifetime  time.Duration
	ConnMaxIdleTime  time.Duration
	StatementTimeout time.Duration
	LockTimeout      time.Duration
}

type AuthConfig struct {
	SessionTTL           time.Duration
	RememberedSessionTTL time.Duration
	LoginFailureLimit    int
	LoginFailureWindow   time.Duration
	SessionCookieName    string
	SessionCookieSecure  bool
}

type SecurityConfig struct {
	MaxHeaderBytes      int
	MaxRequestBodyBytes int64
	CORSAllowedOrigins  []string
	CORSAllowedMethods  []string
	CORSAllowedHeaders  []string
	TrustedProxyCIDRs   []string
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
	Enabled           bool
	AllowMock         bool
	URL               string
	Token             string
	Timeout           time.Duration
	MaxRetries        int
	ModelVersion      string
	PromptVersion     string
	MinConfidence     float64
	ProviderKey       string
	DeploymentKey     string
	AdapterType       string
	DeploymentRegion  string
	CapabilityProfile string
}

type FileConfig struct {
	Bucket                    string
	MaxUploadBytes            int64
	AllowedExtensions         []string
	ReconciliationInterval    time.Duration
	ReconciliationStaleAfter  time.Duration
	ReconciliationBatchSize   int
	ReconciliationObjectLimit int
}

type ObservabilityConfig struct {
	SlowRequestThreshold      time.Duration
	WorkerHeartbeatStaleAfter time.Duration
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
	secretDefaults := map[string]string{
		"EDUGRADE_POSTGRES_DSN":      "postgres://edugrade:edugrade_dev@127.0.0.1:5432/edugrade?sslmode=disable",
		"EDUGRADE_REDIS_PASSWORD":    "",
		"EDUGRADE_MINIO_ACCESS_KEY":  "edugrade",
		"EDUGRADE_MINIO_SECRET_KEY":  "edugrade_dev_secret",
		"EDUGRADE_QDRANT_API_KEY":    "",
		"EDUGRADE_AI_SERVICE_TOKEN":  "",
		"EDUGRADE_BARCODE_HMAC_KEYS": "",
	}
	secretValues := make(map[string]string, len(secretDefaults))
	for key, fallback := range secretDefaults {
		value, err := getEnvOrFile(key, fallback)
		if err != nil {
			return Config{}, err
		}
		secretValues[key] = value
	}
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
			WriteTimeout:      getEnvDuration("EDUGRADE_HTTP_WRITE_TIMEOUT", 780*time.Second),
			IdleTimeout:       getEnvDuration("EDUGRADE_HTTP_IDLE_TIMEOUT", 60*time.Second),
		},
		Auth: AuthConfig{
			SessionTTL:           getEnvDuration("EDUGRADE_SESSION_TTL", 8*time.Hour),
			RememberedSessionTTL: getEnvDuration("EDUGRADE_REMEMBERED_SESSION_TTL", 30*24*time.Hour),
			LoginFailureLimit:    getEnvInt("EDUGRADE_LOGIN_FAILURE_LIMIT", 5),
			LoginFailureWindow:   getEnvDuration("EDUGRADE_LOGIN_FAILURE_WINDOW", 15*time.Minute),
			SessionCookieName:    getEnv("EDUGRADE_SESSION_COOKIE_NAME", "edugrade_session"),
			SessionCookieSecure:  sessionCookieSecure,
		},
		Security: SecurityConfig{
			MaxHeaderBytes:      getEnvInt("EDUGRADE_HTTP_MAX_HEADER_BYTES", 1<<20),
			MaxRequestBodyBytes: int64(getEnvInt("EDUGRADE_MAX_REQUEST_BODY_BYTES", 2*1024*1024)),
			CORSAllowedOrigins:  splitCSV(getEnv("EDUGRADE_CORS_ALLOWED_ORIGINS", "http://127.0.0.1:5173,http://127.0.0.1:5174,http://127.0.0.1:5180,http://localhost:5173,http://localhost:5174,http://localhost:5180")),
			CORSAllowedMethods:  splitCSV(getEnv("EDUGRADE_CORS_ALLOWED_METHODS", "GET,POST,PUT,PATCH,DELETE,OPTIONS")),
			CORSAllowedHeaders:  splitCSV(getEnv("EDUGRADE_CORS_ALLOWED_HEADERS", "Authorization,Content-Type,X-Request-ID,X-Trace-ID,Idempotency-Key,X-EduGrade-CSRF")),
			TrustedProxyCIDRs:   splitCSV(getEnv("EDUGRADE_TRUSTED_PROXY_CIDRS", "")),
		},
		Postgres: PostgresConfig{
			DSN:              secretValues["EDUGRADE_POSTGRES_DSN"],
			MaxOpenConns:     getEnvInt("EDUGRADE_POSTGRES_MAX_OPEN_CONNS", 10),
			MaxIdleConns:     getEnvInt("EDUGRADE_POSTGRES_MAX_IDLE_CONNS", 5),
			ConnMaxLifetime:  getEnvDuration("EDUGRADE_POSTGRES_CONN_MAX_LIFETIME", 30*time.Minute),
			ConnMaxIdleTime:  getEnvDuration("EDUGRADE_POSTGRES_CONN_MAX_IDLE_TIME", 5*time.Minute),
			StatementTimeout: getEnvDuration("EDUGRADE_POSTGRES_STATEMENT_TIMEOUT", 60*time.Second),
			LockTimeout:      getEnvDuration("EDUGRADE_POSTGRES_LOCK_TIMEOUT", 5*time.Second),
		},
		Redis: RedisConfig{
			Addr:     getEnv("EDUGRADE_REDIS_ADDR", "127.0.0.1:6379"),
			Password: secretValues["EDUGRADE_REDIS_PASSWORD"],
			DB:       getEnvInt("EDUGRADE_REDIS_DB", 0),
		},
		MinIO: MinIOConfig{
			Endpoint:  getEnv("EDUGRADE_MINIO_ENDPOINT", "127.0.0.1:9000"),
			AccessKey: secretValues["EDUGRADE_MINIO_ACCESS_KEY"],
			SecretKey: secretValues["EDUGRADE_MINIO_SECRET_KEY"],
			UseSSL:    getEnvBool("EDUGRADE_MINIO_USE_SSL", false),
		},
		Qdrant: QdrantConfig{
			URL:    getEnv("EDUGRADE_QDRANT_URL", ""),
			APIKey: secretValues["EDUGRADE_QDRANT_API_KEY"],
		},
		AIService: AIServiceConfig{
			Enabled:           getEnvBool("EDUGRADE_AI_GRADING_ENABLED", false),
			AllowMock:         getEnvBool("EDUGRADE_ALLOW_MOCK_AI", false),
			URL:               getEnv("EDUGRADE_AI_SERVICE_URL", ""),
			Token:             secretValues["EDUGRADE_AI_SERVICE_TOKEN"],
			Timeout:           getEnvDuration("EDUGRADE_AI_SERVICE_TIMEOUT", 750*time.Second),
			MaxRetries:        getEnvInt("EDUGRADE_AI_SERVICE_MAX_RETRIES", 0),
			ModelVersion:      getEnv("EDUGRADE_AI_MODEL_VERSION", "Qwen/Qwen3-4B-GGUF:Q4_K_M"),
			PromptVersion:     getEnv("EDUGRADE_AI_PROMPT_VERSION", "subjective-governed-cn-subject-routing-v5"),
			MinConfidence:     getEnvFloat("EDUGRADE_AI_MIN_CONFIDENCE", 0.8),
			ProviderKey:       getEnv("EDUGRADE_AI_PROVIDER_KEY", "local"),
			DeploymentKey:     getEnv("EDUGRADE_AI_DEPLOYMENT_KEY", "local-qwen3-4b-q4-k-m"),
			AdapterType:       getEnv("EDUGRADE_AI_ADAPTER_TYPE", "local_llama_cpp"),
			DeploymentRegion:  getEnv("EDUGRADE_AI_DEPLOYMENT_REGION", "on_premise"),
			CapabilityProfile: getEnv("EDUGRADE_AI_CAPABILITY_PROFILE", "local-pilot-v1"),
		},
		Files: FileConfig{
			Bucket:                    getEnv("EDUGRADE_FILE_BUCKET", "edugrade-files"),
			MaxUploadBytes:            int64(getEnvInt("EDUGRADE_FILE_MAX_UPLOAD_BYTES", 104857600)),
			AllowedExtensions:         splitCSV(getEnv("EDUGRADE_FILE_ALLOWED_EXTENSIONS", ".pdf,.png,.jpg,.jpeg,.csv,.docx")),
			ReconciliationInterval:    getEnvDuration("EDUGRADE_FILE_RECONCILIATION_INTERVAL", 24*time.Hour),
			ReconciliationStaleAfter:  getEnvDuration("EDUGRADE_FILE_RECONCILIATION_STALE_AFTER", time.Hour),
			ReconciliationBatchSize:   getEnvInt("EDUGRADE_FILE_RECONCILIATION_BATCH_SIZE", 200),
			ReconciliationObjectLimit: getEnvInt("EDUGRADE_FILE_RECONCILIATION_OBJECT_LIMIT", 1000),
		},
		Observability: ObservabilityConfig{
			SlowRequestThreshold:      getEnvDuration("EDUGRADE_SLOW_REQUEST_THRESHOLD", 2*time.Second),
			WorkerHeartbeatStaleAfter: getEnvDuration("EDUGRADE_WORKER_HEARTBEAT_STALE_AFTER", 30*time.Second),
		},
		Barcode: BarcodeConfig{
			ActiveKeyID: getEnv("EDUGRADE_BARCODE_ACTIVE_KEY_ID", "local-v1"),
			HMACKeys:    parseBarcodeKeys(secretValues["EDUGRADE_BARCODE_HMAC_KEYS"]),
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
	if err := validatePostgresCapacity(cfg.Postgres); err != nil {
		return err
	}
	for _, cidr := range cfg.Security.TrustedProxyCIDRs {
		if _, _, err := net.ParseCIDR(cidr); err != nil {
			return fmt.Errorf("invalid EDUGRADE_TRUSTED_PROXY_CIDRS entry %q", cidr)
		}
	}
	if cfg.AIService.Enabled && strings.TrimSpace(cfg.AIService.URL) != "" {
		if len(cfg.AIService.Token) < 32 {
			return fmt.Errorf("unsafe AI service configuration: EDUGRADE_AI_SERVICE_TOKEN must contain at least 32 characters")
		}
		if cfg.AIService.Timeout <= 0 || cfg.AIService.MaxRetries < 0 || cfg.AIService.MaxRetries > 1 || cfg.AIService.MinConfidence < 0 || cfg.AIService.MinConfidence > 1 {
			return fmt.Errorf("invalid AI service timeout, retry, or confidence configuration")
		}
		if cfg.Service.WriteTimeout <= cfg.AIService.Timeout {
			return fmt.Errorf("EDUGRADE_HTTP_WRITE_TIMEOUT must exceed EDUGRADE_AI_SERVICE_TIMEOUT")
		}
		identity := map[string]string{
			"EDUGRADE_AI_PROVIDER_KEY":       cfg.AIService.ProviderKey,
			"EDUGRADE_AI_DEPLOYMENT_KEY":     cfg.AIService.DeploymentKey,
			"EDUGRADE_AI_ADAPTER_TYPE":       cfg.AIService.AdapterType,
			"EDUGRADE_AI_DEPLOYMENT_REGION":  cfg.AIService.DeploymentRegion,
			"EDUGRADE_AI_CAPABILITY_PROFILE": cfg.AIService.CapabilityProfile,
		}
		for name, value := range identity {
			if strings.TrimSpace(value) == "" || len(value) > 128 || strings.ContainsAny(value, " \t\r\n") {
				return fmt.Errorf("invalid AI service identity: %s must be a non-empty bounded identifier", name)
			}
		}
	}
	environment := strings.ToLower(strings.TrimSpace(cfg.Service.Environment))
	if environment == "demo" {
		if cfg.AIService.Enabled && strings.TrimSpace(cfg.AIService.URL) == "" && !cfg.AIService.AllowMock {
			return fmt.Errorf("AI grading is enabled in demo but neither EDUGRADE_AI_SERVICE_URL nor EDUGRADE_ALLOW_MOCK_AI=true is configured")
		}
		return nil
	}
	if !isProductionLike(cfg.Service.Environment) {
		return nil
	}
	var problems []string
	if cfg.AIService.AllowMock {
		problems = append(problems, "EDUGRADE_ALLOW_MOCK_AI must be false in production-like environments")
	}
	if cfg.AIService.Enabled && strings.TrimSpace(cfg.AIService.URL) == "" {
		problems = append(problems, "EDUGRADE_AI_SERVICE_URL must be configured when AI grading is enabled")
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

func validatePostgresCapacity(cfg PostgresConfig) error {
	if cfg.MaxOpenConns < 1 || cfg.MaxOpenConns > 500 {
		return fmt.Errorf("EDUGRADE_POSTGRES_MAX_OPEN_CONNS must be between 1 and 500")
	}
	if cfg.MaxIdleConns < 0 || cfg.MaxIdleConns > cfg.MaxOpenConns {
		return fmt.Errorf("EDUGRADE_POSTGRES_MAX_IDLE_CONNS must be between 0 and EDUGRADE_POSTGRES_MAX_OPEN_CONNS")
	}
	if cfg.ConnMaxLifetime < time.Minute || cfg.ConnMaxLifetime > 24*time.Hour {
		return fmt.Errorf("EDUGRADE_POSTGRES_CONN_MAX_LIFETIME must be between 1m and 24h")
	}
	if cfg.ConnMaxIdleTime < 10*time.Second || cfg.ConnMaxIdleTime > cfg.ConnMaxLifetime {
		return fmt.Errorf("EDUGRADE_POSTGRES_CONN_MAX_IDLE_TIME must be between 10s and EDUGRADE_POSTGRES_CONN_MAX_LIFETIME")
	}
	if cfg.StatementTimeout < time.Second || cfg.StatementTimeout > 15*time.Minute {
		return fmt.Errorf("EDUGRADE_POSTGRES_STATEMENT_TIMEOUT must be between 1s and 15m")
	}
	if cfg.LockTimeout < 100*time.Millisecond || cfg.LockTimeout > cfg.StatementTimeout {
		return fmt.Errorf("EDUGRADE_POSTGRES_LOCK_TIMEOUT must be between 100ms and EDUGRADE_POSTGRES_STATEMENT_TIMEOUT")
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

func getEnvOrFile(key, fallback string) (string, error) {
	if value, ok := os.LookupEnv(key); ok {
		if filePath := strings.TrimSpace(os.Getenv(key + "_FILE")); filePath != "" {
			if strings.TrimSpace(value) != "" {
				return "", fmt.Errorf("%s and %s_FILE cannot both be set", key, key)
			}
			content, err := os.ReadFile(filePath)
			if err != nil {
				return "", fmt.Errorf("read %s_FILE: %w", key, err)
			}
			return strings.TrimRight(string(content), "\r\n"), nil
		}
		return value, nil
	}
	filePath := strings.TrimSpace(os.Getenv(key + "_FILE"))
	if filePath == "" {
		return fallback, nil
	}
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("read %s_FILE: %w", key, err)
	}
	return strings.TrimRight(string(content), "\r\n"), nil
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
