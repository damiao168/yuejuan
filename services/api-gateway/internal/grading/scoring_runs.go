package grading

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"edugrade-enterprise/services/api-gateway/internal/paper"
	"edugrade-enterprise/services/api-gateway/internal/workerruntime"
)

type ScoringRun struct {
	ID                  string     `json:"id"`
	TenantID            string     `json:"tenant_id"`
	ExamID              string     `json:"exam_id"`
	IdempotencyKey      string     `json:"idempotency_key"`
	Status              string     `json:"status"`
	TotalCount          int        `json:"total_count"`
	QueuedCount         int        `json:"queued_count"`
	AutoConfirmedCount  int        `json:"auto_confirmed_count"`
	HumanConfirmedCount int        `json:"human_confirmed_count"`
	ReviewCount         int        `json:"review_count"`
	FailedCount         int        `json:"failed_count"`
	StartedBy           string     `json:"started_by"`
	StartedAt           *time.Time `json:"started_at,omitempty"`
	CompletedAt         *time.Time `json:"completed_at,omitempty"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

type StartScoringRunInput struct {
	IdempotencyKey string `json:"idempotency_key"`
}

type ScoringQuestionSummary struct {
	QuestionID   string `json:"question_id"`
	QuestionNo   string `json:"question_no"`
	QuestionType string `json:"question_type"`
	Total        int    `json:"total"`
	Queued       int    `json:"queued"`
	Confirmed    int    `json:"confirmed"`
	Review       int    `json:"review"`
	Failed       int    `json:"failed"`
}

type ScoringSummary struct {
	Run       *ScoringRun              `json:"run,omitempty"`
	Questions []ScoringQuestionSummary `json:"questions"`
}

// ScoringRunItem is the operator-facing state of one answer segment in a run.
// It intentionally contains no student identity; the grading workspace remains
// the only place that resolves an anonymous task to protected answer assets.
type ScoringRunItem struct {
	AnswerSegmentID       string   `json:"answer_segment_id"`
	QuestionID            string   `json:"question_id"`
	QuestionNo            string   `json:"question_no"`
	QuestionType          string   `json:"question_type"`
	AnonymousCode         string   `json:"anonymous_code"`
	State                 string   `json:"state"`
	RecognitionSource     string   `json:"recognition_source,omitempty"`
	RecognizedAnswer      string   `json:"recognized_answer,omitempty"`
	RecognitionDecision   string   `json:"recognition_decision,omitempty"`
	RecognitionConfidence *float64 `json:"recognition_confidence,omitempty"`
	StandardAnswer        any      `json:"standard_answer,omitempty"`
	RuleType              string   `json:"rule_type,omitempty"`
	Score                 *float64 `json:"score,omitempty"`
	MaxScore              *float64 `json:"max_score,omitempty"`
	GradeSource           string   `json:"grade_source,omitempty"`
	OMRRunID              string   `json:"omr_run_id,omitempty"`
	RuntimeTaskID         string   `json:"runtime_task_id,omitempty"`
	RuntimeStatus         string   `json:"runtime_status,omitempty"`
	ReviewTaskID          string   `json:"review_task_id,omitempty"`
	ReviewStatus          string   `json:"review_status,omitempty"`
	ReasonCode            string   `json:"reason_code,omitempty"`
	ErrorCode             string   `json:"error_code,omitempty"`
}

type ScoringRunDetail struct {
	Run   ScoringRun       `json:"run"`
	Items []ScoringRunItem `json:"items"`
}

type ExamAutomationResults struct {
	Items []ScoringRunItem `json:"items"`
}

type FailedOMRTask struct {
	OMRRunID      string `json:"omr_run_id"`
	RuntimeTaskID string `json:"runtime_task_id"`
}

type OMRRun struct {
	ID                       string           `json:"id"`
	TenantID                 string           `json:"tenant_id"`
	ScoringRunID             string           `json:"scoring_run_id"`
	AnswerSegmentID          string           `json:"answer_segment_id"`
	ExamID                   string           `json:"exam_id"`
	RuntimeTaskID            string           `json:"runtime_task_id"`
	Status                   string           `json:"status"`
	Decision                 string           `json:"decision,omitempty"`
	Confidence               *float64         `json:"confidence,omitempty"`
	Measurements             []map[string]any `json:"measurements"`
	SelectedOptions          []string         `json:"selected_options"`
	OverlayFileAssetID       string           `json:"overlay_file_asset_id,omitempty"`
	CropSHA256               string           `json:"crop_sha256"`
	ReferenceFileAssetID     string           `json:"reference_file_asset_id,omitempty"`
	ReferenceSHA256          string           `json:"reference_sha256,omitempty"`
	CalibrationSessionID     string           `json:"calibration_session_id,omitempty"`
	CalibrationEvidenceHash  string           `json:"calibration_evidence_hash,omitempty"`
	ProfileVersion           string           `json:"profile_version"`
	ProfileHash              string           `json:"profile_hash"`
	AutoConfirmMinConfidence float64          `json:"auto_confirm_min_confidence"`
	AutoConfirmEligible      bool             `json:"auto_confirm_eligible"`
	AutoConfirmReason        string           `json:"auto_confirm_reason"`
}

type OMRResultInput struct {
	TaskID             string           `json:"task_id"`
	LeaseToken         string           `json:"lease_token"`
	ResultVersion      string           `json:"result_version"`
	DurationMS         int              `json:"duration_ms"`
	Decision           string           `json:"decision"`
	Selected           []string         `json:"selected"`
	Confidence         float64          `json:"confidence"`
	NeedsHumanReview   bool             `json:"needs_human_review"`
	Measurements       []map[string]any `json:"measurements"`
	ProfileVersion     string           `json:"profile_version"`
	ProfileHash        string           `json:"profile_hash"`
	ReferenceSHA256    string           `json:"reference_sha256,omitempty"`
	Thresholds         map[string]any   `json:"thresholds"`
	OverlayFileAssetID string           `json:"overlay_file_asset_id"`
	OverlaySHA256      string           `json:"overlay_sha256"`
}

type OMRFailureInput struct {
	TaskID      string         `json:"task_id"`
	LeaseToken  string         `json:"lease_token"`
	Retryable   bool           `json:"retryable"`
	ErrorCode   string         `json:"error_code"`
	ErrorDetail map[string]any `json:"error_detail"`
	DurationMS  int            `json:"duration_ms"`
}

type ScoringRunStore interface {
	StartScoringRun(context.Context, string, string, string, StartScoringRunInput) (ScoringRun, error)
	GetScoringSummary(context.Context, string, string) (ScoringSummary, error)
	GetOMRRun(context.Context, string, string) (OMRRun, error)
	ApplyOMRResult(context.Context, string, string, string, OMRResultInput, *Engine) (OMRRun, *QuestionGrade, error)
	ApplyOMRFailure(context.Context, string, string, string, map[string]any, bool) (OMRRun, error)
	ConfirmRuleGrade(context.Context, string, string, string, Grade) (QuestionGrade, error)
	ProcessRuleCandidates(context.Context, string, string, string, *Engine) error
}

// ScoringRecoveryStore holds the operations that coordinate scoring facts with
// Worker Runtime recovery. It is separate from normal grading operations so a
// caller cannot accidentally invoke recovery behavior through a basic store.
type ScoringRecoveryStore interface {
	GetScoringRunDetail(context.Context, string, string) (ScoringRunDetail, error)
	GetExamAutomationResults(context.Context, string, string) (ExamAutomationResults, error)
	BeginScoringRunCancellation(context.Context, string, string) (ScoringRun, []string, error)
	FinalizeScoringRunCancellation(context.Context, string, string) (ScoringRun, error)
	ListFailedOMRTasks(context.Context, string, string) ([]FailedOMRTask, error)
	PrepareOMRRetry(context.Context, string, string) (FailedOMRTask, error)
	RestoreOMRRetry(context.Context, string, string) error
	RefreshScoringRun(context.Context, string, string) (ScoringRun, error)
	ReprocessSegmentScore(context.Context, string, string, string, StartScoringRunInput) (ScoringRun, error)
}

type QuestionGrade struct {
	ID              string         `json:"id"`
	AnswerSegmentID string         `json:"answer_segment_id"`
	QuestionID      string         `json:"question_id"`
	Score           float64        `json:"score"`
	MaxScore        float64        `json:"max_score"`
	Source          string         `json:"source"`
	Version         int            `json:"version"`
	Evidence        map[string]any `json:"evidence"`
	CreatedAt       time.Time      `json:"created_at"`
}

func (s *PostgresStore) StartScoringRun(ctx context.Context, tenantID, examID, actorID string, input StartScoringRunInput) (ScoringRun, error) {
	return s.startScoringRun(ctx, tenantID, examID, actorID, input, "")
}

func (s *PostgresStore) ReprocessSegmentScore(ctx context.Context, tenantID, segmentID, actorID string, input StartScoringRunInput) (ScoringRun, error) {
	segmentID = strings.TrimSpace(segmentID)
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	if segmentID == "" || input.IdempotencyKey == "" || len(input.IdempotencyKey) > 100 {
		return ScoringRun{}, ErrInvalidInput
	}
	var examID string
	err := s.db.QueryRowContext(ctx, `SELECT sub.exam_id::text FROM answer_segment seg JOIN submission sub ON sub.tenant_id=seg.tenant_id AND sub.id=seg.submission_id AND sub.deleted_at IS NULL WHERE seg.tenant_id=$1::uuid AND seg.id=$2::uuid AND seg.deleted_at IS NULL`, tenantID, segmentID).Scan(&examID)
	if errors.Is(err, sql.ErrNoRows) {
		return ScoringRun{}, ErrNotFound
	}
	if err != nil {
		return ScoringRun{}, err
	}
	input.IdempotencyKey = "segment-reprocess:" + segmentID + ":" + input.IdempotencyKey
	return s.startScoringRun(ctx, tenantID, examID, actorID, input, segmentID)
}

func (s *PostgresStore) startScoringRun(ctx context.Context, tenantID, examID, actorID string, input StartScoringRunInput, segmentID string) (ScoringRun, error) {
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	if input.IdempotencyKey == "" || len(input.IdempotencyKey) > 160 {
		return ScoringRun{}, ErrInvalidInput
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ScoringRun{}, err
	}
	defer tx.Rollback()
	// Serialise starts for one exam. A second unresolved run would otherwise
	// queue duplicate OMR work and compete to replace the same current grades.
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, examID+":scoring-run"); err != nil {
		return ScoringRun{}, err
	}
	var existing string
	err = tx.QueryRowContext(ctx, `SELECT id::text FROM scoring_run WHERE tenant_id=$1::uuid AND exam_id=$2::uuid AND idempotency_key=$3 AND deleted_at IS NULL`, tenantID, examID, input.IdempotencyKey).Scan(&existing)
	if err == nil {
		run, getErr := scanScoringRun(tx.QueryRowContext(ctx, scoringRunSelect+` WHERE tenant_id=$1::uuid AND id=$2::uuid`, tenantID, existing))
		if getErr != nil {
			return ScoringRun{}, getErr
		}
		return run, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return ScoringRun{}, err
	}
	var unresolvedRunID string
	err = tx.QueryRowContext(ctx, `
SELECT id::text
FROM scoring_run
WHERE tenant_id=$1::uuid AND exam_id=$2::uuid AND deleted_at IS NULL
  AND status IN ('queued','processing','needs_review','failed','cancelling')
ORDER BY created_at DESC
LIMIT 1
`, tenantID, examID).Scan(&unresolvedRunID)
	if err == nil {
		return ScoringRun{}, ErrInvalidTransition
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return ScoringRun{}, err
	}
	if segmentID != "" {
		var exists int
		err = tx.QueryRowContext(ctx, `
SELECT 1
FROM answer_segment seg
JOIN submission sub ON sub.tenant_id=seg.tenant_id AND sub.id=seg.submission_id AND sub.deleted_at IS NULL
WHERE seg.tenant_id=$1::uuid AND sub.exam_id=$2::uuid AND seg.id=$3::uuid
  AND seg.deleted_at IS NULL AND seg.processing_status='completed' AND seg.crop_file_asset_id IS NOT NULL
`, tenantID, examID, segmentID).Scan(&exists)
		if errors.Is(err, sql.ErrNoRows) {
			return ScoringRun{}, ErrNotFound
		}
		if err != nil {
			return ScoringRun{}, err
		}
		var activeReview, activeOMR int
		if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM review_task WHERE tenant_id=$1::uuid AND answer_segment_id=$2::uuid AND status IN ('pending','assigned','in_progress','returned') AND deleted_at IS NULL`, tenantID, segmentID).Scan(&activeReview); err != nil {
			return ScoringRun{}, err
		}
		if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM omr_run WHERE tenant_id=$1::uuid AND answer_segment_id=$2::uuid AND status IN ('queued','processing','retryable_error') AND deleted_at IS NULL`, tenantID, segmentID).Scan(&activeOMR); err != nil {
			return ScoringRun{}, err
		}
		if activeReview > 0 || activeOMR > 0 {
			return ScoringRun{}, ErrInvalidTransition
		}
	}
	var runID string
	err = tx.QueryRowContext(ctx, `
INSERT INTO scoring_run (tenant_id, exam_id, idempotency_key, status, started_by, started_at)
SELECT $1::uuid, e.id, $3, 'processing', $4::uuid, now()
FROM exam e WHERE e.tenant_id=$1::uuid AND e.id=$2::uuid AND e.deleted_at IS NULL AND e.status IN ('collecting','processing','grading')
RETURNING id::text`, tenantID, examID, input.IdempotencyKey, actorID).Scan(&runID)
	if errors.Is(err, sql.ErrNoRows) {
		return ScoringRun{}, ErrInvalidInput
	}
	if err != nil {
		return ScoringRun{}, err
	}
	if segmentID != "" {
		if _, err = tx.ExecContext(ctx, `UPDATE answer_candidate SET is_current=false WHERE tenant_id=$1::uuid AND answer_segment_id=$2::uuid AND is_current`, tenantID, segmentID); err != nil {
			return ScoringRun{}, err
		}
		if _, err = tx.ExecContext(ctx, `UPDATE question_grade SET is_current=false,status='invalidated' WHERE tenant_id=$1::uuid AND answer_segment_id=$2::uuid AND is_current AND deleted_at IS NULL`, tenantID, segmentID); err != nil {
			return ScoringRun{}, err
		}
	}

	rows, err := tx.QueryContext(ctx, `
 SELECT seg.id::text, seg.submission_id::text, q.id::text, q.question_no, q.question_type,
   seg.crop_file_asset_id::text, seg.crop_sha256, seg.template_id::text, seg.template_content_hash,
   q.answer_area, COALESCE(NULLIF(sub.candidate_no,''), seg.id::text), fa.content_type,
   COALESCE(sr.id::text,''), COALESCE(ans.id::text,''), COALESCE(ans.answer_text,''),
   COALESCE(ans.answer_payload,'{}'::jsonb), COALESCE(ans.source,''), ans.confidence::float8,
   COALESCE(ast.status,''), COALESCE(ast.content_hash,''), COALESCE(ast.layout,'{}'::jsonb),
   COALESCE(reference_file.id::text,''), COALESCE(reference_file.hash_sha256,''), COALESCE(reference_file.content_type,'')
 FROM answer_segment seg
 JOIN submission sub ON sub.tenant_id=seg.tenant_id AND sub.id=seg.submission_id AND sub.deleted_at IS NULL
 JOIN question q ON q.tenant_id=seg.tenant_id AND q.id=seg.question_id AND q.deleted_at IS NULL
 JOIN file_asset fa ON fa.tenant_id=seg.tenant_id AND fa.id=seg.crop_file_asset_id AND fa.deleted_at IS NULL
	LEFT JOIN answer_sheet_template ast ON ast.tenant_id=seg.tenant_id AND ast.id=seg.template_id AND ast.deleted_at IS NULL
 LEFT JOIN exam_paper reference_paper ON reference_paper.tenant_id=ast.tenant_id AND reference_paper.id=ast.exam_paper_id AND reference_paper.deleted_at IS NULL
 LEFT JOIN file_asset reference_file ON reference_file.tenant_id=reference_paper.tenant_id AND reference_file.id=reference_paper.file_asset_id AND reference_file.deleted_at IS NULL
 LEFT JOIN scoring_rule sr ON sr.tenant_id=q.tenant_id AND sr.question_id=q.id AND sr.status='published' AND sr.deleted_at IS NULL
LEFT JOIN LATERAL (SELECT id,answer_text,answer_payload,source,confidence FROM answer_segment_answer WHERE tenant_id=seg.tenant_id AND answer_segment_id=seg.id AND deleted_at IS NULL ORDER BY created_at DESC LIMIT 1) ans ON true
WHERE seg.tenant_id=$1::uuid AND sub.exam_id=$2::uuid AND seg.deleted_at IS NULL
  AND seg.processing_status='completed' AND seg.crop_file_asset_id IS NOT NULL
  AND (NULLIF($3,'')::uuid IS NULL OR seg.id=NULLIF($3,'')::uuid)
ORDER BY q.sort_order, seg.created_at`, tenantID, examID, segmentID)
	if err != nil {
		return ScoringRun{}, err
	}
	type segmentRow struct {
		id, submissionID, questionID, no, kind, cropID, cropHash, templateID, templateHash, anonymous, contentType, ruleID string
		answerID, answerText, answerSource, templateStatus, currentTemplateHash                                            string
		referenceAssetID, referenceHash, referenceContentType                                                              string
		area, answerPayload, templateLayout                                                                                []byte
		answerConfidence                                                                                                   sql.NullFloat64
	}
	segments := []segmentRow{}
	for rows.Next() {
		var item segmentRow
		if err := rows.Scan(&item.id, &item.submissionID, &item.questionID, &item.no, &item.kind, &item.cropID, &item.cropHash, &item.templateID, &item.templateHash, &item.area, &item.anonymous, &item.contentType, &item.ruleID, &item.answerID, &item.answerText, &item.answerPayload, &item.answerSource, &item.answerConfidence, &item.templateStatus, &item.currentTemplateHash, &item.templateLayout, &item.referenceAssetID, &item.referenceHash, &item.referenceContentType); err != nil {
			rows.Close()
			return ScoringRun{}, err
		}
		segments = append(segments, item)
	}
	if err = rows.Close(); err != nil {
		return ScoringRun{}, err
	}
	insertReviewTask := func(segment segmentRow, source, reason string) (bool, error) {
		result, insertErr := tx.ExecContext(ctx, `INSERT INTO review_task (tenant_id,exam_id,question_id,question_no,answer_segment_id,submission_id,anonymous_code,source,status,priority,grade_round,reason_code,scoring_run_id,created_by)
VALUES ($1::uuid,$2::uuid,$3::uuid,$4,$5::uuid,$6::uuid,$7,$8,'pending',50,'single',$9,$10::uuid,$11::uuid)
ON CONFLICT (tenant_id,answer_segment_id,source) WHERE status IN ('pending','assigned','in_progress','returned') AND deleted_at IS NULL DO NOTHING`, tenantID, examID, segment.questionID, segment.no, segment.id, segment.submissionID, segment.anonymous, source, reason, runID, actorID)
		if insertErr != nil {
			return false, insertErr
		}
		count, _ := result.RowsAffected()
		return count == 1, nil
	}
	queued, review := 0, 0
	for _, segment := range segments {
		var area map[string]any
		_ = json.Unmarshal(segment.area, &area)
		options, _ := area["option_regions"].([]any)
		isOMR := segment.kind == "single_choice" || segment.kind == "true_false" || segment.kind == "multiple_choice"
		isTextRule := segment.kind == "fill_blank" || segment.kind == "numeric"
		if isTextRule && segment.ruleID != "" && segment.answerID != "" {
			confidence := 0.0
			if segment.answerConfidence.Valid {
				confidence = segment.answerConfidence.Float64
			}
			confirmed := segment.answerSource == "manual_entry" || segment.answerSource == "imported_answer" || confidence >= 0.9
			decision := "ambiguous"
			if confirmed {
				decision = "confirmed"
			}
			source := "ocr"
			if segment.answerSource == "manual_entry" {
				source = "manual"
			}
			if segment.answerSource == "imported_answer" {
				source = "imported"
			}
			if _, err = tx.ExecContext(ctx, `UPDATE answer_candidate SET is_current=false WHERE tenant_id=$1::uuid AND answer_segment_id=$2::uuid AND is_current`, tenantID, segment.id); err != nil {
				return ScoringRun{}, err
			}
			_, err = tx.ExecContext(ctx, `INSERT INTO answer_candidate(tenant_id,answer_segment_id,scoring_run_id,source,payload,display_text,confidence,decision,evidence,engine_version,profile_version,input_hash,is_current,created_by) VALUES($1::uuid,$2::uuid,$3::uuid,$4,$5::jsonb,$6,NULLIF($7,0),$8,jsonb_build_object('answer_segment_answer_id',$9),'rule-input-v1','rule-input-v1',$10,true,$11::uuid)`, tenantID, segment.id, runID, source, segment.answerPayload, segment.answerText, confidence, decision, segment.answerID, "sha256:"+strings.ReplaceAll(segment.answerID, "-", ""), actorID)
			if err != nil {
				return ScoringRun{}, err
			}
			if !confirmed {
				created, insertErr := insertReviewTask(segment, "rule_review_required", "answer_low_confidence")
				if insertErr != nil {
					return ScoringRun{}, insertErr
				}
				if created {
					review++
				}
			}
		} else if isOMR && segment.ruleID != "" && len(options) >= 2 {
			var templateLayout paper.TemplateLayout
			_ = json.Unmarshal(segment.templateLayout, &templateLayout)
			currentReference := paper.TemplateOMRReference{
				Source:      paper.OMRReferenceSourceExamPaper,
				FileAssetID: segment.referenceAssetID,
				HashSHA256:  segment.referenceHash,
				ContentType: segment.referenceContentType,
			}
			basePolicy := paper.OMRAutoConfirmPolicyForTemplateReference(templateLayout, segment.templateStatus, segment.currentTemplateHash, segment.templateHash, currentReference, segment.questionID)
			var calibration *paper.OMRCalibrationApproval
			if basePolicy.Reference != nil && basePolicy.RuntimeProfile.Mode == paper.OMRProfileModeTemplateDifference {
				calibration, err = s.loadApprovedOMRCalibrationTx(ctx, tx, tenantID, segment.templateID, segment.templateHash, segment.questionID, basePolicy.RuntimeProfile.Version, basePolicy.ProfileHash, basePolicy.Reference.FileAssetID, basePolicy.Reference.HashSHA256)
				if err != nil {
					return ScoringRun{}, err
				}
			}
			policy := paper.OMRAutoConfirmPolicyForTemplateReferenceAndCalibration(templateLayout, segment.templateStatus, segment.currentTemplateHash, segment.templateHash, currentReference, segment.questionID, segment.templateID, calibration)
			referenceAssetID, referenceSHA256 := "", ""
			if policy.Reference != nil {
				referenceAssetID, referenceSHA256 = policy.Reference.FileAssetID, policy.Reference.HashSHA256
			}
			calibrationID, calibrationEvidenceHash := "", ""
			if policy.Calibration != nil {
				calibrationID, calibrationEvidenceHash = policy.Calibration.ID, policy.Calibration.EvidenceHash
			}
			var omrID string
			err = tx.QueryRowContext(ctx, `INSERT INTO omr_run (tenant_id,scoring_run_id,answer_segment_id,crop_file_asset_id,crop_sha256,template_id,template_content_hash,profile_version,profile_hash,reference_file_asset_id,reference_sha256,calibration_session_id,calibration_evidence_hash,auto_confirm_min_confidence,auto_confirm_eligible,auto_confirm_reason)
VALUES ($1::uuid,$2::uuid,$3::uuid,$4::uuid,$5,$6::uuid,$7,$8,$9,NULLIF($10,'')::uuid,NULLIF($11,''),NULLIF($12,'')::uuid,NULLIF($13,''),$14,$15,$16) RETURNING id::text`, tenantID, runID, segment.id, segment.cropID, segment.cropHash, segment.templateID, segment.templateHash, policy.RuntimeProfile.Version, policy.ProfileHash, referenceAssetID, referenceSHA256, calibrationID, calibrationEvidenceHash, policy.MinimumConfidence, policy.AutoConfirmEligible, policy.Reason).Scan(&omrID)
			if err != nil {
				return ScoringRun{}, err
			}
			taskPayload := map[string]any{"exam_id": examID, "answer_segment_id": segment.id, "omr_run_id": omrID, "source_download_url": "/api/v1/answer-segments/" + segment.id + "/image", "source_content_type": segment.contentType, "option_regions": options, "multiple": segment.kind == "multiple_choice", "profile": policy.RuntimeProfile, "profile_hash": policy.ProfileHash}
			if policy.Reference != nil {
				taskPayload["reference"] = map[string]any{
					"file_asset_id": policy.Reference.FileAssetID,
					"download_url":  "/api/v1/files/" + policy.Reference.FileAssetID + "/download",
					"sha256":        policy.Reference.HashSHA256,
					"content_type":  policy.Reference.ContentType,
					"page_no":       policy.Reference.PageNo,
					"question_region": map[string]any{
						"x": policy.Reference.X, "y": policy.Reference.Y,
						"width": policy.Reference.Width, "height": policy.Reference.Height,
					},
				}
			}
			payload, _ := json.Marshal(taskPayload)
			var taskID string
			err = tx.QueryRowContext(ctx, `INSERT INTO agent_worker_task (tenant_id,task_type,queue_name,source_type,source_id,priority,payload,payload_schema_version,idempotency_key,dedupe_key,max_attempts,retry_backoff_seconds,created_by)
VALUES ($1::uuid,'omr_extract','page-processing','omr_run',$2::uuid,50,$3::jsonb,'omr-task-v2',$4,$4,3,5,$5::uuid)
ON CONFLICT (tenant_id,task_type,idempotency_key) DO UPDATE SET idempotency_key=EXCLUDED.idempotency_key RETURNING id::text`, tenantID, omrID, payload, "omr:"+omrID+":"+segment.cropHash, actorID).Scan(&taskID)
			if err != nil {
				return ScoringRun{}, err
			}
			if _, err = tx.ExecContext(ctx, `UPDATE omr_run SET runtime_task_id=$3::uuid WHERE tenant_id=$1::uuid AND id=$2::uuid`, tenantID, omrID, taskID); err != nil {
				return ScoringRun{}, err
			}
			queued++
		} else {
			reason := "rule_review_required"
			if isOMR && len(options) < 2 {
				reason = "omr_option_regions_missing"
			}
			if isOMR && segment.ruleID == "" {
				reason = "scoring_rule_missing"
			}
			created, insertErr := insertReviewTask(segment, "rule_review_required", reason)
			if insertErr != nil {
				return ScoringRun{}, insertErr
			}
			if created {
				review++
			}
		}
	}
	status := "processing"
	if queued == 0 && review > 0 {
		status = "needs_review"
	}
	if len(segments) == 0 {
		status = "failed"
	}
	_, err = tx.ExecContext(ctx, `UPDATE scoring_run SET status=$3,total_count=$4,queued_count=$5,review_count=$6,failed_count=$7,updated_at=now() WHERE tenant_id=$1::uuid AND id=$2::uuid`, tenantID, runID, status, len(segments), queued, review, map[bool]int{true: 1, false: 0}[len(segments) == 0])
	if err != nil {
		return ScoringRun{}, err
	}
	run, err := scanScoringRun(tx.QueryRowContext(ctx, scoringRunSelect+` WHERE tenant_id=$1::uuid AND id=$2::uuid`, tenantID, runID))
	if err != nil {
		return ScoringRun{}, err
	}
	return run, tx.Commit()
}

const scoringRunSelect = `SELECT id::text,tenant_id::text,exam_id::text,idempotency_key,status,total_count,queued_count,auto_confirmed_count,human_confirmed_count,review_count,failed_count,started_by::text,started_at,completed_at,created_at,updated_at FROM scoring_run`

func scanScoringRun(row ruleScanner) (ScoringRun, error) {
	var out ScoringRun
	var started, completed sql.NullTime
	err := row.Scan(&out.ID, &out.TenantID, &out.ExamID, &out.IdempotencyKey, &out.Status, &out.TotalCount, &out.QueuedCount, &out.AutoConfirmedCount, &out.HumanConfirmedCount, &out.ReviewCount, &out.FailedCount, &out.StartedBy, &started, &completed, &out.CreatedAt, &out.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ScoringRun{}, ErrNotFound
	}
	if err != nil {
		return ScoringRun{}, err
	}
	if started.Valid {
		v := started.Time.UTC()
		out.StartedAt = &v
	}
	if completed.Valid {
		v := completed.Time.UTC()
		out.CompletedAt = &v
	}
	return out, nil
}

func (s *PostgresStore) GetScoringSummary(ctx context.Context, tenantID, examID string) (ScoringSummary, error) {
	var summary ScoringSummary
	summary.Questions = []ScoringQuestionSummary{}
	run, err := scanScoringRun(s.db.QueryRowContext(ctx, scoringRunSelect+` WHERE tenant_id=$1::uuid AND exam_id=$2::uuid AND deleted_at IS NULL ORDER BY created_at DESC LIMIT 1`, tenantID, examID))
	if err == nil {
		summary.Run = &run
	} else if !errors.Is(err, ErrNotFound) {
		return summary, err
	}
	runID := ""
	if summary.Run != nil {
		runID = summary.Run.ID
	}
	rows, err := s.db.QueryContext(ctx, `SELECT q.id::text,q.question_no,q.question_type,count(seg.id)::int,
count(seg.id) FILTER(WHERE EXISTS(SELECT 1 FROM omr_run o WHERE o.tenant_id=seg.tenant_id AND o.answer_segment_id=seg.id AND o.scoring_run_id=NULLIF($3,'')::uuid AND o.status IN ('queued','processing') AND o.deleted_at IS NULL))::int,
count(seg.id) FILTER(WHERE EXISTS(SELECT 1 FROM question_grade g WHERE g.tenant_id=seg.tenant_id AND g.answer_segment_id=seg.id AND g.is_current AND g.deleted_at IS NULL))::int,
count(seg.id) FILTER(WHERE EXISTS(SELECT 1 FROM review_task rt WHERE rt.tenant_id=seg.tenant_id AND rt.answer_segment_id=seg.id AND rt.scoring_run_id=NULLIF($3,'')::uuid AND rt.status IN ('pending','assigned','in_progress','returned') AND rt.deleted_at IS NULL))::int,
count(seg.id) FILTER(WHERE EXISTS(SELECT 1 FROM omr_run o WHERE o.tenant_id=seg.tenant_id AND o.answer_segment_id=seg.id AND o.scoring_run_id=NULLIF($3,'')::uuid AND o.status='terminal_error' AND o.deleted_at IS NULL))::int
FROM question q LEFT JOIN answer_segment seg ON seg.tenant_id=q.tenant_id AND seg.question_id=q.id AND seg.deleted_at IS NULL
WHERE q.tenant_id=$1::uuid AND q.exam_id=$2::uuid AND q.deleted_at IS NULL GROUP BY q.id ORDER BY q.sort_order`, tenantID, examID, runID)
	if err != nil {
		return summary, err
	}
	defer rows.Close()
	for rows.Next() {
		var q ScoringQuestionSummary
		if err := rows.Scan(&q.QuestionID, &q.QuestionNo, &q.QuestionType, &q.Total, &q.Queued, &q.Confirmed, &q.Review, &q.Failed); err != nil {
			return summary, err
		}
		summary.Questions = append(summary.Questions, q)
	}
	return summary, rows.Err()
}

func (s *PostgresStore) GetScoringRunDetail(ctx context.Context, tenantID, runID string) (ScoringRunDetail, error) {
	run, err := scanScoringRun(s.db.QueryRowContext(ctx, scoringRunSelect+` WHERE tenant_id=$1::uuid AND id=$2::uuid AND deleted_at IS NULL`, tenantID, runID))
	if err != nil {
		return ScoringRunDetail{}, err
	}
	detail := ScoringRunDetail{Run: run, Items: []ScoringRunItem{}}
	rows, err := s.db.QueryContext(ctx, `
SELECT seg.id::text,q.id::text,q.question_no,q.question_type,COALESCE(NULLIF(sub.candidate_no,''),seg.id::text),
  COALESCE(ac.source,''),COALESCE(ac.display_text,''),COALESCE(ac.decision,''),ac.confidence::float8,
  COALESCE(ak.standard_answer,'null'::jsonb),COALESCE(sr.rule_type,''),g.score::float8,g.max_score::float8,COALESCE(g.source,''),
  COALESCE(o.id::text,''),COALESCE(o.status,''),COALESCE(o.runtime_task_id::text,''),COALESCE(wt.status,''),COALESCE(o.error_code,''),
  COALESCE(rt.id::text,''),COALESCE(rt.status,''),COALESCE(rt.reason_code,''),COALESCE(g.id::text,'')
FROM answer_segment seg
JOIN submission sub ON sub.tenant_id=seg.tenant_id AND sub.id=seg.submission_id AND sub.deleted_at IS NULL
JOIN question q ON q.tenant_id=seg.tenant_id AND q.id=seg.question_id AND q.deleted_at IS NULL
LEFT JOIN omr_run o ON o.tenant_id=seg.tenant_id AND o.scoring_run_id=$3::uuid AND o.answer_segment_id=seg.id AND o.deleted_at IS NULL
LEFT JOIN agent_worker_task wt ON wt.tenant_id=seg.tenant_id AND wt.id=o.runtime_task_id
LEFT JOIN LATERAL (
  SELECT source,display_text,decision,confidence FROM answer_candidate
  WHERE tenant_id=seg.tenant_id AND scoring_run_id=$3::uuid AND answer_segment_id=seg.id AND deleted_at IS NULL
  ORDER BY created_at DESC LIMIT 1
) ac ON true
LEFT JOIN LATERAL (
  SELECT standard_answer FROM question_answer_key
  WHERE tenant_id=q.tenant_id AND question_id=q.id AND deleted_at IS NULL
  ORDER BY created_at DESC LIMIT 1
) ak ON true
LEFT JOIN LATERAL (
  SELECT rule_type FROM scoring_rule
  WHERE tenant_id=q.tenant_id AND question_id=q.id AND status='published' AND deleted_at IS NULL
  ORDER BY version DESC LIMIT 1
) sr ON true
LEFT JOIN LATERAL (
  SELECT id::text,status,reason_code FROM review_task
  WHERE tenant_id=seg.tenant_id AND scoring_run_id=$3::uuid AND answer_segment_id=seg.id AND deleted_at IS NULL
  ORDER BY created_at DESC LIMIT 1
) rt ON true
LEFT JOIN LATERAL (
  SELECT id::text,source,score,max_score FROM question_grade
  WHERE tenant_id=seg.tenant_id AND scoring_run_id=$3::uuid AND answer_segment_id=seg.id AND is_current AND deleted_at IS NULL
  ORDER BY created_at DESC LIMIT 1
) g ON true
WHERE seg.tenant_id=$1::uuid AND sub.exam_id=$2::uuid AND seg.deleted_at IS NULL
  AND (
    EXISTS (SELECT 1 FROM omr_run run_omr WHERE run_omr.tenant_id=seg.tenant_id AND run_omr.scoring_run_id=$3::uuid AND run_omr.answer_segment_id=seg.id AND run_omr.deleted_at IS NULL)
    OR EXISTS (SELECT 1 FROM answer_candidate run_candidate WHERE run_candidate.tenant_id=seg.tenant_id AND run_candidate.scoring_run_id=$3::uuid AND run_candidate.answer_segment_id=seg.id AND run_candidate.deleted_at IS NULL)
    OR EXISTS (SELECT 1 FROM review_task run_review WHERE run_review.tenant_id=seg.tenant_id AND run_review.scoring_run_id=$3::uuid AND run_review.answer_segment_id=seg.id AND run_review.deleted_at IS NULL)
    OR EXISTS (SELECT 1 FROM question_grade run_grade WHERE run_grade.tenant_id=seg.tenant_id AND run_grade.scoring_run_id=$3::uuid AND run_grade.answer_segment_id=seg.id AND run_grade.deleted_at IS NULL)
  )
ORDER BY q.sort_order,seg.created_at`, tenantID, run.ExamID, runID)
	if err != nil {
		return ScoringRunDetail{}, err
	}
	defer rows.Close()
	for rows.Next() {
		item, scanErr := scanScoringRunItem(rows, run.Status)
		if scanErr != nil {
			return ScoringRunDetail{}, scanErr
		}
		detail.Items = append(detail.Items, item)
	}
	return detail, rows.Err()
}

// GetExamAutomationResults returns the current, exam-wide objective grading
// facts independently of a particular orchestration run. This is the operator
// view: a rerun is process history, while the latest recognition and grade are
// the facts that must be auditable during live marking.
func (s *PostgresStore) GetExamAutomationResults(ctx context.Context, tenantID, examID string) (ExamAutomationResults, error) {
	out := ExamAutomationResults{Items: []ScoringRunItem{}}
	rows, err := s.db.QueryContext(ctx, `
SELECT seg.id::text,q.id::text,q.question_no,q.question_type,COALESCE(NULLIF(sub.candidate_no,''),seg.id::text),
  COALESCE(NULLIF(ac.source,''),ans.source,''),COALESCE(NULLIF(ac.display_text,''),ans.answer_text,''),
  COALESCE(NULLIF(ac.decision,''),CASE WHEN ans.id IS NOT NULL THEN 'confirmed' ELSE '' END),COALESCE(ac.confidence,ans.confidence)::float8,
  COALESCE(ak.standard_answer,'null'::jsonb),COALESCE(sr.rule_type,q.question_type),
  COALESCE(g.score,ag.suggested_score)::float8,COALESCE(g.max_score,ag.max_score)::float8,
  COALESCE(g.source,CASE WHEN ag.id IS NOT NULL THEN 'ai_suggestion' ELSE '' END),
  COALESCE(o.id::text,''),COALESCE(o.status,CASE WHEN ag.status='failed' THEN 'terminal_error' ELSE '' END),
  COALESCE(o.runtime_task_id::text,''),COALESCE(wt.status,''),
  COALESCE(o.error_code,CASE WHEN ag.status='failed' THEN ag.failure_reason ELSE '' END,''),
  COALESCE(rt.id::text,''),COALESCE(rt.status,''),COALESCE(rt.reason_code,''),COALESCE(g.id::text,'')
FROM answer_segment seg
JOIN submission sub ON sub.tenant_id=seg.tenant_id AND sub.id=seg.submission_id AND sub.deleted_at IS NULL
JOIN question q ON q.tenant_id=seg.tenant_id AND q.id=seg.question_id AND q.deleted_at IS NULL
LEFT JOIN LATERAL (
  SELECT id,source,display_text,decision,confidence FROM answer_candidate
  WHERE tenant_id=seg.tenant_id AND answer_segment_id=seg.id AND is_current AND deleted_at IS NULL
  ORDER BY created_at DESC LIMIT 1
) ac ON true
LEFT JOIN LATERAL (
  SELECT id,source,answer_text,confidence FROM answer_segment_answer
  WHERE tenant_id=seg.tenant_id AND answer_segment_id=seg.id AND deleted_at IS NULL
  ORDER BY created_at DESC LIMIT 1
) ans ON true
LEFT JOIN LATERAL (
  SELECT standard_answer FROM question_answer_key
  WHERE tenant_id=q.tenant_id AND question_id=q.id AND deleted_at IS NULL
  ORDER BY created_at DESC LIMIT 1
) ak ON true
LEFT JOIN LATERAL (
  SELECT rule_type FROM scoring_rule
  WHERE tenant_id=q.tenant_id AND question_id=q.id AND status='published' AND deleted_at IS NULL
  ORDER BY version DESC LIMIT 1
) sr ON true
LEFT JOIN LATERAL (
  SELECT id,source,score,max_score FROM question_grade
  WHERE tenant_id=seg.tenant_id AND answer_segment_id=seg.id AND is_current AND deleted_at IS NULL
  ORDER BY created_at DESC LIMIT 1
) g ON true
LEFT JOIN LATERAL (
  SELECT id,suggested_score,max_score,status,failure_reason FROM ai_grade
  WHERE tenant_id=seg.tenant_id AND answer_segment_id=seg.id AND deleted_at IS NULL
  ORDER BY created_at DESC LIMIT 1
) ag ON true
LEFT JOIN LATERAL (
  SELECT id,status,runtime_task_id,error_code FROM omr_run
  WHERE tenant_id=seg.tenant_id AND answer_segment_id=seg.id AND deleted_at IS NULL
  ORDER BY created_at DESC LIMIT 1
) o ON true
LEFT JOIN agent_worker_task wt ON wt.tenant_id=seg.tenant_id AND wt.id=o.runtime_task_id
LEFT JOIN LATERAL (
  SELECT id,status,reason_code FROM review_task
  WHERE tenant_id=seg.tenant_id AND answer_segment_id=seg.id AND deleted_at IS NULL
  ORDER BY created_at DESC LIMIT 1
) rt ON true
WHERE seg.tenant_id=$1::uuid AND sub.exam_id=$2::uuid AND seg.deleted_at IS NULL
  AND q.question_type IN ('single_choice','multiple_choice','true_false','fill_blank','numeric')
ORDER BY COALESCE(NULLIF(sub.candidate_no,''),seg.id::text),q.sort_order,seg.created_at`, tenantID, examID)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		item, scanErr := scanScoringRunItem(rows, "")
		if scanErr != nil {
			return ExamAutomationResults{}, scanErr
		}
		out.Items = append(out.Items, item)
	}
	return out, rows.Err()
}

func scanScoringRunItem(row ruleScanner, runStatus string) (ScoringRunItem, error) {
	var item ScoringRunItem
	var recognitionConfidence, score, maxScore sql.NullFloat64
	var standardAnswerRaw []byte
	var gradeID string
	if err := row.Scan(&item.AnswerSegmentID, &item.QuestionID, &item.QuestionNo, &item.QuestionType, &item.AnonymousCode,
		&item.RecognitionSource, &item.RecognizedAnswer, &item.RecognitionDecision, &recognitionConfidence,
		&standardAnswerRaw, &item.RuleType, &score, &maxScore, &item.GradeSource,
		&item.OMRRunID, &item.State, &item.RuntimeTaskID, &item.RuntimeStatus, &item.ErrorCode,
		&item.ReviewTaskID, &item.ReviewStatus, &item.ReasonCode, &gradeID); err != nil {
		return ScoringRunItem{}, err
	}
	if recognitionConfidence.Valid {
		value := recognitionConfidence.Float64
		item.RecognitionConfidence = &value
	}
	if score.Valid {
		value := score.Float64
		item.Score = &value
	}
	if maxScore.Valid {
		value := maxScore.Float64
		item.MaxScore = &value
	}
	_ = json.Unmarshal(standardAnswerRaw, &item.StandardAnswer)
	item.State = scoringItemState(runStatus, item.State, item.ReviewStatus, gradeID)
	return item, nil
}

func scoringItemState(runStatus, omrStatus, reviewStatus, gradeID string) string {
	if runStatus == "cancelled" || runStatus == "cancelling" {
		return runStatus
	}
	if omrStatus == "terminal_error" || (runStatus == "failed" && gradeID == "" && reviewStatus == "") {
		return "failed"
	}
	if reviewStatus == "pending" || reviewStatus == "assigned" || reviewStatus == "in_progress" || reviewStatus == "returned" {
		return "review"
	}
	if gradeID != "" {
		return "confirmed"
	}
	if omrStatus == "queued" || omrStatus == "processing" || omrStatus == "retryable_error" {
		return "processing"
	}
	return "pending"
}

func (s *PostgresStore) BeginScoringRunCancellation(ctx context.Context, tenantID, runID string) (ScoringRun, []string, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ScoringRun{}, nil, err
	}
	defer tx.Rollback()
	run, err := scanScoringRun(tx.QueryRowContext(ctx, scoringRunSelect+` WHERE tenant_id=$1::uuid AND id=$2::uuid AND deleted_at IS NULL FOR UPDATE`, tenantID, runID))
	if err != nil {
		return ScoringRun{}, nil, err
	}
	if run.Status == "cancelled" {
		return run, []string{}, tx.Commit()
	}
	if run.Status != "cancelling" {
		if run.Status != "queued" && run.Status != "processing" && run.Status != "needs_review" && run.Status != "failed" {
			return ScoringRun{}, nil, ErrInvalidTransition
		}
		if _, err = tx.ExecContext(ctx, `UPDATE scoring_run SET status='cancelling',updated_at=now() WHERE tenant_id=$1::uuid AND id=$2::uuid`, tenantID, runID); err != nil {
			return ScoringRun{}, nil, err
		}
	}
	rows, err := tx.QueryContext(ctx, `
SELECT o.runtime_task_id::text
FROM omr_run o
JOIN agent_worker_task wt ON wt.tenant_id=o.tenant_id AND wt.id=o.runtime_task_id
WHERE o.tenant_id=$1::uuid AND o.scoring_run_id=$2::uuid AND o.runtime_task_id IS NOT NULL AND o.deleted_at IS NULL
  AND wt.status IN ('queued','leased','running')
ORDER BY o.created_at`, tenantID, runID)
	if err != nil {
		return ScoringRun{}, nil, err
	}
	taskIDs := []string{}
	for rows.Next() {
		var taskID string
		if err := rows.Scan(&taskID); err != nil {
			rows.Close()
			return ScoringRun{}, nil, err
		}
		taskIDs = append(taskIDs, taskID)
	}
	if err = rows.Close(); err != nil {
		return ScoringRun{}, nil, err
	}
	run, err = scanScoringRun(tx.QueryRowContext(ctx, scoringRunSelect+` WHERE tenant_id=$1::uuid AND id=$2::uuid`, tenantID, runID))
	if err != nil {
		return ScoringRun{}, nil, err
	}
	return run, taskIDs, tx.Commit()
}

func (s *PostgresStore) FinalizeScoringRunCancellation(ctx context.Context, tenantID, runID string) (ScoringRun, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ScoringRun{}, err
	}
	defer tx.Rollback()
	run, err := scanScoringRun(tx.QueryRowContext(ctx, scoringRunSelect+` WHERE tenant_id=$1::uuid AND id=$2::uuid AND deleted_at IS NULL FOR UPDATE`, tenantID, runID))
	if err != nil {
		return ScoringRun{}, err
	}
	if run.Status == "cancelled" {
		return run, tx.Commit()
	}
	if run.Status != "cancelling" {
		return ScoringRun{}, ErrInvalidTransition
	}
	var active int
	if err = tx.QueryRowContext(ctx, `
SELECT count(*)
FROM omr_run o
JOIN agent_worker_task wt ON wt.tenant_id=o.tenant_id AND wt.id=o.runtime_task_id
WHERE o.tenant_id=$1::uuid AND o.scoring_run_id=$2::uuid AND o.deleted_at IS NULL
  AND wt.status IN ('queued','leased','running')`, tenantID, runID).Scan(&active); err != nil {
		return ScoringRun{}, err
	}
	if active > 0 {
		return ScoringRun{}, ErrInvalidTransition
	}
	if _, err = tx.ExecContext(ctx, `UPDATE omr_run SET status='invalidated',completed_at=COALESCE(completed_at,now()),updated_at=now() WHERE tenant_id=$1::uuid AND scoring_run_id=$2::uuid AND status IN ('queued','processing','retryable_error','terminal_error') AND deleted_at IS NULL`, tenantID, runID); err != nil {
		return ScoringRun{}, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE review_task SET status='cancelled',assigned_to=NULL,claimed_at=NULL,claim_expires_at=NULL,return_reason='scoring_run_cancelled',revision=revision+1,updated_at=now() WHERE tenant_id=$1::uuid AND scoring_run_id=$2::uuid AND status IN ('pending','assigned','in_progress','returned') AND deleted_at IS NULL`, tenantID, runID); err != nil {
		return ScoringRun{}, err
	}
	run, err = scanScoringRun(tx.QueryRowContext(ctx, `
UPDATE scoring_run
SET status='cancelled',cancelled_at=now(),completed_at=now(),queued_count=0,review_count=0,updated_at=now()
WHERE tenant_id=$1::uuid AND id=$2::uuid
RETURNING id::text,tenant_id::text,exam_id::text,idempotency_key,status,total_count,queued_count,auto_confirmed_count,human_confirmed_count,review_count,failed_count,started_by::text,started_at,completed_at,created_at,updated_at`, tenantID, runID))
	if err != nil {
		return ScoringRun{}, err
	}
	return run, tx.Commit()
}

func (s *PostgresStore) ListFailedOMRTasks(ctx context.Context, tenantID, runID string) ([]FailedOMRTask, error) {
	if _, err := scanScoringRun(s.db.QueryRowContext(ctx, scoringRunSelect+` WHERE tenant_id=$1::uuid AND id=$2::uuid AND deleted_at IS NULL`, tenantID, runID)); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT o.id::text,o.runtime_task_id::text
FROM omr_run o
JOIN agent_worker_task wt ON wt.tenant_id=o.tenant_id AND wt.id=o.runtime_task_id
JOIN scoring_run sr ON sr.tenant_id=o.tenant_id AND sr.id=o.scoring_run_id
WHERE o.tenant_id=$1::uuid AND o.scoring_run_id=$2::uuid AND o.status='terminal_error' AND o.deleted_at IS NULL
  AND sr.status NOT IN ('cancelling','cancelled') AND wt.status IN ('failed','dead_letter')
ORDER BY o.created_at`, tenantID, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []FailedOMRTask{}
	for rows.Next() {
		var item FailedOMRTask
		if err := rows.Scan(&item.OMRRunID, &item.RuntimeTaskID); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *PostgresStore) PrepareOMRRetry(ctx context.Context, tenantID, omrRunID string) (FailedOMRTask, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return FailedOMRTask{}, err
	}
	defer tx.Rollback()
	var item FailedOMRTask
	var runStatus string
	err = tx.QueryRowContext(ctx, `
SELECT o.id::text,o.runtime_task_id::text,sr.status
FROM omr_run o
JOIN scoring_run sr ON sr.tenant_id=o.tenant_id AND sr.id=o.scoring_run_id
WHERE o.tenant_id=$1::uuid AND o.id=$2::uuid AND o.deleted_at IS NULL
FOR UPDATE OF o,sr`, tenantID, omrRunID).Scan(&item.OMRRunID, &item.RuntimeTaskID, &runStatus)
	if errors.Is(err, sql.ErrNoRows) {
		return FailedOMRTask{}, ErrNotFound
	}
	if err != nil {
		return FailedOMRTask{}, err
	}
	if runStatus == "cancelling" || runStatus == "cancelled" {
		return FailedOMRTask{}, ErrInvalidTransition
	}
	result, err := tx.ExecContext(ctx, `UPDATE omr_run SET status='queued',completed_at=NULL,updated_at=now() WHERE tenant_id=$1::uuid AND id=$2::uuid AND status='terminal_error'`, tenantID, omrRunID)
	if err != nil {
		return FailedOMRTask{}, err
	}
	if count, _ := result.RowsAffected(); count != 1 {
		return FailedOMRTask{}, ErrInvalidTransition
	}
	return item, tx.Commit()
}

func (s *PostgresStore) RestoreOMRRetry(ctx context.Context, tenantID, omrRunID string) error {
	result, err := s.db.ExecContext(ctx, `UPDATE omr_run SET status='terminal_error',completed_at=now(),updated_at=now() WHERE tenant_id=$1::uuid AND id=$2::uuid AND status='queued'`, tenantID, omrRunID)
	if err != nil {
		return err
	}
	if count, _ := result.RowsAffected(); count != 1 {
		return ErrInvalidTransition
	}
	return nil
}

func (s *PostgresStore) RefreshScoringRun(ctx context.Context, tenantID, runID string) (ScoringRun, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ScoringRun{}, err
	}
	defer tx.Rollback()
	run, err := s.refreshScoringRunTx(ctx, tx, tenantID, runID)
	if err != nil {
		return ScoringRun{}, err
	}
	return run, tx.Commit()
}

func (s *PostgresStore) refreshScoringRunTx(ctx context.Context, tx *sql.Tx, tenantID, runID string) (ScoringRun, error) {
	run, err := scanScoringRun(tx.QueryRowContext(ctx, scoringRunSelect+` WHERE tenant_id=$1::uuid AND id=$2::uuid AND deleted_at IS NULL FOR UPDATE`, tenantID, runID))
	if err != nil {
		return ScoringRun{}, err
	}
	if run.Status == "cancelling" || run.Status == "cancelled" {
		return run, nil
	}
	var queued, review, failed, autoConfirmed, humanConfirmed int
	err = tx.QueryRowContext(ctx, `
SELECT
  (SELECT count(*) FROM omr_run WHERE tenant_id=$1::uuid AND scoring_run_id=$2::uuid AND status IN ('queued','processing','retryable_error') AND deleted_at IS NULL),
  (SELECT count(*) FROM review_task WHERE tenant_id=$1::uuid AND scoring_run_id=$2::uuid AND status IN ('pending','assigned','in_progress','returned') AND deleted_at IS NULL),
  (SELECT count(*) FROM omr_run WHERE tenant_id=$1::uuid AND scoring_run_id=$2::uuid AND status='terminal_error' AND deleted_at IS NULL),
  (SELECT count(*) FROM question_grade WHERE tenant_id=$1::uuid AND scoring_run_id=$2::uuid AND source='rule_confirmed' AND is_current AND status='confirmed' AND deleted_at IS NULL),
  (SELECT count(*) FROM question_grade WHERE tenant_id=$1::uuid AND scoring_run_id=$2::uuid AND source='human' AND is_current AND status='confirmed' AND deleted_at IS NULL)
`, tenantID, runID).Scan(&queued, &review, &failed, &autoConfirmed, &humanConfirmed)
	if err != nil {
		return ScoringRun{}, err
	}
	status := "processing"
	if run.TotalCount == 0 {
		status = "failed"
	} else if failed > 0 {
		status = "failed"
	} else if queued == 0 && review > 0 {
		status = "needs_review"
	} else if queued == 0 && review == 0 && autoConfirmed+humanConfirmed >= run.TotalCount {
		status = "completed"
	}
	return scanScoringRun(tx.QueryRowContext(ctx, `
UPDATE scoring_run
SET status=$3,queued_count=$4,review_count=$5,failed_count=$6,auto_confirmed_count=$7,human_confirmed_count=$8,
  completed_at=CASE WHEN $3='completed' THEN now() ELSE NULL END,updated_at=now()
WHERE tenant_id=$1::uuid AND id=$2::uuid
RETURNING id::text,tenant_id::text,exam_id::text,idempotency_key,status,total_count,queued_count,auto_confirmed_count,human_confirmed_count,review_count,failed_count,started_by::text,started_at,completed_at,created_at,updated_at`, tenantID, runID, status, queued, review, failed, autoConfirmed, humanConfirmed))
}

func (s *PostgresStore) GetOMRRun(ctx context.Context, tenantID, id string) (OMRRun, error) {
	row := s.db.QueryRowContext(ctx, `SELECT o.id::text,o.tenant_id::text,o.scoring_run_id::text,o.answer_segment_id::text,s.exam_id::text,COALESCE(o.runtime_task_id::text,''),o.status,COALESCE(o.decision,''),o.confidence,o.measurements,o.selected_options,COALESCE(o.overlay_file_asset_id::text,''),o.crop_sha256,COALESCE(o.reference_file_asset_id::text,''),COALESCE(o.reference_sha256,''),COALESCE(o.calibration_session_id::text,''),COALESCE(o.calibration_evidence_hash,''),o.profile_version,o.profile_hash,o.auto_confirm_min_confidence,o.auto_confirm_eligible,o.auto_confirm_reason FROM omr_run o JOIN scoring_run s ON s.tenant_id=o.tenant_id AND s.id=o.scoring_run_id WHERE o.tenant_id=$1::uuid AND o.id=$2::uuid AND o.deleted_at IS NULL`, tenantID, id)
	var out OMRRun
	var confidence sql.NullFloat64
	var measurements, selected []byte
	err := row.Scan(&out.ID, &out.TenantID, &out.ScoringRunID, &out.AnswerSegmentID, &out.ExamID, &out.RuntimeTaskID, &out.Status, &out.Decision, &confidence, &measurements, &selected, &out.OverlayFileAssetID, &out.CropSHA256, &out.ReferenceFileAssetID, &out.ReferenceSHA256, &out.CalibrationSessionID, &out.CalibrationEvidenceHash, &out.ProfileVersion, &out.ProfileHash, &out.AutoConfirmMinConfidence, &out.AutoConfirmEligible, &out.AutoConfirmReason)
	if errors.Is(err, sql.ErrNoRows) {
		return out, ErrNotFound
	}
	if err != nil {
		return out, err
	}
	if confidence.Valid {
		v := confidence.Float64
		out.Confidence = &v
	}
	_ = json.Unmarshal(measurements, &out.Measurements)
	_ = json.Unmarshal(selected, &out.SelectedOptions)
	return out, nil
}

// ApplyOMRResult commits the worker result, OMR facts, and either an automatic
// grade or an actionable review task in one transaction. A worker result must
// never become durable without a durable next grading action.
func (s *PostgresStore) ApplyOMRResult(ctx context.Context, tenantID, id, actorID string, input OMRResultInput, engine *Engine) (OMRRun, *QuestionGrade, error) {
	if input.Decision != "selected" && input.Decision != "blank" && input.Decision != "multiple" && input.Decision != "ambiguous" {
		return OMRRun{}, nil, ErrInvalidInput
	}
	if input.Confidence < 0 || input.Confidence > 1 || input.ResultVersion == "" || strings.TrimSpace(input.ProfileVersion) == "" || strings.TrimSpace(input.ProfileHash) == "" {
		return OMRRun{}, nil, ErrInvalidInput
	}
	measurements, err := json.Marshal(input.Measurements)
	if err != nil {
		return OMRRun{}, nil, ErrInvalidInput
	}
	selected, err := json.Marshal(input.Selected)
	if err != nil {
		return OMRRun{}, nil, ErrInvalidInput
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return OMRRun{}, nil, err
	}
	defer tx.Rollback()
	var currentStatus, currentVersion, expectedProfileVersion, expectedProfileHash, expectedReferenceAssetID, expectedReferenceSHA256, expectedCalibrationID, expectedCalibrationEvidenceHash, calibrationStatus, calibrationEvidenceHash, autoConfirmReason string
	var autoConfirmEligible bool
	var autoConfirmMinimumConfidence float64
	if err = tx.QueryRowContext(ctx, `SELECT o.status,COALESCE(o.result_version,''),o.profile_version,o.profile_hash,COALESCE(o.reference_file_asset_id::text,''),COALESCE(o.reference_sha256,''),COALESCE(o.calibration_session_id::text,''),COALESCE(o.calibration_evidence_hash,''),COALESCE(c.status,''),COALESCE(c.evidence_hash,''),o.auto_confirm_min_confidence,o.auto_confirm_eligible,o.auto_confirm_reason FROM omr_run o LEFT JOIN omr_calibration_session c ON c.tenant_id=o.tenant_id AND c.id=o.calibration_session_id WHERE o.tenant_id=$1::uuid AND o.id=$2::uuid AND o.deleted_at IS NULL FOR UPDATE OF o`, tenantID, id).Scan(&currentStatus, &currentVersion, &expectedProfileVersion, &expectedProfileHash, &expectedReferenceAssetID, &expectedReferenceSHA256, &expectedCalibrationID, &expectedCalibrationEvidenceHash, &calibrationStatus, &calibrationEvidenceHash, &autoConfirmMinimumConfidence, &autoConfirmEligible, &autoConfirmReason); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return OMRRun{}, nil, ErrNotFound
		}
		return OMRRun{}, nil, err
	}
	if currentStatus != "completed" && (input.ProfileVersion != expectedProfileVersion || input.ProfileHash != expectedProfileHash || input.ReferenceSHA256 != expectedReferenceSHA256) {
		return OMRRun{}, nil, ErrInvalidInput
	}
	if currentStatus != "completed" && expectedCalibrationID != "" && (calibrationStatus != "approved" || calibrationEvidenceHash != expectedCalibrationEvidenceHash) {
		autoConfirmEligible = false
		autoConfirmReason = paper.OMRAutoConfirmReasonCalibrationRevoked
		if _, err = tx.ExecContext(ctx, `UPDATE omr_run SET auto_confirm_eligible=false,auto_confirm_reason=$3,updated_at=now() WHERE tenant_id=$1::uuid AND id=$2::uuid`, tenantID, id, autoConfirmReason); err != nil {
			return OMRRun{}, nil, err
		}
	}
	task, err := workerruntime.CompleteTaskInTx(ctx, tx, tenantID, input.TaskID, workerruntime.CompleteInput{LeaseToken: input.LeaseToken, ResultSchemaVersion: "omr-result-v1", Result: omrRuntimeResult(id, input), DurationMS: input.DurationMS})
	if err != nil {
		return OMRRun{}, nil, err
	}
	if task.SourceType != "omr_run" || task.SourceID != id {
		return OMRRun{}, nil, ErrInvalidTransition
	}
	if currentStatus == "completed" {
		if currentVersion != input.ResultVersion {
			return OMRRun{}, nil, ErrRevisionConflict
		}
		if err = tx.Commit(); err != nil {
			return OMRRun{}, nil, err
		}
		out, getErr := s.GetOMRRun(ctx, tenantID, id)
		return out, nil, getErr
	}
	var segmentID, scoringRunID string
	err = tx.QueryRowContext(ctx, `
UPDATE omr_run o
SET status='completed',decision=$3,confidence=$4,measurements=$5::jsonb,selected_options=$6::jsonb,
  overlay_file_asset_id=$7::uuid,result_version=$8,duration_ms=$9,completed_at=now(),updated_at=now()
FROM scoring_run sr
WHERE o.tenant_id=$1::uuid AND o.id=$2::uuid AND o.status IN ('queued','processing')
	  AND o.runtime_task_id=$10::uuid
  AND sr.tenant_id=o.tenant_id AND sr.id=o.scoring_run_id AND sr.status='processing'
RETURNING o.answer_segment_id::text,o.scoring_run_id::text`, tenantID, id, input.Decision, input.Confidence, measurements, selected, input.OverlayFileAssetID, input.ResultVersion, input.DurationMS, task.ID).Scan(&segmentID, &scoringRunID)
	if errors.Is(err, sql.ErrNoRows) {
		return OMRRun{}, nil, ErrRevisionConflict
	}
	if err != nil {
		return OMRRun{}, nil, err
	}
	evidence, err := json.Marshal(map[string]any{
		"measurements":                input.Measurements,
		"thresholds":                  input.Thresholds,
		"needs_human_review":          input.NeedsHumanReview,
		"profile_version":             expectedProfileVersion,
		"profile_hash":                expectedProfileHash,
		"reference_file_asset_id":     expectedReferenceAssetID,
		"reference_sha256":            expectedReferenceSHA256,
		"calibration_session_id":      expectedCalibrationID,
		"calibration_evidence_hash":   expectedCalibrationEvidenceHash,
		"auto_confirm_min_confidence": autoConfirmMinimumConfidence,
		"auto_confirm_eligible":       autoConfirmEligible,
		"auto_confirm_reason":         autoConfirmReason,
		"overlay_file_asset_id":       input.OverlayFileAssetID,
		"overlay_sha256":              input.OverlaySHA256,
	})
	if err != nil {
		return OMRRun{}, nil, ErrInvalidInput
	}
	_, err = tx.ExecContext(ctx, `UPDATE answer_candidate SET is_current=false WHERE tenant_id=$1::uuid AND answer_segment_id=$2::uuid AND is_current`, tenantID, segmentID)
	if err != nil {
		return OMRRun{}, nil, err
	}
	answerText := strings.Join(input.Selected, ",")
	decision := input.Decision
	if omrResultCanAutoConfirm(input, autoConfirmEligible, autoConfirmMinimumConfidence) {
		decision = "confirmed"
	}
	var candidateID string
	err = tx.QueryRowContext(ctx, `INSERT INTO answer_candidate(tenant_id,answer_segment_id,scoring_run_id,source,payload,display_text,confidence,decision,evidence,engine_version,profile_version,input_hash,is_current,created_by) SELECT $1::uuid,$2::uuid,$3::uuid,'omr',jsonb_build_object('answers',$4::jsonb),$5,$6,$7,$8::jsonb,'opencv-omr-v1',$9,seg.crop_sha256,true,$10::uuid FROM answer_segment seg WHERE seg.tenant_id=$1::uuid AND seg.id=$2::uuid RETURNING id::text`, tenantID, segmentID, scoringRunID, selected, answerText, input.Confidence, decision, evidence, expectedProfileVersion, actorID).Scan(&candidateID)
	if err != nil {
		return OMRRun{}, nil, err
	}
	var answerID string
	err = tx.QueryRowContext(ctx, `INSERT INTO answer_segment_answer(tenant_id,answer_segment_id,answer_text,answer_payload,source,confidence,recorded_by) VALUES($1::uuid,$2::uuid,$3,jsonb_build_object('answers',$4::jsonb), 'omr',$5,$6::uuid) RETURNING id::text`, tenantID, segmentID, answerText, selected, input.Confidence, actorID).Scan(&answerID)
	if err != nil {
		return OMRRun{}, nil, err
	}
	var questionGrade *QuestionGrade
	if decision != "confirmed" {
		err = s.insertScoringReviewTaskTx(ctx, tx, tenantID, segmentID, scoringRunID, actorID, "omr_ambiguous", omrReviewReason(input, autoConfirmEligible, autoConfirmReason, autoConfirmMinimumConfidence))
	} else if engine == nil {
		err = s.insertScoringReviewTaskTx(ctx, tx, tenantID, segmentID, scoringRunID, actorID, "grading_failure", "auto_grade_engine_unavailable")
	} else {
		gradingContext, contextErr := s.loadContextForAnswerTx(ctx, tx, tenantID, segmentID, answerID)
		if contextErr != nil {
			err = s.insertScoringReviewTaskTx(ctx, tx, tenantID, segmentID, scoringRunID, actorID, "grading_failure", "auto_grade_context_failed")
		} else {
			evaluation, gradeErr := engine.Grade(gradingContext)
			if gradeErr != nil {
				err = s.insertScoringReviewTaskTx(ctx, tx, tenantID, segmentID, scoringRunID, actorID, "grading_failure", "auto_grade_execution_failed")
			} else if !evaluation.AutoPass || evaluation.NeedsHumanReview {
				err = s.insertScoringReviewTaskTx(ctx, tx, tenantID, segmentID, scoringRunID, actorID, "rule_review_required", "rule_not_auto_confirmed")
			} else {
				confirmed, confirmErr := s.confirmRuleGradeForCandidateTx(ctx, tx, tenantID, segmentID, candidateID, actorID, evaluation)
				if confirmErr != nil {
					err = s.insertScoringReviewTaskTx(ctx, tx, tenantID, segmentID, scoringRunID, actorID, "grading_failure", "auto_grade_confirmation_failed")
				} else {
					questionGrade = &confirmed
				}
			}
		}
	}
	if err != nil {
		return OMRRun{}, nil, err
	}
	if _, err = s.refreshScoringRunTx(ctx, tx, tenantID, scoringRunID); err != nil {
		return OMRRun{}, nil, err
	}
	if err = tx.Commit(); err != nil {
		return OMRRun{}, nil, err
	}
	out, getErr := s.GetOMRRun(ctx, tenantID, id)
	return out, questionGrade, getErr
}

// omrRuntimeResult is the exact semantic payload used by Worker Runtime's
// idempotency hash. Keeping every persisted OMR decision input here prevents a
// replay with the same result version from silently changing the durable facts.
func omrRuntimeResult(id string, input OMRResultInput) map[string]any {
	return map[string]any{
		"omr_run_id":            id,
		"result_version":        input.ResultVersion,
		"decision":              input.Decision,
		"selected":              input.Selected,
		"confidence":            input.Confidence,
		"needs_human_review":    input.NeedsHumanReview,
		"measurements":          input.Measurements,
		"profile_version":       input.ProfileVersion,
		"profile_hash":          input.ProfileHash,
		"reference_sha256":      input.ReferenceSHA256,
		"thresholds":            input.Thresholds,
		"overlay_file_asset_id": input.OverlayFileAssetID,
		"overlay_sha256":        input.OverlaySHA256,
	}
}

func omrResultCanAutoConfirm(input OMRResultInput, autoConfirmEligible bool, minimumConfidence float64) bool {
	return autoConfirmEligible && input.Decision == "selected" && len(input.Selected) == 1 && !input.NeedsHumanReview && input.Confidence >= minimumConfidence
}

func omrReviewReason(input OMRResultInput, autoConfirmEligible bool, autoConfirmReason string, minimumConfidence float64) string {
	if input.Decision == "selected" && len(input.Selected) == 1 && !input.NeedsHumanReview && input.Confidence >= minimumConfidence && !autoConfirmEligible && strings.TrimSpace(autoConfirmReason) != "" {
		return "omr_" + autoConfirmReason
	}
	return "omr_" + input.Decision
}

func (s *PostgresStore) insertScoringReviewTaskTx(ctx context.Context, tx *sql.Tx, tenantID, segmentID, scoringRunID, actorID, source, reason string) error {
	_, err := tx.ExecContext(ctx, `
INSERT INTO review_task(tenant_id,exam_id,question_id,question_no,answer_segment_id,submission_id,anonymous_code,source,status,priority,grade_round,reason_code,scoring_run_id,created_by)
SELECT seg.tenant_id,sub.exam_id,q.id,q.question_no,seg.id,sub.id,COALESCE(NULLIF(sub.candidate_no,''),seg.id::text),$3,'pending',80,'single',$4,$5::uuid,$6::uuid
FROM answer_segment seg
JOIN submission sub ON sub.tenant_id=seg.tenant_id AND sub.id=seg.submission_id AND sub.deleted_at IS NULL
JOIN question q ON q.tenant_id=seg.tenant_id AND q.id=seg.question_id AND q.deleted_at IS NULL
WHERE seg.tenant_id=$1::uuid AND seg.id=$2::uuid
ON CONFLICT (tenant_id,answer_segment_id,source) WHERE status IN ('pending','assigned','in_progress','returned') AND deleted_at IS NULL DO NOTHING`,
		tenantID, segmentID, source, reason, scoringRunID, actorID)
	return err
}

// loadContextForAnswerTx binds automatic grading to the answer row inserted by
// this OMR callback, rather than whichever answer happens to be latest.
func (s *PostgresStore) loadContextForAnswerTx(ctx context.Context, tx *sql.Tx, tenantID, segmentID, answerID string) (Context, error) {
	row := tx.QueryRowContext(ctx, `
SELECT
  seg.id::text,
  q.id::text,q.tenant_id::text,q.exam_id::text,q.question_no,q.question_type,q.score::float8,
  COALESCE(ak.id::text,''),COALESCE(ak.answer_version,''),
  COALESCE(ak.standard_answer,'null'::jsonb),COALESCE(ak.equivalent_answers,'[]'::jsonb),
  COALESCE(sr.config,ak.tolerance,'{}'::jsonb),
  ans.id::text,ans.answer_text,ans.answer_payload,ans.source,ans.confidence::float8,ans.recorded_by::text,ans.created_at
FROM answer_segment seg
JOIN question q ON q.tenant_id=seg.tenant_id AND q.id=seg.question_id
LEFT JOIN LATERAL (
  SELECT id,answer_version,standard_answer,equivalent_answers,tolerance
  FROM question_answer_key
  WHERE tenant_id=q.tenant_id AND question_id=q.id AND deleted_at IS NULL
  ORDER BY created_at DESC LIMIT 1
) ak ON true
LEFT JOIN LATERAL (
  SELECT config FROM scoring_rule
  WHERE tenant_id=q.tenant_id AND question_id=q.id AND status='published' AND deleted_at IS NULL
  ORDER BY version DESC LIMIT 1
) sr ON true
JOIN answer_segment_answer ans ON ans.tenant_id=seg.tenant_id AND ans.answer_segment_id=seg.id AND ans.id=$3::uuid AND ans.deleted_at IS NULL
WHERE seg.tenant_id=$1::uuid AND seg.id=$2::uuid AND seg.deleted_at IS NULL
FOR UPDATE OF seg,ans`, tenantID, segmentID, answerID)

	var segmentIDOut, keyID, keyVersion string
	var question paper.Question
	var standardRaw, equivalentRaw, toleranceRaw, payloadRaw []byte
	var answer SegmentAnswer
	var confidence sql.NullFloat64
	if err := row.Scan(
		&segmentIDOut,
		&question.ID, &question.TenantID, &question.ExamID, &question.QuestionNo, &question.QuestionType, &question.Score,
		&keyID, &keyVersion, &standardRaw, &equivalentRaw, &toleranceRaw,
		&answer.ID, &answer.AnswerText, &payloadRaw, &answer.Source, &confidence, &answer.RecordedBy, &answer.CreatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Context{}, ErrNotFound
		}
		return Context{}, err
	}
	if keyID == "" {
		return Context{}, ErrAnswerKeyMissing
	}
	answerKey := paper.AnswerKey{ID: keyID, QuestionID: question.ID, AnswerVersion: keyVersion}
	_ = json.Unmarshal(standardRaw, &answerKey.StandardAnswer)
	_ = json.Unmarshal(equivalentRaw, &answerKey.EquivalentAnswers)
	_ = json.Unmarshal(toleranceRaw, &answerKey.Tolerance)
	question.AnswerKey = &answerKey
	answer.TenantID = tenantID
	answer.AnswerSegmentID = segmentIDOut
	answer.AnswerPayload = map[string]any{}
	_ = json.Unmarshal(payloadRaw, &answer.AnswerPayload)
	if confidence.Valid {
		value := confidence.Float64
		answer.Confidence = &value
	}
	answer.CreatedAt = answer.CreatedAt.UTC()
	return Context{SegmentID: segmentIDOut, Question: question, AnswerKey: answerKey, Answer: answer}, nil
}

// confirmRuleGradeForCandidateTx is the transaction-scoped counterpart of
// ConfirmRuleGrade. Its candidate ID is explicit so a concurrent or older
// answer can never be graded by accident.
func (s *PostgresStore) confirmRuleGradeForCandidateTx(ctx context.Context, tx *sql.Tx, tenantID, segmentID, candidateID, actorID string, grade Grade) (QuestionGrade, error) {
	if grade.AnswerSegmentID != segmentID || grade.NeedsHumanReview || !grade.AutoPass || grade.SuggestedScore < 0 || grade.SuggestedScore > grade.MaxScore {
		return QuestionGrade{}, ErrInvalidInput
	}
	var examID, submissionID, questionID, ruleID, scoringRunID string
	err := tx.QueryRowContext(ctx, `
SELECT sub.exam_id::text,seg.submission_id::text,seg.question_id::text,sr.id::text,COALESCE(ac.scoring_run_id::text,'')
FROM answer_segment seg
JOIN submission sub ON sub.tenant_id=seg.tenant_id AND sub.id=seg.submission_id AND sub.deleted_at IS NULL
JOIN answer_candidate ac ON ac.tenant_id=seg.tenant_id AND ac.answer_segment_id=seg.id AND ac.id=$3::uuid AND ac.is_current AND ac.decision='confirmed' AND ac.deleted_at IS NULL
JOIN scoring_rule sr ON sr.tenant_id=seg.tenant_id AND sr.question_id=seg.question_id AND sr.status='published' AND sr.deleted_at IS NULL
WHERE seg.tenant_id=$1::uuid AND seg.id=$2::uuid AND seg.deleted_at IS NULL
FOR UPDATE OF seg,ac`, tenantID, segmentID, candidateID).Scan(&examID, &submissionID, &questionID, &ruleID, &scoringRunID)
	if errors.Is(err, sql.ErrNoRows) {
		return QuestionGrade{}, ErrNotFound
	}
	if err != nil {
		return QuestionGrade{}, err
	}
	var existing QuestionGrade
	var existingEvidence []byte
	err = tx.QueryRowContext(ctx, `SELECT id::text,answer_segment_id::text,question_id::text,score::float8,max_score::float8,source,version,evidence,created_at FROM question_grade WHERE tenant_id=$1::uuid AND answer_segment_id=$2::uuid AND answer_candidate_id=$3::uuid AND scoring_rule_id=$4::uuid AND is_current AND deleted_at IS NULL`, tenantID, segmentID, candidateID, ruleID).Scan(&existing.ID, &existing.AnswerSegmentID, &existing.QuestionID, &existing.Score, &existing.MaxScore, &existing.Source, &existing.Version, &existingEvidence, &existing.CreatedAt)
	if err == nil {
		_ = json.Unmarshal(existingEvidence, &existing.Evidence)
		return existing, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return QuestionGrade{}, err
	}
	evidence, err := json.Marshal(map[string]any{"grader_type": grade.GraderType, "rule_version": grade.RuleVersion, "confidence": grade.Confidence, "matched_points": grade.MatchedPoints, "missing_points": grade.MissingPoints, "evidence": grade.Evidence, "risk_flags": grade.RiskFlags})
	if err != nil {
		return QuestionGrade{}, err
	}
	var priorID string
	_ = tx.QueryRowContext(ctx, `SELECT id::text FROM question_grade WHERE tenant_id=$1::uuid AND answer_segment_id=$2::uuid AND is_current AND deleted_at IS NULL FOR UPDATE`, tenantID, segmentID).Scan(&priorID)
	if priorID != "" {
		if _, err = tx.ExecContext(ctx, `UPDATE question_grade SET is_current=false,status='superseded' WHERE tenant_id=$1::uuid AND id=$2::uuid`, tenantID, priorID); err != nil {
			return QuestionGrade{}, err
		}
	}
	row := tx.QueryRowContext(ctx, `INSERT INTO question_grade(tenant_id,exam_id,submission_id,question_id,answer_segment_id,scoring_run_id,answer_candidate_id,scoring_rule_id,source,status,score,max_score,evidence,version,supersedes_id,is_current,confirmed_by)
VALUES($1::uuid,$2::uuid,$3::uuid,$4::uuid,$5::uuid,NULLIF($6,'')::uuid,$7::uuid,$8::uuid,'rule_confirmed','confirmed',$9,$10,$11::jsonb,(SELECT COALESCE(MAX(version),0)+1 FROM question_grade WHERE tenant_id=$1::uuid AND answer_segment_id=$5::uuid),NULLIF($12,'')::uuid,true,$13::uuid)
RETURNING id::text,answer_segment_id::text,question_id::text,score::float8,max_score::float8,source,version,evidence,created_at`, tenantID, examID, submissionID, questionID, segmentID, scoringRunID, candidateID, ruleID, grade.SuggestedScore, grade.MaxScore, evidence, priorID, actorID)
	var out QuestionGrade
	var evidenceRaw []byte
	if err = row.Scan(&out.ID, &out.AnswerSegmentID, &out.QuestionID, &out.Score, &out.MaxScore, &out.Source, &out.Version, &evidenceRaw, &out.CreatedAt); err != nil {
		return QuestionGrade{}, err
	}
	_ = json.Unmarshal(evidenceRaw, &out.Evidence)
	return out, nil
}

func (s *PostgresStore) ApplyOMRFailure(ctx context.Context, tenantID, id, errorCode string, detail map[string]any, retryable bool) (OMRRun, error) {
	status := "terminal_error"
	if retryable {
		status = "retryable_error"
	}
	raw, _ := json.Marshal(detail)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return OMRRun{}, err
	}
	defer tx.Rollback()
	var runID string
	err = tx.QueryRowContext(ctx, `
UPDATE omr_run o
SET status=$3,error_code=$4,error_detail=$5::jsonb,updated_at=now(),
  completed_at=CASE WHEN $3='terminal_error' THEN now() ELSE NULL END
FROM scoring_run sr
WHERE o.tenant_id=$1::uuid AND o.id=$2::uuid AND o.status IN ('queued','processing','retryable_error')
  AND sr.tenant_id=o.tenant_id AND sr.id=o.scoring_run_id AND sr.status='processing'
RETURNING o.scoring_run_id::text`, tenantID, id, status, errorCode, raw).Scan(&runID)
	if errors.Is(err, sql.ErrNoRows) {
		return OMRRun{}, ErrRevisionConflict
	}
	if err != nil {
		return OMRRun{}, err
	}
	if _, err = s.refreshScoringRunTx(ctx, tx, tenantID, runID); err != nil {
		return OMRRun{}, err
	}
	if err = tx.Commit(); err != nil {
		return OMRRun{}, err
	}
	return s.GetOMRRun(ctx, tenantID, id)
}

func (s *PostgresStore) ConfirmRuleGrade(ctx context.Context, tenantID, segmentID, actorID string, grade Grade) (QuestionGrade, error) {
	if grade.AnswerSegmentID != segmentID || grade.NeedsHumanReview || !grade.AutoPass || grade.SuggestedScore < 0 || grade.SuggestedScore > grade.MaxScore {
		return QuestionGrade{}, ErrInvalidInput
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return QuestionGrade{}, err
	}
	defer tx.Rollback()
	var examID, submissionID, questionID, candidateID, ruleID, runID string
	err = tx.QueryRowContext(ctx, `
SELECT sub.exam_id::text, seg.submission_id::text, seg.question_id::text,
  ac.id::text, sr.id::text, COALESCE(ac.scoring_run_id::text,'')
FROM answer_segment seg
JOIN submission sub ON sub.tenant_id=seg.tenant_id AND sub.id=seg.submission_id AND sub.deleted_at IS NULL
JOIN answer_candidate ac ON ac.tenant_id=seg.tenant_id AND ac.answer_segment_id=seg.id AND ac.is_current AND ac.decision='confirmed' AND ac.deleted_at IS NULL
JOIN scoring_rule sr ON sr.tenant_id=seg.tenant_id AND sr.question_id=seg.question_id AND sr.status='published' AND sr.deleted_at IS NULL
WHERE seg.tenant_id=$1::uuid AND seg.id=$2::uuid AND seg.deleted_at IS NULL
FOR UPDATE OF seg`, tenantID, segmentID).Scan(&examID, &submissionID, &questionID, &candidateID, &ruleID, &runID)
	if errors.Is(err, sql.ErrNoRows) {
		return QuestionGrade{}, ErrNotFound
	}
	if err != nil {
		return QuestionGrade{}, err
	}
	var existing QuestionGrade
	var existingEvidence []byte
	err = tx.QueryRowContext(ctx, `SELECT id::text,answer_segment_id::text,question_id::text,score::float8,max_score::float8,source,version,evidence,created_at FROM question_grade WHERE tenant_id=$1::uuid AND answer_segment_id=$2::uuid AND answer_candidate_id=$3::uuid AND scoring_rule_id=$4::uuid AND is_current AND deleted_at IS NULL`, tenantID, segmentID, candidateID, ruleID).Scan(&existing.ID, &existing.AnswerSegmentID, &existing.QuestionID, &existing.Score, &existing.MaxScore, &existing.Source, &existing.Version, &existingEvidence, &existing.CreatedAt)
	if err == nil {
		_ = json.Unmarshal(existingEvidence, &existing.Evidence)
		return existing, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return QuestionGrade{}, err
	}
	evidenceMap := map[string]any{"grader_type": grade.GraderType, "rule_version": grade.RuleVersion, "confidence": grade.Confidence, "matched_points": grade.MatchedPoints, "missing_points": grade.MissingPoints, "evidence": grade.Evidence, "risk_flags": grade.RiskFlags}
	evidence, _ := json.Marshal(evidenceMap)
	var priorID string
	_ = tx.QueryRowContext(ctx, `SELECT id::text FROM question_grade WHERE tenant_id=$1::uuid AND answer_segment_id=$2::uuid AND is_current AND deleted_at IS NULL FOR UPDATE`, tenantID, segmentID).Scan(&priorID)
	if priorID != "" {
		if _, err = tx.ExecContext(ctx, `UPDATE question_grade SET is_current=false,status='superseded' WHERE tenant_id=$1::uuid AND id=$2::uuid`, tenantID, priorID); err != nil {
			return QuestionGrade{}, err
		}
	}
	row := tx.QueryRowContext(ctx, `INSERT INTO question_grade(tenant_id,exam_id,submission_id,question_id,answer_segment_id,scoring_run_id,answer_candidate_id,scoring_rule_id,source,status,score,max_score,evidence,version,supersedes_id,is_current,confirmed_by)
VALUES($1::uuid,$2::uuid,$3::uuid,$4::uuid,$5::uuid,NULLIF($6,'')::uuid,$7::uuid,$8::uuid,'rule_confirmed','confirmed',$9,$10,$11::jsonb,(SELECT COALESCE(MAX(version),0)+1 FROM question_grade WHERE tenant_id=$1::uuid AND answer_segment_id=$5::uuid),NULLIF($12,'')::uuid,true,$13::uuid)
RETURNING id::text,answer_segment_id::text,question_id::text,score::float8,max_score::float8,source,version,evidence,created_at`, tenantID, examID, submissionID, questionID, segmentID, runID, candidateID, ruleID, grade.SuggestedScore, grade.MaxScore, evidence, priorID, actorID)
	var out QuestionGrade
	var evidenceRaw []byte
	if err = row.Scan(&out.ID, &out.AnswerSegmentID, &out.QuestionID, &out.Score, &out.MaxScore, &out.Source, &out.Version, &evidenceRaw, &out.CreatedAt); err != nil {
		return out, err
	}
	_ = json.Unmarshal(evidenceRaw, &out.Evidence)
	if runID != "" {
		if _, err = tx.ExecContext(ctx, `UPDATE scoring_run SET auto_confirmed_count=auto_confirmed_count+1,status=CASE WHEN queued_count=0 AND failed_count=0 AND review_count=0 THEN 'completed' WHEN queued_count=0 AND review_count>0 THEN 'needs_review' ELSE status END,completed_at=CASE WHEN queued_count=0 AND review_count=0 AND failed_count=0 THEN now() ELSE completed_at END,updated_at=now() WHERE tenant_id=$1::uuid AND id=$2::uuid`, tenantID, runID); err != nil {
			return out, err
		}
	}
	return out, tx.Commit()
}

func (s *PostgresStore) ProcessRuleCandidates(ctx context.Context, tenantID, runID, actorID string, engine *Engine) error {
	rows, err := s.db.QueryContext(ctx, `SELECT ac.answer_segment_id::text FROM answer_candidate ac WHERE ac.tenant_id=$1::uuid AND ac.scoring_run_id=$2::uuid AND ac.is_current AND ac.decision='confirmed' AND ac.source IN ('ocr','manual','imported') AND ac.deleted_at IS NULL AND NOT EXISTS(SELECT 1 FROM question_grade g WHERE g.tenant_id=ac.tenant_id AND g.answer_segment_id=ac.answer_segment_id AND g.answer_candidate_id=ac.id AND g.is_current AND g.deleted_at IS NULL) ORDER BY ac.created_at`, tenantID, runID)
	if err != nil {
		return err
	}
	segmentIDs := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		segmentIDs = append(segmentIDs, id)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for _, segmentID := range segmentIDs {
		contextValue, loadErr := s.LoadContext(ctx, tenantID, segmentID)
		if loadErr != nil {
			return loadErr
		}
		grade, gradeErr := engine.Grade(contextValue)
		if gradeErr != nil {
			return gradeErr
		}
		if grade.AutoPass && !grade.NeedsHumanReview {
			if _, confirmErr := s.ConfirmRuleGrade(ctx, tenantID, segmentID, actorID, grade); confirmErr != nil {
				return confirmErr
			}
			continue
		}
		result, insertErr := s.db.ExecContext(ctx, `INSERT INTO review_task(tenant_id,exam_id,question_id,question_no,answer_segment_id,submission_id,anonymous_code,source,status,priority,grade_round,reason_code,scoring_run_id,created_by) SELECT seg.tenant_id,sub.exam_id,q.id,q.question_no,seg.id,sub.id,COALESCE(NULLIF(sub.candidate_no,''),seg.id::text),'rule_review_required','pending',60,'single','rule_not_auto_confirmed',$3::uuid,$4::uuid FROM answer_segment seg JOIN submission sub ON sub.tenant_id=seg.tenant_id AND sub.id=seg.submission_id JOIN question q ON q.tenant_id=seg.tenant_id AND q.id=seg.question_id WHERE seg.tenant_id=$1::uuid AND seg.id=$2::uuid ON CONFLICT (tenant_id,answer_segment_id,source) WHERE status IN ('pending','assigned','in_progress','returned') AND deleted_at IS NULL DO NOTHING`, tenantID, segmentID, actorID, runID)
		if insertErr != nil {
			return insertErr
		}
		count, _ := result.RowsAffected()
		if count == 1 {
			if _, err = s.db.ExecContext(ctx, `UPDATE scoring_run SET review_count=review_count+1,updated_at=now() WHERE tenant_id=$1::uuid AND id=$2::uuid`, tenantID, runID); err != nil {
				return err
			}
		}
	}
	_, err = s.RefreshScoringRun(ctx, tenantID, runID)
	return err
}
