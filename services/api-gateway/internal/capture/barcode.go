package capture

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

const controlledBarcodePrefix = "EG1"

var ErrBarcodeInvalid = errors.New("controlled barcode invalid")

type BarcodeClaims struct {
	Version             int    `json:"v"`
	KeyID               string `json:"kid"`
	TenantID            string `json:"tenant_id"`
	ExamID              string `json:"exam_id"`
	TemplateID          string `json:"template_id"`
	TemplateContentHash string `json:"template_content_hash"`
	PageNo              int    `json:"page_no"`
	Nonce               string `json:"nonce"`
}

type BarcodeKeyring struct {
	ActiveKeyID string
	Keys        map[string][]byte
}

func (k BarcodeKeyring) Sign(claims BarcodeClaims) (string, error) {
	claims.Version = 1
	claims.KeyID = k.ActiveKeyID
	if err := validateBarcodeClaims(claims); err != nil {
		return "", err
	}
	key := k.Keys[claims.KeyID]
	if len(key) < 32 {
		return "", fmt.Errorf("%w: signing key unavailable", ErrBarcodeInvalid)
	}
	payload, err := json.Marshal(map[string]any{
		"exam_id": claims.ExamID, "kid": claims.KeyID, "nonce": claims.Nonce,
		"page_no": claims.PageNo, "template_content_hash": claims.TemplateContentHash,
		"template_id": claims.TemplateID, "tenant_id": claims.TenantID, "v": claims.Version,
	})
	if err != nil {
		return "", err
	}
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	signed := controlledBarcodePrefix + "." + encoded
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(signed))
	return signed + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func (k BarcodeKeyring) Verify(value string) (BarcodeClaims, error) {
	var claims BarcodeClaims
	if len(value) > 2048 {
		return claims, fmt.Errorf("%w: value_too_long", ErrBarcodeInvalid)
	}
	parts := strings.Split(value, ".")
	if len(parts) != 3 || parts[0] != controlledBarcodePrefix {
		return claims, fmt.Errorf("%w: format_invalid", ErrBarcodeInvalid)
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || json.Unmarshal(payload, &claims) != nil {
		return claims, fmt.Errorf("%w: payload_invalid", ErrBarcodeInvalid)
	}
	key := k.Keys[claims.KeyID]
	if len(key) < 32 {
		return claims, fmt.Errorf("%w: key_unknown", ErrBarcodeInvalid)
	}
	provided, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return claims, fmt.Errorf("%w: signature_invalid", ErrBarcodeInvalid)
	}
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(parts[0] + "." + parts[1]))
	if !hmac.Equal(provided, mac.Sum(nil)) {
		return claims, fmt.Errorf("%w: signature_invalid", ErrBarcodeInvalid)
	}
	if err = validateBarcodeClaims(claims); err != nil {
		return claims, err
	}
	return claims, nil
}

func validateBarcodeClaims(claims BarcodeClaims) error {
	if claims.Version != 1 || claims.KeyID == "" || claims.PageNo <= 0 || claims.Nonce == "" || claims.TemplateContentHash == "" {
		return fmt.Errorf("%w: claims_invalid", ErrBarcodeInvalid)
	}
	for _, value := range []string{claims.TenantID, claims.ExamID, claims.TemplateID} {
		if _, err := uuid.Parse(value); err != nil {
			return fmt.Errorf("%w: claims_invalid", ErrBarcodeInvalid)
		}
	}
	return nil
}
