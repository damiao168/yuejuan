package modelgovernance

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"
)

var (
	ErrManagedConfigUnavailable = errors.New("managed model API configuration is unavailable")
	ErrInvalidManagedConfig     = errors.New("invalid managed model API configuration")
)

type ManagedAPIConfig struct {
	ID                   string     `json:"id"`
	TenantID             string     `json:"tenant_id"`
	ProviderKey          string     `json:"provider_key"`
	DisplayName          string     `json:"display_name"`
	AdapterType          string     `json:"adapter_type"`
	BaseURL              string     `json:"base_url"`
	ModelName            string     `json:"model_name"`
	ModelVersion         string     `json:"model_version"`
	Region               string     `json:"region"`
	CredentialConfigured bool       `json:"credential_configured"`
	CredentialHint       string     `json:"credential_hint,omitempty"`
	Status               string     `json:"status"`
	IsDefault            bool       `json:"is_default"`
	LastTestStatus       string     `json:"last_test_status"`
	LastTestMessage      string     `json:"last_test_message,omitempty"`
	LastTestedAt         *time.Time `json:"last_tested_at,omitempty"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
}

type ManagedAPIConfigInput struct {
	TenantID     string `json:"tenant_id"`
	ProviderKey  string `json:"provider_key"`
	DisplayName  string `json:"display_name"`
	AdapterType  string `json:"adapter_type"`
	BaseURL      string `json:"base_url"`
	ModelName    string `json:"model_name"`
	ModelVersion string `json:"model_version"`
	Region       string `json:"region"`
	APIKey       string `json:"api_key"`
	Status       string `json:"status"`
	IsDefault    bool   `json:"is_default"`
}

type ManagedAPIConfigUpdateInput struct {
	DisplayName  string `json:"display_name"`
	AdapterType  string `json:"adapter_type"`
	BaseURL      string `json:"base_url"`
	ModelName    string `json:"model_name"`
	ModelVersion string `json:"model_version"`
	Region       string `json:"region"`
	APIKey       string `json:"api_key,omitempty"`
	Status       string `json:"status"`
	IsDefault    bool   `json:"is_default"`
}

type ManagedAPIConnection struct {
	Config ManagedAPIConfig
	APIKey string
}

type ManagedAPIProbeResult struct {
	OK         bool   `json:"ok"`
	StatusCode int    `json:"status_code,omitempty"`
	LatencyMS  int64  `json:"latency_ms"`
	Message    string `json:"message"`
}

type ManagedAPIConfigStore interface {
	ListManagedAPIConfigs(ctx context.Context, tenantID string) ([]ManagedAPIConfig, error)
	CreateManagedAPIConfig(ctx context.Context, tenantID, actorID string, input ManagedAPIConfigInput) (ManagedAPIConfig, error)
	UpdateManagedAPIConfig(ctx context.Context, tenantID, id string, input ManagedAPIConfigUpdateInput) (ManagedAPIConfig, error)
	GetManagedAPIConnection(ctx context.Context, tenantID, id string) (ManagedAPIConnection, error)
	RecordManagedAPIProbe(ctx context.Context, tenantID, id string, result ManagedAPIProbeResult) (ManagedAPIConfig, error)
}

type ManagedAPIProber interface {
	Probe(ctx context.Context, connection ManagedAPIConnection) ManagedAPIProbeResult
}

// No default means the existing local parser remains in use. An explicitly
// selected but disabled/unreadable default must not silently switch providers.
func ResolveDefaultManagedAPI(ctx context.Context, store ManagedAPIConfigStore, tenantID string) (*ManagedAPIConnection, error) {
	items, err := store.ListManagedAPIConfigs(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if item.TenantID != tenantID || !item.IsDefault {
			continue
		}
		connection, err := store.GetManagedAPIConnection(ctx, tenantID, item.ID)
		if err != nil {
			return nil, err
		}
		if connection.Config.TenantID != tenantID || !connection.Config.IsDefault || connection.Config.Status != "active" {
			return nil, ErrManagedConfigUnavailable
		}
		return &connection, nil
	}
	return nil, nil
}

type CredentialCipher struct {
	aead cipher.AEAD
}

func NewCredentialCipher(masterKey string) (*CredentialCipher, error) {
	if len(strings.TrimSpace(masterKey)) < 32 {
		return nil, ErrManagedConfigUnavailable
	}
	key := sha256.Sum256([]byte(masterKey))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &CredentialCipher{aead: aead}, nil
}

func (c *CredentialCipher) Encrypt(plaintext, tenantID, configID string) ([]byte, []byte, error) {
	if c == nil || strings.TrimSpace(plaintext) == "" {
		return nil, nil, ErrInvalidManagedConfig
	}
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, err
	}
	ciphertext := c.aead.Seal(nil, nonce, []byte(plaintext), managedCredentialAAD(tenantID, configID))
	return ciphertext, nonce, nil
}

func (c *CredentialCipher) Decrypt(ciphertext, nonce []byte, tenantID, configID string) (string, error) {
	if c == nil || len(nonce) != c.aead.NonceSize() || len(ciphertext) < c.aead.Overhead() {
		return "", ErrManagedConfigUnavailable
	}
	plaintext, err := c.aead.Open(nil, nonce, ciphertext, managedCredentialAAD(tenantID, configID))
	if err != nil {
		return "", ErrManagedConfigUnavailable
	}
	return string(plaintext), nil
}

func managedCredentialAAD(tenantID, configID string) []byte {
	return []byte("edugrade:model-api:v1:" + tenantID + ":" + configID)
}

func normalizeManagedAPIInput(input ManagedAPIConfigInput, requireKey bool) (ManagedAPIConfigInput, error) {
	input.TenantID = strings.TrimSpace(input.TenantID)
	input.ProviderKey = strings.ToLower(strings.TrimSpace(input.ProviderKey))
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	input.AdapterType = strings.TrimSpace(input.AdapterType)
	input.BaseURL = strings.TrimRight(strings.TrimSpace(input.BaseURL), "/")
	input.ModelName = strings.TrimSpace(input.ModelName)
	input.ModelVersion = strings.TrimSpace(input.ModelVersion)
	input.Region = strings.TrimSpace(input.Region)
	input.APIKey = strings.TrimSpace(input.APIKey)
	input.Status = strings.TrimSpace(input.Status)
	if input.Status == "" {
		input.Status = "active"
	}
	if input.Region == "" {
		input.Region = "global"
	}
	if !governanceKey.MatchString(input.ProviderKey) ||
		(input.AdapterType != "openai_compatible" && input.AdapterType != "dashscope_native") ||
		(input.Status != "active" && input.Status != "disabled") ||
		!boundedManagedValue(input.DisplayName) || !boundedManagedValue(input.ModelName) ||
		!boundedManagedValue(input.ModelVersion) || !boundedManagedValue(input.Region) ||
		(requireKey && len(input.APIKey) < 16) || (!requireKey && input.APIKey != "" && len(input.APIKey) < 16) {
		return ManagedAPIConfigInput{}, ErrInvalidManagedConfig
	}
	parsed, err := url.Parse(input.BaseURL)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return ManagedAPIConfigInput{}, ErrInvalidManagedConfig
	}
	if input.AdapterType == "dashscope_native" && (parsed.Host != "dashscope.aliyuncs.com" || strings.TrimRight(parsed.Path, "/") != "/api/v1") {
		return ManagedAPIConfigInput{}, ErrInvalidManagedConfig
	}
	if len(input.APIKey) > 1024 || strings.ContainsAny(input.APIKey, "\r\n\x00") {
		return ManagedAPIConfigInput{}, ErrInvalidManagedConfig
	}
	return input, nil
}

func normalizeManagedAPIUpdate(input ManagedAPIConfigUpdateInput) (ManagedAPIConfigUpdateInput, error) {
	normalized, err := normalizeManagedAPIInput(ManagedAPIConfigInput{
		ProviderKey: "update-placeholder", DisplayName: input.DisplayName,
		AdapterType: input.AdapterType, BaseURL: input.BaseURL, ModelName: input.ModelName,
		ModelVersion: input.ModelVersion, Region: input.Region, APIKey: input.APIKey,
		Status: input.Status, IsDefault: input.IsDefault,
	}, false)
	if err != nil {
		return ManagedAPIConfigUpdateInput{}, err
	}
	return ManagedAPIConfigUpdateInput{
		DisplayName: normalized.DisplayName, AdapterType: normalized.AdapterType,
		BaseURL: normalized.BaseURL, ModelName: normalized.ModelName,
		ModelVersion: normalized.ModelVersion, Region: normalized.Region,
		APIKey: normalized.APIKey, Status: normalized.Status, IsDefault: normalized.IsDefault,
	}, nil
}

func boundedManagedValue(value string) bool {
	return value != "" && len(value) <= 256 && !strings.ContainsAny(value, "\r\n\x00")
}

func credentialHint(apiKey string) string {
	runes := []rune(strings.TrimSpace(apiKey))
	if len(runes) <= 4 {
		return "••••"
	}
	return fmt.Sprintf("•••• %s", string(runes[len(runes)-4:]))
}
