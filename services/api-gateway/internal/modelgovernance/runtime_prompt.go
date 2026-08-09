package modelgovernance

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const maxRuntimePromptResponseBytes int64 = 512 * 1024

var ErrRuntimePromptUnavailable = errors.New("runtime prompt unavailable")

type RuntimePromptComponent struct {
	Key      string `json:"key"`
	Filename string `json:"filename"`
	SHA256   string `json:"sha256"`
	Content  string `json:"content"`
}

type RuntimePrompt struct {
	PromptVersion    string                   `json:"prompt_version"`
	BundleSHA256     string                   `json:"bundle_sha256"`
	Components       []RuntimePromptComponent `json:"components"`
	ActivationMode   string                   `json:"activation_mode"`
	MutableAtRuntime bool                     `json:"mutable_at_runtime"`
}

type RuntimePromptSource interface {
	Current(context.Context) (RuntimePrompt, error)
}

type HTTPRuntimePromptSource struct {
	baseURL string
	token   string
	client  *http.Client
}

func NewHTTPRuntimePromptSource(baseURL string, token string, timeout time.Duration) *HTTPRuntimePromptSource {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &HTTPRuntimePromptSource{
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		token:   token,
		client:  &http.Client{Timeout: timeout},
	}
}

func (s *HTTPRuntimePromptSource) Current(ctx context.Context) (RuntimePrompt, error) {
	if s == nil || s.baseURL == "" || len(s.token) < 32 {
		return RuntimePrompt{}, ErrRuntimePromptUnavailable
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.baseURL+"/grading/prompts/current", nil)
	if err != nil {
		return RuntimePrompt{}, ErrRuntimePromptUnavailable
	}
	req.Header.Set("Authorization", "Bearer "+s.token)
	req.Header.Set("Accept", "application/json")
	response, err := s.client.Do(req)
	if err != nil {
		return RuntimePrompt{}, ErrRuntimePromptUnavailable
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return RuntimePrompt{}, fmt.Errorf("%w: status %d", ErrRuntimePromptUnavailable, response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxRuntimePromptResponseBytes+1))
	if err != nil || int64(len(body)) > maxRuntimePromptResponseBytes {
		return RuntimePrompt{}, ErrRuntimePromptUnavailable
	}
	var envelope struct {
		Prompt RuntimePrompt `json:"prompt"`
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&envelope); err != nil || strings.TrimSpace(envelope.Prompt.PromptVersion) == "" || len(envelope.Prompt.BundleSHA256) != 64 {
		return RuntimePrompt{}, ErrRuntimePromptUnavailable
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return RuntimePrompt{}, ErrRuntimePromptUnavailable
	}
	if envelope.Prompt.ActivationMode != "deployment_manifest" || envelope.Prompt.MutableAtRuntime || len(envelope.Prompt.Components) != 6 {
		return RuntimePrompt{}, ErrRuntimePromptUnavailable
	}
	allowedKeys := map[string]bool{"base": true, "short_answer": true, "calculation": true, "essay": true, "discussion": true, "structured": true}
	seenKeys := make(map[string]bool, len(envelope.Prompt.Components))
	for _, component := range envelope.Prompt.Components {
		if !allowedKeys[component.Key] || seenKeys[component.Key] || component.Filename == "" || len(component.SHA256) != 64 || strings.TrimSpace(component.Content) == "" {
			return RuntimePrompt{}, ErrRuntimePromptUnavailable
		}
		seenKeys[component.Key] = true
	}
	return envelope.Prompt, nil
}
