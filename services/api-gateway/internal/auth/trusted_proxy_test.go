package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRemoteIPOnlyTrustsConfiguredProxyChain(t *testing.T) {
	untrusted := NewHandler(NewMemoryStore(), 0)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "203.0.113.10:4312"
	req.Header.Set("X-Forwarded-For", "198.51.100.8")
	if got := untrusted.remoteIP(req); got != "203.0.113.10" {
		t.Fatalf("untrusted peer must not select client IP, got %q", got)
	}

	trusted := NewHandler(NewMemoryStore(), 0, HandlerOptions{TrustedProxyCIDRs: []string{"10.0.0.0/8"}})
	req.RemoteAddr = "10.0.0.5:4312"
	req.Header.Set("X-Forwarded-For", "198.51.100.8, 10.0.0.4")
	if got := trusted.remoteIP(req); got != "198.51.100.8" {
		t.Fatalf("trusted proxy chain should resolve first untrusted client, got %q", got)
	}

	req.Header.Set("X-Forwarded-For", "forged-not-an-ip")
	if got := trusted.remoteIP(req); got != "10.0.0.5" {
		t.Fatalf("malformed forwarding chain must fall back to peer, got %q", got)
	}
}
