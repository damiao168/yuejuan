package capture

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

func (s *PostgresStore) IssueTemplateBarcodes(ctx context.Context, tenantID, templateID string) (IssuedTemplateBarcodes, error) {
	var examID, contentHash string
	var layoutRaw []byte
	if err := s.db.QueryRowContext(ctx, `SELECT exam_id::text,content_hash,layout FROM answer_sheet_template WHERE tenant_id=$1 AND id=$2::uuid AND status='locked' AND deleted_at IS NULL`, tenantID, templateID).Scan(&examID, &contentHash, &layoutRaw); err != nil {
		return IssuedTemplateBarcodes{}, mapNotFound(err)
	}
	var layout templateLayout
	if json.Unmarshal(layoutRaw, &layout) != nil || len(layout.Pages) == 0 {
		return IssuedTemplateBarcodes{}, ErrInvalidInput
	}
	out := IssuedTemplateBarcodes{TemplateID: templateID, TemplateContentHash: contentHash, KeyID: s.barcodeKeyring.ActiveKeyID, Pages: []IssuedPageBarcode{}}
	for _, page := range layout.Pages {
		value, err := s.barcodeKeyring.Sign(BarcodeClaims{TenantID: tenantID, ExamID: examID, TemplateID: templateID, TemplateContentHash: contentHash, PageNo: page.PageNo, Nonce: uuid.NewString()})
		if err != nil {
			return IssuedTemplateBarcodes{}, ErrInvalidTransition
		}
		out.Pages = append(out.Pages, IssuedPageBarcode{PageNo: page.PageNo, Value: value})
	}
	return out, nil
}

type barcodeEvaluation struct {
	Evidence       []any
	Candidates     []any
	AssignedPageNo int
	NeedsReview    bool
}

func (s *PostgresStore) evaluateBarcodesTx(ctx context.Context, tx *sql.Tx, tenantID, examID string, observations []BarcodeObservation) barcodeEvaluation {
	out := barcodeEvaluation{Evidence: []any{}, Candidates: []any{}}
	unique := map[string]BarcodeClaims{}
	for _, observation := range observations {
		sum := sha256.Sum256([]byte(observation.Text))
		evidence := map[string]any{
			"format": observation.Format, "value_sha256": "sha256:" + hex.EncodeToString(sum[:]),
			"polygon": observation.Polygon, "orientation": observation.Orientation,
		}
		if !strings.HasPrefix(observation.Text, controlledBarcodePrefix+".") {
			evidence["status"] = "ignored"
			evidence["code"] = "not_controlled"
			out.Evidence = append(out.Evidence, evidence)
			continue
		}
		claims, err := s.barcodeKeyring.Verify(observation.Text)
		if err != nil {
			evidence["status"] = "rejected"
			evidence["code"] = barcodeRejectionCode(err)
			out.NeedsReview = true
			out.Evidence = append(out.Evidence, evidence)
			continue
		}
		code := s.validateBarcodeOwnershipTx(ctx, tx, tenantID, examID, claims)
		if code != "" {
			evidence["status"] = "rejected"
			evidence["code"] = code
			out.NeedsReview = true
			out.Evidence = append(out.Evidence, evidence)
			continue
		}
		evidence["status"] = "verified"
		evidence["kid"] = claims.KeyID
		evidence["template_id"] = claims.TemplateID
		evidence["template_content_hash"] = claims.TemplateContentHash
		evidence["page_no"] = claims.PageNo
		out.Evidence = append(out.Evidence, evidence)
		unique[claims.TemplateID+":"+claims.TemplateContentHash+":"+strconv.Itoa(claims.PageNo)] = claims
	}
	keys := make([]string, 0, len(unique))
	for key := range unique {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		claims := unique[key]
		out.Candidates = append(out.Candidates, map[string]any{
			"method": "controlled_barcode", "confidence": 1.0, "verified": true,
			"template_id": claims.TemplateID, "template_content_hash": claims.TemplateContentHash,
			"page_no": claims.PageNo, "kid": claims.KeyID,
		})
		out.AssignedPageNo = claims.PageNo
	}
	if len(unique) > 1 {
		out.AssignedPageNo = 0
		out.NeedsReview = true
	}
	return out
}

func (s *PostgresStore) validateBarcodeOwnershipTx(ctx context.Context, tx *sql.Tx, tenantID, examID string, claims BarcodeClaims) string {
	if claims.TenantID != tenantID {
		return "tenant_mismatch"
	}
	if claims.ExamID != examID {
		return "exam_mismatch"
	}
	var contentHash string
	var layoutRaw []byte
	if err := tx.QueryRowContext(ctx, `SELECT content_hash,layout FROM answer_sheet_template WHERE tenant_id=$1 AND exam_id=$2::uuid AND id=$3::uuid AND status='locked' AND deleted_at IS NULL`, tenantID, examID, claims.TemplateID).Scan(&contentHash, &layoutRaw); err != nil {
		return "template_not_locked"
	}
	if contentHash != claims.TemplateContentHash {
		return "template_hash_mismatch"
	}
	var layout templateLayout
	if json.Unmarshal(layoutRaw, &layout) != nil {
		return "template_layout_invalid"
	}
	for _, page := range layout.Pages {
		if page.PageNo == claims.PageNo {
			return ""
		}
	}
	return "page_not_in_template"
}

func barcodeRejectionCode(err error) string {
	for _, code := range []string{"value_too_long", "format_invalid", "payload_invalid", "key_unknown", "signature_invalid", "claims_invalid"} {
		if strings.Contains(err.Error(), code) {
			return code
		}
	}
	return "verification_failed"
}
