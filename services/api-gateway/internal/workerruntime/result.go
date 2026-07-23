package workerruntime

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

// CanonicalResultJSON returns the deterministic JSON envelope used when
// calculating a result payload hash. encoding/json sorts map keys, so
// semantically identical result maps produce identical bytes.
func CanonicalResultJSON(schema string, result map[string]any) ([]byte, error) {
	return json.Marshal(struct {
		Schema  string         `json:"schema"`
		Payload map[string]any `json:"payload"`
	}{
		Schema:  schema,
		Payload: result,
	})
}

// ResultPayloadHash returns the canonical SHA-256 hash used for idempotent
// completion checks and trusted imports.
func ResultPayloadHash(schema string, result map[string]any) (string, error) {
	raw, err := CanonicalResultJSON(schema, result)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

func payloadHash(schema string, payload map[string]any) string {
	hash, _ := ResultPayloadHash(schema, payload)
	return hash
}
