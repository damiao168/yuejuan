package modelgovernance

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHTTPRuntimePromptSourceReadsAuthenticatedRuntimeSnapshot(t *testing.T) {
	const token = "synthetic-service-token-with-32-characters"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/grading/prompts/current" || r.Header.Get("Authorization") != "Bearer "+token {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"prompt":{"prompt_version":"subjective-v2","bundle_sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","components":[{"key":"base","filename":"base.md","sha256":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","content":"Return JSON only."},{"key":"short_answer","filename":"short.md","sha256":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","content":"Short answer."},{"key":"calculation","filename":"calculation.md","sha256":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","content":"Calculation."},{"key":"essay","filename":"essay.md","sha256":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","content":"Essay."},{"key":"discussion","filename":"discussion.md","sha256":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","content":"Discussion."},{"key":"structured","filename":"structured.md","sha256":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","content":"Structured."}],"activation_mode":"deployment_manifest","mutable_at_runtime":false}}`))
	}))
	defer server.Close()

	source := NewHTTPRuntimePromptSource(server.URL, token, time.Second)
	prompt, err := source.Current(context.Background())
	if err != nil {
		t.Fatalf("read runtime prompt: %v", err)
	}
	if prompt.PromptVersion != "subjective-v2" || len(prompt.Components) != 6 || prompt.Components[0].Content != "Return JSON only." {
		t.Fatalf("unexpected prompt snapshot: %#v", prompt)
	}
}

func TestHTTPRuntimePromptSourceFailsClosedOnInvalidSnapshot(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"prompt":{"prompt_version":"","bundle_sha256":"bad","components":[],"activation_mode":"deployment_manifest","mutable_at_runtime":false}}`))
	}))
	defer server.Close()

	source := NewHTTPRuntimePromptSource(server.URL, "synthetic-service-token-with-32-characters", time.Second)
	if _, err := source.Current(context.Background()); err == nil {
		t.Fatal("expected invalid runtime prompt to fail closed")
	}
}
