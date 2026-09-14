package idempotency

import (
	"net/http"
	"strings"

	"edugrade-enterprise/services/api-gateway/internal/commandreceipt"
	"edugrade-enterprise/services/api-gateway/internal/httpx"
)

// CommandIdentity validates the existing key format for transactional command
// handlers. It deliberately does not replay the HTTP response cache: handlers
// recheck current private-content ACLs before returning a durable receipt.
func CommandIdentity(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			next.ServeHTTP(w, r)
			return
		}
		key := strings.TrimSpace(r.Header.Get(Header))
		if key == "" {
			httpx.Error(w, r, 400, "idempotency_key_required", "this operation requires an Idempotency-Key")
			return
		}
		if !keyPattern.MatchString(key) {
			httpx.Error(w, r, 400, "invalid_idempotency_key", "Idempotency-Key format is invalid")
			return
		}
		next.ServeHTTP(w, r.WithContext(commandreceipt.WithID(r.Context(), key)))
	})
}
