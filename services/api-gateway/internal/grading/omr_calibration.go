package grading

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	"edugrade-enterprise/services/api-gateway/internal/paper"
)

// OMRCalibrationSession freezes the exact template, reference asset, worker
// profile, question, and deterministic sample population used to justify an
// automatic-confirmation permission.
type OMRCalibrationSession struct {
	ID                      string                `json:"id"`
	TenantID                string                `json:"tenant_id"`
	TemplateID              string                `json:"template_id"`
	TemplateContentHash     string                `json:"template_content_hash"`
	QuestionID              string                `json:"question_id"`
	QuestionType            string                `json:"question_type"`
	ProfileVersion          string                `json:"profile_version"`
	ProfileHash             string                `json:"profile_hash"`
	ReferenceFileAssetID    string                `json:"reference_file_asset_id"`
	ReferenceSHA256         string                `json:"reference_sha256"`
	OptionLabels            []string              `json:"option_labels"`
	SampleSeed              string                `json:"sample_seed"`
	SampleCount             int                   `json:"sample_count"`
	MinimumSamples          int                   `json:"minimum_samples"`
	MinimumSamplesPerOption int                   `json:"minimum_samples_per_option"`
	MinimumConfidence       float64               `json:"minimum_confidence"`
	Status                  string                `json:"status"`
	CreatedBy               string                `json:"created_by"`
	CreatedAt               time.Time             `json:"created_at"`
	ApprovedBy              string                `json:"approved_by,omitempty"`
	ApprovedAt              *time.Time            `json:"approved_at,omitempty"`
	ApprovalNote            string                `json:"approval_note,omitempty"`
	EvidenceHash            string                `json:"evidence_hash,omitempty"`
	RevokedBy               string                `json:"revoked_by,omitempty"`
	RevokedAt               *time.Time            `json:"revoked_at,omitempty"`
	RevokeReason            string                `json:"revoke_reason,omitempty"`
	DiscardedBy             string                `json:"discarded_by,omitempty"`
	DiscardedAt             *time.Time            `json:"discarded_at,omitempty"`
	DiscardReason           string                `json:"discard_reason,omitempty"`
	UpdatedAt               time.Time             `json:"updated_at"`
	Summary                 OMRCalibrationSummary `json:"summary"`
}

// OMRCalibrationCase is an immutable OMR output snapshot that an operator
// labels after inspecting the protected crop and overlay. A label can be set
// exactly once while the session is a draft.
type OMRCalibrationCase struct {
	ID                 string           `json:"id"`
	CalibrationID      string           `json:"calibration_id"`
	OMRRunID           string           `json:"omr_run_id"`
	AnswerSegmentID    string           `json:"answer_segment_id"`
	CropSHA256         string           `json:"crop_sha256"`
	ObservedDecision   string           `json:"observed_decision"`
	ObservedOptions    []string         `json:"observed_options"`
	ObservedConfidence float64          `json:"observed_confidence"`
	Measurements       []map[string]any `json:"measurements"`
	ExpectedOptions    []string         `json:"expected_options,omitempty"`
	Matches            *bool            `json:"matches,omitempty"`
	LabeledBy          string           `json:"labeled_by,omitempty"`
	LabeledAt          *time.Time       `json:"labeled_at,omitempty"`
	CreatedAt          time.Time        `json:"created_at"`
	OverlayFileAssetID string           `json:"overlay_file_asset_id,omitempty"`
	SegmentImageURL    string           `json:"segment_image_url,omitempty"`
}

type OMRCalibrationSummary struct {
	TotalCount     int            `json:"total_count"`
	LabeledCount   int            `json:"labeled_count"`
	PendingCount   int            `json:"pending_count"`
	MatchCount     int            `json:"match_count"`
	MismatchCount  int            `json:"mismatch_count"`
	OptionCoverage map[string]int `json:"option_coverage"`
	ReadyToApprove bool           `json:"ready_to_approve"`
	Blockers       []string       `json:"blockers"`
}

type OMRCalibrationDetail struct {
	Session OMRCalibrationSession `json:"session"`
	Cases   []OMRCalibrationCase  `json:"cases"`
}

type CreateOMRCalibrationInput struct {
	QuestionID string `json:"question_id"`
}

type LabelOMRCalibrationCaseInput struct {
	ExpectedOption string `json:"expected_option"`
}

type ApproveOMRCalibrationInput struct {
	ApprovalNote string `json:"approval_note"`
}

type RevokeOMRCalibrationInput struct {
	Reason string `json:"reason"`
}

type DiscardOMRCalibrationInput struct {
	Reason string `json:"reason"`
}

// OMRCalibrationStore is separate from the base grading Store. This keeps
// legacy in-memory grading tests independent while production PostgreSQL owns
// the approval and evidence chain.
type OMRCalibrationStore interface {
	CreateOMRCalibration(context.Context, string, string, string, CreateOMRCalibrationInput) (OMRCalibrationDetail, error)
	ListOMRCalibrations(context.Context, string, string) ([]OMRCalibrationSession, error)
	GetOMRCalibration(context.Context, string, string) (OMRCalibrationDetail, error)
	LabelOMRCalibrationCase(context.Context, string, string, string, string, LabelOMRCalibrationCaseInput) (OMRCalibrationDetail, error)
	ApproveOMRCalibration(context.Context, string, string, string, ApproveOMRCalibrationInput) (OMRCalibrationDetail, error)
	RevokeOMRCalibration(context.Context, string, string, string, RevokeOMRCalibrationInput) (OMRCalibrationDetail, error)
	DiscardOMRCalibration(context.Context, string, string, string, DiscardOMRCalibrationInput) (OMRCalibrationDetail, error)
}

type calibrationQueryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

const omrCalibrationSessionColumns = `
id::text,tenant_id::text,template_id::text,template_content_hash,question_id::text,question_type,
profile_version,profile_hash,reference_file_asset_id::text,reference_sha256,option_labels,sample_seed::text,
sample_count,minimum_samples,minimum_samples_per_option,minimum_confidence,status,created_by::text,created_at,
COALESCE(approved_by::text,''),approved_at,approval_note,COALESCE(evidence_hash,''),COALESCE(revoked_by::text,''),
revoked_at,revoke_reason,COALESCE(discarded_by::text,''),discarded_at,discard_reason,updated_at`

func (s *PostgresStore) CreateOMRCalibration(ctx context.Context, tenantID, templateID, actorID string, input CreateOMRCalibrationInput) (OMRCalibrationDetail, error) {
	templateID = strings.TrimSpace(templateID)
	input.QuestionID = strings.TrimSpace(input.QuestionID)
	if templateID == "" || input.QuestionID == "" || strings.TrimSpace(actorID) == "" {
		return OMRCalibrationDetail{}, ErrInvalidInput
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return OMRCalibrationDetail{}, err
	}
	defer tx.Rollback()

	var contentHash, templateStatus, questionType, referenceAssetID, referenceSHA256, referenceContentType string
	var layoutRaw []byte
	err = tx.QueryRowContext(ctx, `
SELECT t.content_hash,t.status,t.layout,q.question_type,fa.id::text,fa.hash_sha256,fa.content_type
FROM answer_sheet_template t
JOIN question q ON q.tenant_id=t.tenant_id AND q.exam_id=t.exam_id AND q.id=$3::uuid AND q.deleted_at IS NULL
JOIN exam_paper ep ON ep.tenant_id=t.tenant_id AND ep.id=t.exam_paper_id AND ep.deleted_at IS NULL
JOIN file_asset fa ON fa.tenant_id=ep.tenant_id AND fa.id=ep.file_asset_id AND fa.deleted_at IS NULL
WHERE t.tenant_id=$1::uuid AND t.id=$2::uuid AND t.deleted_at IS NULL
FOR UPDATE OF t
`, tenantID, templateID, input.QuestionID).Scan(&contentHash, &templateStatus, &layoutRaw, &questionType, &referenceAssetID, &referenceSHA256, &referenceContentType)
	if errors.Is(err, sql.ErrNoRows) {
		return OMRCalibrationDetail{}, ErrNotFound
	}
	if err != nil {
		return OMRCalibrationDetail{}, err
	}
	if questionType != "single_choice" && questionType != "true_false" {
		return OMRCalibrationDetail{}, ErrInvalidInput
	}
	var layout paper.TemplateLayout
	if err := json.Unmarshal(layoutRaw, &layout); err != nil {
		return OMRCalibrationDetail{}, ErrInvalidInput
	}
	reference := paper.TemplateOMRReference{Source: paper.OMRReferenceSourceExamPaper, FileAssetID: referenceAssetID, HashSHA256: referenceSHA256, ContentType: referenceContentType}
	policy := paper.OMRAutoConfirmPolicyForTemplateReference(layout, templateStatus, contentHash, contentHash, reference, input.QuestionID)
	if policy.RuntimeProfile.Mode != paper.OMRProfileModeTemplateDifference || policy.Reference == nil || policy.ProfileHash == "" {
		return OMRCalibrationDetail{}, ErrInvalidInput
	}
	optionLabels, ok := calibrationOptionLabels(layout, input.QuestionID)
	if !ok {
		return OMRCalibrationDetail{}, ErrInvalidInput
	}
	scopeKey := strings.Join([]string{tenantID, templateID, contentHash, input.QuestionID, policy.ProfileHash, policy.Reference.HashSHA256}, "|")
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, scopeKey); err != nil {
		return OMRCalibrationDetail{}, err
	}
	if existing, existingErr := getActiveOMRCalibrationSessionTx(ctx, tx, tenantID, templateID, contentHash, input.QuestionID, policy.RuntimeProfile.Version, policy.ProfileHash, policy.Reference.FileAssetID, policy.Reference.HashSHA256); existingErr == nil {
		if err = tx.Commit(); err != nil {
			return OMRCalibrationDetail{}, err
		}
		return s.GetOMRCalibration(ctx, tenantID, existing.ID)
	} else if !errors.Is(existingErr, ErrNotFound) {
		return OMRCalibrationDetail{}, existingErr
	}

	labelsRaw, err := json.Marshal(optionLabels)
	if err != nil {
		return OMRCalibrationDetail{}, ErrInvalidInput
	}
	var session OMRCalibrationSession
	err = scanOMRCalibrationSession(tx.QueryRowContext(ctx, `
INSERT INTO omr_calibration_session (
  tenant_id,template_id,template_content_hash,question_id,question_type,profile_version,profile_hash,
  reference_file_asset_id,reference_sha256,option_labels,minimum_samples,minimum_samples_per_option,
  minimum_confidence,status,created_by
)
VALUES ($1::uuid,$2::uuid,$3,$4::uuid,$5,$6,$7,$8::uuid,$9,$10::jsonb,$11,$12,$13,'draft',$14::uuid)
RETURNING `+omrCalibrationSessionColumns, tenantID, templateID, contentHash, input.QuestionID, questionType,
		policy.RuntimeProfile.Version, policy.ProfileHash, policy.Reference.FileAssetID, policy.Reference.HashSHA256, labelsRaw,
		paper.OMRCalibrationMinimumSampleCount, paper.OMRCalibrationMinimumSamplesPerOption, paper.OMRCalibrationMinimumConfidence, actorID), &session)
	if err != nil {
		return OMRCalibrationDetail{}, err
	}
	if err = s.populateOMRCalibrationCasesTx(ctx, tx, tenantID, session); err != nil {
		return OMRCalibrationDetail{}, err
	}
	if err = tx.Commit(); err != nil {
		return OMRCalibrationDetail{}, err
	}
	return s.GetOMRCalibration(ctx, tenantID, session.ID)
}

func (s *PostgresStore) ListOMRCalibrations(ctx context.Context, tenantID, templateID string) ([]OMRCalibrationSession, error) {
	templateID = strings.TrimSpace(templateID)
	if templateID == "" {
		return nil, ErrInvalidInput
	}
	rows, err := s.db.QueryContext(ctx, `SELECT `+omrCalibrationSessionColumns+`
FROM omr_calibration_session
WHERE tenant_id=$1::uuid AND template_id=$2::uuid AND deleted_at IS NULL
ORDER BY created_at DESC`, tenantID, templateID)
	if err != nil {
		return nil, err
	}
	out := []OMRCalibrationSession{}
	for rows.Next() {
		var item OMRCalibrationSession
		if err := scanOMRCalibrationSession(rows, &item); err != nil {
			_ = rows.Close()
			return nil, err
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	for index := range out {
		cases, caseErr := listOMRCalibrationCases(ctx, s.db, tenantID, out[index].ID)
		if caseErr != nil {
			return nil, caseErr
		}
		out[index].Summary = summarizeOMRCalibration(out[index], cases)
	}
	return out, nil
}

func (s *PostgresStore) GetOMRCalibration(ctx context.Context, tenantID, id string) (OMRCalibrationDetail, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return OMRCalibrationDetail{}, ErrInvalidInput
	}
	session, err := getOMRCalibrationSession(ctx, s.db, tenantID, id, false)
	if err != nil {
		return OMRCalibrationDetail{}, err
	}
	cases, err := listOMRCalibrationCases(ctx, s.db, tenantID, id)
	if err != nil {
		return OMRCalibrationDetail{}, err
	}
	session.Summary = summarizeOMRCalibration(session, cases)
	return OMRCalibrationDetail{Session: session, Cases: cases}, nil
}

func (s *PostgresStore) LabelOMRCalibrationCase(ctx context.Context, tenantID, calibrationID, caseID, actorID string, input LabelOMRCalibrationCaseInput) (OMRCalibrationDetail, error) {
	calibrationID, caseID, actorID = strings.TrimSpace(calibrationID), strings.TrimSpace(caseID), strings.TrimSpace(actorID)
	input.ExpectedOption = strings.TrimSpace(input.ExpectedOption)
	if calibrationID == "" || caseID == "" || actorID == "" || input.ExpectedOption == "" {
		return OMRCalibrationDetail{}, ErrInvalidInput
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return OMRCalibrationDetail{}, err
	}
	defer tx.Rollback()
	session, err := getOMRCalibrationSession(ctx, tx, tenantID, calibrationID, true)
	if err != nil {
		return OMRCalibrationDetail{}, err
	}
	if session.Status != "draft" {
		return OMRCalibrationDetail{}, ErrInvalidTransition
	}
	if !containsCalibrationOption(session.OptionLabels, input.ExpectedOption) {
		return OMRCalibrationDetail{}, ErrInvalidInput
	}
	item, err := getOMRCalibrationCaseTx(ctx, tx, tenantID, calibrationID, caseID)
	if err != nil {
		return OMRCalibrationDetail{}, err
	}
	if item.ExpectedOptions != nil || item.Matches != nil {
		return OMRCalibrationDetail{}, ErrInvalidTransition
	}
	matches := len(item.ObservedOptions) == 1 && item.ObservedOptions[0] == input.ExpectedOption
	result, err := tx.ExecContext(ctx, `
UPDATE omr_calibration_case
SET expected_options=jsonb_build_array($4::text),matches=$5,labeled_by=$6::uuid,labeled_at=now()
WHERE tenant_id=$1::uuid AND calibration_session_id=$2::uuid AND id=$3::uuid AND expected_options IS NULL`, tenantID, calibrationID, caseID, input.ExpectedOption, matches, actorID)
	if err != nil {
		return OMRCalibrationDetail{}, err
	}
	if changed, _ := result.RowsAffected(); changed != 1 {
		return OMRCalibrationDetail{}, ErrInvalidTransition
	}
	if err = tx.Commit(); err != nil {
		return OMRCalibrationDetail{}, err
	}
	return s.GetOMRCalibration(ctx, tenantID, calibrationID)
}

func (s *PostgresStore) ApproveOMRCalibration(ctx context.Context, tenantID, calibrationID, actorID string, input ApproveOMRCalibrationInput) (OMRCalibrationDetail, error) {
	calibrationID, actorID = strings.TrimSpace(calibrationID), strings.TrimSpace(actorID)
	input.ApprovalNote = strings.TrimSpace(input.ApprovalNote)
	if calibrationID == "" || actorID == "" || len([]rune(input.ApprovalNote)) < 10 {
		return OMRCalibrationDetail{}, ErrInvalidInput
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return OMRCalibrationDetail{}, err
	}
	defer tx.Rollback()
	session, err := getOMRCalibrationSession(ctx, tx, tenantID, calibrationID, true)
	if err != nil {
		return OMRCalibrationDetail{}, err
	}
	if session.Status != "draft" {
		return OMRCalibrationDetail{}, ErrInvalidTransition
	}
	if session.CreatedBy == actorID {
		return OMRCalibrationDetail{}, ErrForbidden
	}
	cases, err := listOMRCalibrationCases(ctx, tx, tenantID, calibrationID)
	if err != nil {
		return OMRCalibrationDetail{}, err
	}
	for _, item := range cases {
		if item.LabeledBy == actorID {
			return OMRCalibrationDetail{}, ErrForbidden
		}
	}
	summary := summarizeOMRCalibration(session, cases)
	if !summary.ReadyToApprove {
		return OMRCalibrationDetail{}, ErrInvalidTransition
	}
	evidenceHash, err := omrCalibrationEvidenceHash(session, cases)
	if err != nil {
		return OMRCalibrationDetail{}, ErrInvalidInput
	}
	result, err := tx.ExecContext(ctx, `
UPDATE omr_calibration_session
SET status='approved',approved_by=$3::uuid,approved_at=now(),approval_note=$4,evidence_hash=$5,updated_at=now()
WHERE tenant_id=$1::uuid AND id=$2::uuid AND status='draft' AND deleted_at IS NULL`, tenantID, calibrationID, actorID, input.ApprovalNote, evidenceHash)
	if err != nil {
		return OMRCalibrationDetail{}, err
	}
	if changed, _ := result.RowsAffected(); changed != 1 {
		return OMRCalibrationDetail{}, ErrInvalidTransition
	}
	if err = tx.Commit(); err != nil {
		return OMRCalibrationDetail{}, err
	}
	return s.GetOMRCalibration(ctx, tenantID, calibrationID)
}

func (s *PostgresStore) RevokeOMRCalibration(ctx context.Context, tenantID, calibrationID, actorID string, input RevokeOMRCalibrationInput) (OMRCalibrationDetail, error) {
	calibrationID, actorID = strings.TrimSpace(calibrationID), strings.TrimSpace(actorID)
	input.Reason = strings.TrimSpace(input.Reason)
	if calibrationID == "" || actorID == "" || len([]rune(input.Reason)) < 10 {
		return OMRCalibrationDetail{}, ErrInvalidInput
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return OMRCalibrationDetail{}, err
	}
	defer tx.Rollback()
	session, err := getOMRCalibrationSession(ctx, tx, tenantID, calibrationID, true)
	if err != nil {
		return OMRCalibrationDetail{}, err
	}
	if session.Status != "approved" {
		return OMRCalibrationDetail{}, ErrInvalidTransition
	}
	result, err := tx.ExecContext(ctx, `
UPDATE omr_calibration_session
SET status='revoked',revoked_by=$3::uuid,revoked_at=now(),revoke_reason=$4,updated_at=now()
WHERE tenant_id=$1::uuid AND id=$2::uuid AND status='approved' AND deleted_at IS NULL`, tenantID, calibrationID, actorID, input.Reason)
	if err != nil {
		return OMRCalibrationDetail{}, err
	}
	if changed, _ := result.RowsAffected(); changed != 1 {
		return OMRCalibrationDetail{}, ErrInvalidTransition
	}
	if err = tx.Commit(); err != nil {
		return OMRCalibrationDetail{}, err
	}
	return s.GetOMRCalibration(ctx, tenantID, calibrationID)
}

func (s *PostgresStore) DiscardOMRCalibration(ctx context.Context, tenantID, calibrationID, actorID string, input DiscardOMRCalibrationInput) (OMRCalibrationDetail, error) {
	calibrationID, actorID = strings.TrimSpace(calibrationID), strings.TrimSpace(actorID)
	input.Reason = strings.TrimSpace(input.Reason)
	if calibrationID == "" || actorID == "" || len([]rune(input.Reason)) < 10 {
		return OMRCalibrationDetail{}, ErrInvalidInput
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return OMRCalibrationDetail{}, err
	}
	defer tx.Rollback()
	session, err := getOMRCalibrationSession(ctx, tx, tenantID, calibrationID, true)
	if err != nil {
		return OMRCalibrationDetail{}, err
	}
	if session.Status != "draft" {
		return OMRCalibrationDetail{}, ErrInvalidTransition
	}
	result, err := tx.ExecContext(ctx, `
UPDATE omr_calibration_session
SET status='discarded',discarded_by=$3::uuid,discarded_at=now(),discard_reason=$4,updated_at=now()
WHERE tenant_id=$1::uuid AND id=$2::uuid AND status='draft' AND deleted_at IS NULL`, tenantID, calibrationID, actorID, input.Reason)
	if err != nil {
		return OMRCalibrationDetail{}, err
	}
	if changed, _ := result.RowsAffected(); changed != 1 {
		return OMRCalibrationDetail{}, ErrInvalidTransition
	}
	if err = tx.Commit(); err != nil {
		return OMRCalibrationDetail{}, err
	}
	return s.GetOMRCalibration(ctx, tenantID, calibrationID)
}

func (s *PostgresStore) populateOMRCalibrationCasesTx(ctx context.Context, tx *sql.Tx, tenantID string, session OMRCalibrationSession) error {
	rows, err := tx.QueryContext(ctx, `
WITH latest_by_segment AS (
  SELECT DISTINCT ON (o.answer_segment_id)
    o.id::text,o.answer_segment_id::text,o.crop_sha256,o.decision,o.selected_options,o.confidence,o.measurements
  FROM omr_run o
  JOIN answer_segment seg ON seg.tenant_id=o.tenant_id AND seg.id=o.answer_segment_id AND seg.deleted_at IS NULL
  WHERE o.tenant_id=$1::uuid AND o.template_id=$2::uuid AND o.template_content_hash=$3
    AND seg.question_id=$4::uuid AND o.profile_version=$5 AND o.profile_hash=$6
    AND o.reference_file_asset_id=$7::uuid AND o.reference_sha256=$8
    AND o.status='completed' AND o.deleted_at IS NULL
  ORDER BY o.answer_segment_id,o.completed_at DESC NULLS LAST,o.id DESC
)
SELECT id,answer_segment_id,crop_sha256,decision,selected_options,confidence,measurements
FROM latest_by_segment
WHERE decision='selected' AND confidence >= $9 AND jsonb_array_length(selected_options)=1
ORDER BY md5(id || $10)
LIMIT $11`, tenantID, session.TemplateID, session.TemplateContentHash, session.QuestionID,
		session.ProfileVersion, session.ProfileHash, session.ReferenceFileAssetID, session.ReferenceSHA256,
		session.MinimumConfidence, session.SampleSeed, session.MinimumSamples)
	if err != nil {
		return err
	}
	type candidate struct {
		runID, segmentID, cropSHA256, decision string
		selected, measurements                 []byte
		confidence                             float64
	}
	candidates := []candidate{}
	for rows.Next() {
		var item candidate
		if err := rows.Scan(&item.runID, &item.segmentID, &item.cropSHA256, &item.decision, &item.selected, &item.confidence, &item.measurements); err != nil {
			_ = rows.Close()
			return err
		}
		candidates = append(candidates, item)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for _, item := range candidates {
		if _, err := tx.ExecContext(ctx, `
INSERT INTO omr_calibration_case (
  tenant_id,calibration_session_id,omr_run_id,answer_segment_id,crop_sha256,
  observed_decision,observed_options,observed_confidence,measurements
)
VALUES ($1::uuid,$2::uuid,$3::uuid,$4::uuid,$5,$6,$7::jsonb,$8,$9::jsonb)`, tenantID, session.ID, item.runID, item.segmentID, item.cropSHA256, item.decision, item.selected, item.confidence, item.measurements); err != nil {
			return err
		}
	}
	_, err = tx.ExecContext(ctx, `UPDATE omr_calibration_session SET sample_count=$3,updated_at=now() WHERE tenant_id=$1::uuid AND id=$2::uuid`, tenantID, session.ID, len(candidates))
	return err
}

func getActiveOMRCalibrationSessionTx(ctx context.Context, tx *sql.Tx, tenantID, templateID, templateHash, questionID, profileVersion, profileHash, referenceAssetID, referenceSHA256 string) (OMRCalibrationSession, error) {
	row := tx.QueryRowContext(ctx, `SELECT `+omrCalibrationSessionColumns+`
FROM omr_calibration_session
WHERE tenant_id=$1::uuid AND template_id=$2::uuid AND template_content_hash=$3 AND question_id=$4::uuid
  AND profile_version=$5 AND profile_hash=$6 AND reference_file_asset_id=$7::uuid AND reference_sha256=$8
  AND status IN ('draft','approved') AND deleted_at IS NULL
ORDER BY CASE status WHEN 'approved' THEN 0 ELSE 1 END,created_at DESC
LIMIT 1
FOR UPDATE`, tenantID, templateID, templateHash, questionID, profileVersion, profileHash, referenceAssetID, referenceSHA256)
	var out OMRCalibrationSession
	return out, scanOMRCalibrationSession(row, &out)
}

func getOMRCalibrationSession(ctx context.Context, queryer calibrationQueryer, tenantID, id string, forUpdate bool) (OMRCalibrationSession, error) {
	statement := `SELECT ` + omrCalibrationSessionColumns + `
FROM omr_calibration_session
WHERE tenant_id=$1::uuid AND id=$2::uuid AND deleted_at IS NULL`
	if forUpdate {
		statement += ` FOR UPDATE`
	}
	var out OMRCalibrationSession
	return out, scanOMRCalibrationSession(queryer.QueryRowContext(ctx, statement, tenantID, id), &out)
}

func getOMRCalibrationCaseTx(ctx context.Context, tx *sql.Tx, tenantID, calibrationID, caseID string) (OMRCalibrationCase, error) {
	rows, err := listOMRCalibrationCases(ctx, tx, tenantID, calibrationID)
	if err != nil {
		return OMRCalibrationCase{}, err
	}
	for _, item := range rows {
		if item.ID == caseID {
			return item, nil
		}
	}
	return OMRCalibrationCase{}, ErrNotFound
}

func listOMRCalibrationCases(ctx context.Context, queryer calibrationQueryer, tenantID, calibrationID string) ([]OMRCalibrationCase, error) {
	rows, err := queryer.QueryContext(ctx, `
SELECT c.id::text,c.calibration_session_id::text,c.omr_run_id::text,c.answer_segment_id::text,c.crop_sha256,
  c.observed_decision,c.observed_options,c.observed_confidence,c.measurements,
  COALESCE(c.expected_options,'null'::jsonb),c.matches,COALESCE(c.labeled_by::text,''),c.labeled_at,c.created_at,
  COALESCE(o.overlay_file_asset_id::text,'')
FROM omr_calibration_case c
LEFT JOIN omr_run o ON o.tenant_id=c.tenant_id AND o.id=c.omr_run_id
WHERE c.tenant_id=$1::uuid AND c.calibration_session_id=$2::uuid
ORDER BY c.created_at,c.id`, tenantID, calibrationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []OMRCalibrationCase{}
	for rows.Next() {
		var item OMRCalibrationCase
		var observedRaw, measurementsRaw, expectedRaw []byte
		var matches sql.NullBool
		var labeledAt sql.NullTime
		if err := rows.Scan(&item.ID, &item.CalibrationID, &item.OMRRunID, &item.AnswerSegmentID, &item.CropSHA256,
			&item.ObservedDecision, &observedRaw, &item.ObservedConfidence, &measurementsRaw,
			&expectedRaw, &matches, &item.LabeledBy, &labeledAt, &item.CreatedAt, &item.OverlayFileAssetID); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(observedRaw, &item.ObservedOptions); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(measurementsRaw, &item.Measurements); err != nil {
			return nil, err
		}
		if string(expectedRaw) != "null" {
			if err := json.Unmarshal(expectedRaw, &item.ExpectedOptions); err != nil {
				return nil, err
			}
		}
		if matches.Valid {
			value := matches.Bool
			item.Matches = &value
		}
		if labeledAt.Valid {
			value := labeledAt.Time.UTC()
			item.LabeledAt = &value
		}
		item.CreatedAt = item.CreatedAt.UTC()
		out = append(out, item)
	}
	return out, rows.Err()
}

func scanOMRCalibrationSession(row calibrationScanner, out *OMRCalibrationSession) error {
	var optionLabelsRaw []byte
	var approvedAt, revokedAt, discardedAt sql.NullTime
	if err := row.Scan(&out.ID, &out.TenantID, &out.TemplateID, &out.TemplateContentHash, &out.QuestionID, &out.QuestionType,
		&out.ProfileVersion, &out.ProfileHash, &out.ReferenceFileAssetID, &out.ReferenceSHA256, &optionLabelsRaw, &out.SampleSeed,
		&out.SampleCount, &out.MinimumSamples, &out.MinimumSamplesPerOption, &out.MinimumConfidence, &out.Status, &out.CreatedBy, &out.CreatedAt,
		&out.ApprovedBy, &approvedAt, &out.ApprovalNote, &out.EvidenceHash, &out.RevokedBy, &revokedAt, &out.RevokeReason, &out.DiscardedBy, &discardedAt, &out.DiscardReason, &out.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if err := json.Unmarshal(optionLabelsRaw, &out.OptionLabels); err != nil {
		return err
	}
	if approvedAt.Valid {
		value := approvedAt.Time.UTC()
		out.ApprovedAt = &value
	}
	if revokedAt.Valid {
		value := revokedAt.Time.UTC()
		out.RevokedAt = &value
	}
	if discardedAt.Valid {
		value := discardedAt.Time.UTC()
		out.DiscardedAt = &value
	}
	out.CreatedAt = out.CreatedAt.UTC()
	out.UpdatedAt = out.UpdatedAt.UTC()
	return nil
}

type calibrationScanner interface{ Scan(...any) error }

func summarizeOMRCalibration(session OMRCalibrationSession, cases []OMRCalibrationCase) OMRCalibrationSummary {
	summary := OMRCalibrationSummary{OptionCoverage: map[string]int{}, Blockers: []string{}}
	for _, label := range session.OptionLabels {
		summary.OptionCoverage[label] = 0
	}
	for _, item := range cases {
		summary.TotalCount++
		if item.ExpectedOptions == nil || item.Matches == nil {
			summary.PendingCount++
			continue
		}
		summary.LabeledCount++
		if len(item.ExpectedOptions) == 1 {
			summary.OptionCoverage[item.ExpectedOptions[0]]++
		}
		if *item.Matches {
			summary.MatchCount++
		} else {
			summary.MismatchCount++
		}
	}
	if summary.TotalCount < session.MinimumSamples {
		summary.Blockers = append(summary.Blockers, "sample_count_below_minimum")
	}
	if summary.PendingCount > 0 {
		summary.Blockers = append(summary.Blockers, "calibration_labels_pending")
	}
	if summary.MismatchCount > 0 {
		summary.Blockers = append(summary.Blockers, "calibration_mismatch_detected")
	}
	for _, label := range session.OptionLabels {
		if summary.OptionCoverage[label] < session.MinimumSamplesPerOption {
			summary.Blockers = append(summary.Blockers, "option_coverage_incomplete")
			break
		}
	}
	if session.Status != "draft" {
		summary.Blockers = append(summary.Blockers, "calibration_not_draft")
	}
	summary.ReadyToApprove = len(summary.Blockers) == 0
	return summary
}

func calibrationOptionLabels(layout paper.TemplateLayout, questionID string) ([]string, bool) {
	labels := []string{}
	seen := map[string]bool{}
	for _, page := range layout.Pages {
		for _, region := range page.QuestionRegions {
			if region.QuestionID != questionID {
				continue
			}
			for _, option := range region.OptionRegions {
				label := strings.TrimSpace(option.Label)
				if label == "" || seen[label] {
					return nil, false
				}
				seen[label] = true
				labels = append(labels, label)
			}
		}
	}
	return labels, len(labels) >= 2
}

func containsCalibrationOption(options []string, expected string) bool {
	for _, option := range options {
		if option == expected {
			return true
		}
	}
	return false
}

func omrCalibrationEvidenceHash(session OMRCalibrationSession, cases []OMRCalibrationCase) (string, error) {
	sorted := append([]OMRCalibrationCase(nil), cases...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].ID < sorted[j].ID })
	payload := struct {
		Session struct {
			TemplateID              string   `json:"template_id"`
			TemplateContentHash     string   `json:"template_content_hash"`
			QuestionID              string   `json:"question_id"`
			QuestionType            string   `json:"question_type"`
			ProfileVersion          string   `json:"profile_version"`
			ProfileHash             string   `json:"profile_hash"`
			ReferenceFileAssetID    string   `json:"reference_file_asset_id"`
			ReferenceSHA256         string   `json:"reference_sha256"`
			OptionLabels            []string `json:"option_labels"`
			SampleSeed              string   `json:"sample_seed"`
			MinimumSamples          int      `json:"minimum_samples"`
			MinimumSamplesPerOption int      `json:"minimum_samples_per_option"`
			MinimumConfidence       float64  `json:"minimum_confidence"`
		} `json:"session"`
		Cases []OMRCalibrationCase `json:"cases"`
	}{}
	payload.Session.TemplateID = session.TemplateID
	payload.Session.TemplateContentHash = session.TemplateContentHash
	payload.Session.QuestionID = session.QuestionID
	payload.Session.QuestionType = session.QuestionType
	payload.Session.ProfileVersion = session.ProfileVersion
	payload.Session.ProfileHash = session.ProfileHash
	payload.Session.ReferenceFileAssetID = session.ReferenceFileAssetID
	payload.Session.ReferenceSHA256 = session.ReferenceSHA256
	payload.Session.OptionLabels = session.OptionLabels
	payload.Session.SampleSeed = session.SampleSeed
	payload.Session.MinimumSamples = session.MinimumSamples
	payload.Session.MinimumSamplesPerOption = session.MinimumSamplesPerOption
	payload.Session.MinimumConfidence = session.MinimumConfidence
	payload.Cases = sorted
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

// loadApprovedOMRCalibrationTx is deliberately called while creating the OMR
// run, before the worker receives any task. It never trusts a session id from a
// client or worker payload.
func (s *PostgresStore) loadApprovedOMRCalibrationTx(ctx context.Context, tx *sql.Tx, tenantID, templateID, templateContentHash, questionID, profileVersion, profileHash, referenceFileAssetID, referenceSHA256 string) (*paper.OMRCalibrationApproval, error) {
	var out paper.OMRCalibrationApproval
	err := tx.QueryRowContext(ctx, `
SELECT id::text,template_id::text,template_content_hash,question_id::text,profile_version,profile_hash,
  reference_file_asset_id::text,reference_sha256,minimum_confidence,evidence_hash,status
FROM omr_calibration_session
WHERE tenant_id=$1::uuid AND template_id=$2::uuid AND template_content_hash=$3 AND question_id=$4::uuid
  AND profile_version=$5 AND profile_hash=$6 AND reference_file_asset_id=$7::uuid AND reference_sha256=$8
  AND status='approved' AND deleted_at IS NULL
ORDER BY approved_at DESC
LIMIT 1`, tenantID, templateID, templateContentHash, questionID, profileVersion, profileHash, referenceFileAssetID, referenceSHA256).Scan(
		&out.ID, &out.TemplateID, &out.TemplateContentHash, &out.QuestionID, &out.ProfileVersion, &out.ProfileHash,
		&out.ReferenceFileAssetID, &out.ReferenceSHA256, &out.MinimumConfidence, &out.EvidenceHash, &out.Status)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &out, nil
}
