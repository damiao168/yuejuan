package subjective

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"edugrade-enterprise/services/api-gateway/internal/paper"
)

type PostgresStore struct {
	db *sql.DB
}

func NewPostgresStore(db *sql.DB) *PostgresStore {
	return &PostgresStore{db: db}
}

func (s *PostgresStore) LoadContext(ctx context.Context, tenantID string, segmentID string) (Context, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT
  seg.id::text, seg.submission_page_id::text, seg.bbox,
  e.subject, COALESCE(cohort.grade_level, ''),
  q.id::text, q.tenant_id::text, q.exam_id::text, COALESCE(q.exam_paper_id::text, ''),
  q.question_no, q.question_type, q.score::float8, COALESCE(q.stem, ''),
  q.knowledge_points, q.answer_area, q.sort_order, q.status,
  COALESCE(qr.id::text, ''), COALESCE(qr.question_id::text, ''), COALESCE(rv.version, ''),
  COALESCE(qr.status, ''), COALESCE(qr.max_score::float8, 0), COALESCE(qr.points, '[]'::jsonb),
  COALESCE(qr.deductions, '[]'::jsonb), COALESCE(qr.examples, '[]'::jsonb),
  COALESCE(ans.id::text, ''), COALESCE(ans.answer_text, ''), ans.confidence::float8, ans.created_at
FROM answer_segment seg
JOIN question q ON q.tenant_id = seg.tenant_id AND q.id = seg.question_id
JOIN exam e ON e.tenant_id = q.tenant_id AND e.id = q.exam_id AND e.deleted_at IS NULL
LEFT JOIN LATERAL (
  SELECT CASE
    WHEN COUNT(DISTINCT g.level_no) = 1 AND MIN(g.level_no) BETWEEN 7 AND 9 THEN 'junior_middle'
    WHEN COUNT(DISTINCT g.level_no) = 1 AND MIN(g.level_no) BETWEEN 10 AND 12 THEN 'senior_middle'
    ELSE ''
  END AS grade_level
  FROM exam_class ec
  JOIN school_class sc ON sc.tenant_id = ec.tenant_id AND sc.id = ec.class_id AND sc.deleted_at IS NULL
  JOIN grade g ON g.tenant_id = sc.tenant_id AND g.id = sc.grade_id AND g.deleted_at IS NULL
  WHERE ec.tenant_id = q.tenant_id AND ec.exam_id = q.exam_id AND ec.deleted_at IS NULL
) cohort ON true
LEFT JOIN LATERAL (
  SELECT qr.id, qr.question_id, qr.status, qr.max_score, qr.points, qr.deductions, qr.examples, qr.rubric_version_id
  FROM question_rubric qr
  WHERE qr.tenant_id = q.tenant_id AND qr.question_id = q.id AND qr.deleted_at IS NULL
  ORDER BY qr.created_at DESC
  LIMIT 1
) qr ON true
LEFT JOIN rubric_version rv ON rv.tenant_id = q.tenant_id AND rv.id = qr.rubric_version_id
LEFT JOIN LATERAL (
  SELECT id, answer_text, confidence, created_at
  FROM answer_segment_answer
  WHERE tenant_id = seg.tenant_id AND answer_segment_id = seg.id AND deleted_at IS NULL
  ORDER BY created_at DESC
  LIMIT 1
) ans ON true
WHERE seg.tenant_id = $1 AND seg.id::text = $2 AND seg.deleted_at IS NULL
`, tenantID, segmentID)
	var out Context
	var submissionPageID string
	var bboxRaw, kpRaw, areaRaw []byte
	var question paper.Question
	var rubric paper.Rubric
	var pointsRaw, deductionsRaw, examplesRaw []byte
	var answerID string
	var ocrConfidence sql.NullFloat64
	var answerCreated sql.NullTime
	if err := row.Scan(
		&out.SegmentID,
		&submissionPageID,
		&bboxRaw,
		&out.Subject,
		&out.GradeLevel,
		&question.ID,
		&question.TenantID,
		&question.ExamID,
		&question.ExamPaperID,
		&question.QuestionNo,
		&question.QuestionType,
		&question.Score,
		&question.Stem,
		&kpRaw,
		&areaRaw,
		&question.SortOrder,
		&question.Status,
		&rubric.ID,
		&rubric.QuestionID,
		&rubric.Version,
		&rubric.Status,
		&rubric.MaxScore,
		&pointsRaw,
		&deductionsRaw,
		&examplesRaw,
		&answerID,
		&out.AnswerText,
		&ocrConfidence,
		&answerCreated,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Context{}, ErrNotFound
		}
		return Context{}, err
	}
	if !IsSupportedQuestionType(question.QuestionType) {
		return Context{}, ErrUnsupportedQuestionType
	}
	_ = json.Unmarshal(kpRaw, &question.KnowledgePoints)
	if len(areaRaw) > 0 {
		_ = json.Unmarshal(areaRaw, &question.AnswerArea)
	}
	if rubric.ID == "" {
		return Context{}, ErrRubricMissing
	}
	_ = json.Unmarshal(pointsRaw, &rubric.Points)
	_ = json.Unmarshal(deductionsRaw, &rubric.Deductions)
	_ = json.Unmarshal(examplesRaw, &rubric.Examples)
	if answerID == "" {
		return Context{}, ErrAnswerMissing
	}
	out.AnswerVersion = answerID
	var bbox []float64
	_ = json.Unmarshal(bboxRaw, &bbox)
	out.Question = question
	out.Rubric = rubric
	out.AnswerImageRef = map[string]any{"submission_page_id": submissionPageID, "bbox": bbox}
	if ocrConfidence.Valid {
		value := ocrConfidence.Float64
		out.OCRConfidence = &value
	}
	if answerCreated.Valid {
		out.AnswerCreatedAt = answerCreated.Time.UTC()
	}
	return out, nil
}

func (s *PostgresStore) CreateGrade(ctx context.Context, tenantID string, actorID string, grade Grade) (Grade, error) {
	matched, _ := json.Marshal(grade.MatchedPoints)
	missing, _ := json.Marshal(grade.MissingPoints)
	evidence, _ := json.Marshal(grade.Evidence)
	risks, _ := json.Marshal(grade.RiskFlags)
	raw, _ := json.Marshal(cloneMap(grade.RawOutput))
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Grade{}, err
	}
	defer tx.Rollback()
	row := tx.QueryRowContext(ctx, `
INSERT INTO ai_grade (
  tenant_id, answer_segment_id, question_id, question_no, question_type, answer_version,
  grader_type, rule_version, suggested_score, max_score, confidence,
  matched_points, missing_points, evidence, risk_flags, needs_human_review,
  auto_pass, mock, raw_output, created_by, status, failure_reason,
  model_version, prompt_version, student_feedback, teacher_note,
  rubric_version, delivery_mode, capability_profile, adapter_request_id,
  adapter_name, provider_key, deployment_key, deployment_region,
  adapter_attempts, adapter_latency_ms, adapter_repair_attempted
)
VALUES ($1, $2, $3, $4, $5, $6, $7, 'llm-adapter', $8, $9, $10,
  $11, $12, $13, $14, $15, false, $16, $17, $18, $19, NULLIF($20, ''),
  $21, $22, $23, $24, $25, $26, $27, $28, $29, $30, $31, $32, $33, $34, $35)
RETURNING id::text, tenant_id::text, answer_segment_id::text, question_id::text, question_no, question_type,
  answer_version, grader_type, model_version, prompt_version, rubric_version, delivery_mode, capability_profile,
  adapter_request_id, adapter_name, provider_key, deployment_key, deployment_region,
  adapter_attempts, adapter_latency_ms, adapter_repair_attempted,
  suggested_score::float8, max_score::float8, confidence::float8,
  matched_points, missing_points, evidence, risk_flags, needs_human_review, student_feedback, teacher_note,
  mock, status, COALESCE(failure_reason, ''), raw_output, created_by::text, created_at
`, tenantID, grade.AnswerSegmentID, grade.QuestionID, grade.QuestionNo, grade.QuestionType,
		grade.AnswerVersion, grade.GraderType, grade.SuggestedScore, grade.MaxScore, grade.Confidence,
		matched, missing, evidence, risks, grade.NeedsHumanReview, grade.Mock, raw, actorID,
		grade.Status, grade.FailureReason, grade.ModelVersion, grade.PromptVersion, grade.StudentFeedback, grade.TeacherNote,
		grade.RubricVersion, grade.DeliveryMode, grade.CapabilityProfile, grade.AdapterRequestID,
		grade.AdapterName, grade.ProviderKey, grade.DeploymentKey, grade.DeploymentRegion,
		grade.AdapterAttempts, grade.AdapterLatencyMS, grade.AdapterRepairAttempted)
	var out Grade
	if err := scanGrade(row, &out); err != nil {
		return Grade{}, err
	}
	if !out.Mock &&
		out.AdapterRequestID != "" &&
		out.AdapterName != "" &&
		out.ProviderKey != "" &&
		out.DeploymentKey != "" &&
		out.DeploymentRegion != "" {
		if _, err := tx.ExecContext(ctx, `
INSERT INTO model_call_fact (
  tenant_id, request_id, answer_segment_id, question_id,
  provider_key, deployment_key, adapter_type, model_version,
  prompt_version, rubric_version, capability_profile, deployment_region,
  route_mode, route_reason, status, attempts, latency_ms, error_code
)
VALUES (
  $1, $2, $3, $4,
  $5, $6, $7, $8,
  $9, $10, $11, $12,
  'local_only', 'configured governed grading-agent deployment', $13, $14, $15, $16
)
`, tenantID, out.AdapterRequestID, out.AnswerSegmentID, out.QuestionID,
			out.ProviderKey, out.DeploymentKey, out.AdapterName, out.ModelVersion,
			out.PromptVersion, out.RubricVersion, out.CapabilityProfile, out.DeploymentRegion,
			out.Status, out.AdapterAttempts, out.AdapterLatencyMS, out.FailureReason); err != nil {
			return Grade{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return Grade{}, err
	}
	return out, nil
}

type gradeScanner interface {
	Scan(dest ...any) error
}

func scanGrade(row gradeScanner, out *Grade) error {
	var matchedRaw, missingRaw, evidenceRaw, risksRaw, rawOutput []byte
	if err := row.Scan(
		&out.ID,
		&out.TenantID,
		&out.AnswerSegmentID,
		&out.QuestionID,
		&out.QuestionNo,
		&out.QuestionType,
		&out.AnswerVersion,
		&out.GraderType,
		&out.ModelVersion,
		&out.PromptVersion,
		&out.RubricVersion,
		&out.DeliveryMode,
		&out.CapabilityProfile,
		&out.AdapterRequestID,
		&out.AdapterName,
		&out.ProviderKey,
		&out.DeploymentKey,
		&out.DeploymentRegion,
		&out.AdapterAttempts,
		&out.AdapterLatencyMS,
		&out.AdapterRepairAttempted,
		&out.SuggestedScore,
		&out.MaxScore,
		&out.Confidence,
		&matchedRaw,
		&missingRaw,
		&evidenceRaw,
		&risksRaw,
		&out.NeedsHumanReview,
		&out.StudentFeedback,
		&out.TeacherNote,
		&out.Mock,
		&out.Status,
		&out.FailureReason,
		&rawOutput,
		&out.CreatedBy,
		&out.CreatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	_ = json.Unmarshal(matchedRaw, &out.MatchedPoints)
	_ = json.Unmarshal(missingRaw, &out.MissingPoints)
	_ = json.Unmarshal(evidenceRaw, &out.Evidence)
	_ = json.Unmarshal(risksRaw, &out.RiskFlags)
	out.RawOutput = map[string]any{}
	_ = json.Unmarshal(rawOutput, &out.RawOutput)
	out.CreatedAt = out.CreatedAt.UTC()
	return nil
}
