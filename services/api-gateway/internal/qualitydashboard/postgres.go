package qualitydashboard

import (
	"context"
	"database/sql"
)

// PostgresQuestionReader intentionally reads only question metadata and the
// frozen assessment classification. It is kept separate from the dashboard
// service so alternate stores can be used in tests or future read replicas.
type PostgresQuestionReader struct{ db *sql.DB }

func NewPostgresQuestionReader(db *sql.DB) *PostgresQuestionReader {
	return &PostgresQuestionReader{db: db}
}

func (r *PostgresQuestionReader) ListQualityQuestions(ctx context.Context, tenantID, examID string) ([]Question, error) {
	rows, err := r.db.QueryContext(ctx, `
WITH latest_snapshot AS (
  SELECT DISTINCT ON (question_id) question_id, archetype_code, risk_tier
  FROM exam_question_snapshot
  WHERE tenant_id=$1::uuid AND exam_id=$2::uuid
  ORDER BY question_id, snapshot_version DESC
)
SELECT q.id::text, q.question_no,
       COALESCE(snapshot.archetype_code, config.archetype_code, ''),
       COALESCE(snapshot.risk_tier, config.risk_tier, ''),
       q.score::float8
FROM question q
LEFT JOIN latest_snapshot snapshot ON snapshot.question_id=q.id
LEFT JOIN question_assessment_config config
  ON config.tenant_id=q.tenant_id AND config.exam_id=q.exam_id AND config.question_id=q.id
WHERE q.tenant_id=$1::uuid AND q.exam_id=$2::uuid AND q.deleted_at IS NULL
ORDER BY q.sort_order, q.question_no, q.id`, tenantID, examID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Question{}
	for rows.Next() {
		var item Question
		if err := rows.Scan(&item.ID, &item.QuestionNo, &item.ArchetypeCode, &item.RiskTier, &item.MaxScore); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

// PostgresCalibrationReader returns only manager-facing aggregate/session
// state. It never reads a Gold sample manifest, attempt score, or rubric
// selection, so this endpoint cannot leak calibration answer material.
type PostgresCalibrationReader struct{ db *sql.DB }

func NewPostgresCalibrationReader(db *sql.DB) *PostgresCalibrationReader {
	return &PostgresCalibrationReader{db: db}
}

func (r *PostgresCalibrationReader) GetCalibrationSummary(ctx context.Context, tenantID, examID, questionID string) (CalibrationSummary, error) {
	result := CalibrationSummary{Graders: []CalibrationGrader{}}
	if err := r.db.QueryRowContext(ctx, `
SELECT EXISTS(
  SELECT 1 FROM grader_calibration_policy
  WHERE tenant_id=$1::uuid AND exam_id=$2::uuid AND question_id=$3::uuid
)`, tenantID, examID, questionID).Scan(&result.Configured); err != nil {
		return CalibrationSummary{}, err
	}
	rows, err := r.db.QueryContext(ctx, `
SELECT grader_id::text, status,
       COALESCE((metrics_json->>'sample_count')::int, 0)
FROM grader_calibration_session
WHERE tenant_id=$1::uuid AND exam_id=$2::uuid AND question_id=$3::uuid
ORDER BY completed_at DESC NULLS LAST, started_at DESC, id DESC`, tenantID, examID, questionID)
	if err != nil {
		return CalibrationSummary{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var item CalibrationGrader
		if err := rows.Scan(&item.GraderRef, &item.Status, &item.SampleSize); err != nil {
			return CalibrationSummary{}, err
		}
		result.Graders = append(result.Graders, item)
		if item.Status == "passed" || item.Status == "failed" {
			result.Completed++
			if item.Status == "passed" {
				result.Passed++
			}
		}
	}
	if err := rows.Err(); err != nil {
		return CalibrationSummary{}, err
	}
	result.PassRate = ratio(result.Passed, result.Completed)
	return result, nil
}
