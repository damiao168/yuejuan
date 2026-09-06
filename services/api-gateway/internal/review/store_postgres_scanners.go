package review

import (
	"database/sql"
	"errors"
	"strings"
)

type policyScanner interface {
	Scan(dest ...any) error
}

func scanPolicy(row policyScanner) (DoubleMarkPolicy, error) {
	var out DoubleMarkPolicy
	if err := row.Scan(
		&out.ID,
		&out.TenantID,
		&out.ExamID,
		&out.QuestionID,
		&out.Enabled,
		&out.Threshold,
		&out.ResolutionStrategy,
		&out.AllowSameArbitrator,
		&out.CreatedBy,
		&out.CreatedAt,
		&out.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return DoubleMarkPolicy{}, ErrNotFound
		}
		return DoubleMarkPolicy{}, err
	}
	out.CreatedAt = out.CreatedAt.UTC()
	out.UpdatedAt = out.UpdatedAt.UTC()
	return out, nil
}

type sessionScanner interface {
	Scan(dest ...any) error
}

func scanSession(row sessionScanner) (DoubleMarkSession, error) {
	var out DoubleMarkSession
	var diff sql.NullFloat64
	if err := row.Scan(
		&out.ID,
		&out.TenantID,
		&out.ExamID,
		&out.QuestionID,
		&out.QuestionNo,
		&out.AnswerSegmentID,
		&out.SubmissionID,
		&out.AnonymousCode,
		&out.FirstReviewTaskID,
		&out.SecondReviewTaskID,
		&out.FirstReviewerID,
		&out.SecondReviewerID,
		&out.Threshold,
		&out.ResolutionStrategy,
		&out.Status,
		&diff,
		&out.FinalGradeID,
		&out.ArbitrationTaskID,
		&out.CreatedBy,
		&out.CreatedAt,
		&out.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return DoubleMarkSession{}, ErrNotFound
		}
		return DoubleMarkSession{}, err
	}
	if diff.Valid {
		out.ScoreDifference = floatPtr(diff.Float64)
	}
	out.CreatedAt = out.CreatedAt.UTC()
	out.UpdatedAt = out.UpdatedAt.UTC()
	return out, nil
}

func arbitrationSelect() string {
	return `
SELECT id::text, tenant_id::text, double_mark_session_id::text, exam_id::text, question_id::text, question_no,
  answer_segment_id::text, submission_id::text, anonymous_code,
  first_reviewer_id::text, second_reviewer_id::text, first_score::float8, second_score::float8,
  score_difference::float8, difference_reason, status, COALESCE(assigned_to::text, ''),
  final_score::float8, reason, student_feedback, allow_same_arbitrator, context, revision, created_by::text, created_at, updated_at
FROM arbitration_task
`
}

type arbitrationScanner interface {
	Scan(dest ...any) error
}

func scanArbitration(row arbitrationScanner) (ArbitrationTask, error) {
	var out ArbitrationTask
	var finalScore sql.NullFloat64
	var contextRaw []byte
	if err := row.Scan(
		&out.ID,
		&out.TenantID,
		&out.DoubleMarkSessionID,
		&out.ExamID,
		&out.QuestionID,
		&out.QuestionNo,
		&out.AnswerSegmentID,
		&out.SubmissionID,
		&out.AnonymousCode,
		&out.FirstReviewerID,
		&out.SecondReviewerID,
		&out.FirstScore,
		&out.SecondScore,
		&out.ScoreDifference,
		&out.DifferenceReason,
		&out.Status,
		&out.AssignedTo,
		&finalScore,
		&out.Reason,
		&out.StudentFeedback,
		&out.AllowSameArbitrator,
		&contextRaw,
		&out.Revision,
		&out.CreatedBy,
		&out.CreatedAt,
		&out.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ArbitrationTask{}, ErrNotFound
		}
		return ArbitrationTask{}, err
	}
	if finalScore.Valid {
		out.FinalScore = floatPtr(finalScore.Float64)
	}
	if err := decodeJSONB(contextRaw, &out.Context, "arbitration_task.context"); err != nil {
		return ArbitrationTask{}, err
	}
	out.CreatedAt = out.CreatedAt.UTC()
	out.UpdatedAt = out.UpdatedAt.UTC()
	return out, nil
}

type finalGradeScanner interface {
	Scan(dest ...any) error
}

func scanFinalGrade(row finalGradeScanner) (FinalGrade, error) {
	var out FinalGrade
	if err := row.Scan(
		&out.ID,
		&out.TenantID,
		&out.ExamID,
		&out.QuestionID,
		&out.QuestionNo,
		&out.AnswerSegmentID,
		&out.SubmissionID,
		&out.AnonymousCode,
		&out.Score,
		&out.MaxScore,
		&out.Source,
		&out.DoubleMarkSessionID,
		&out.ArbitrationTaskID,
		&out.ResolutionStrategy,
		&out.Locked,
		&out.CreatedBy,
		&out.CreatedAt,
		&out.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return FinalGrade{}, ErrNotFound
		}
		return FinalGrade{}, err
	}
	out.CreatedAt = out.CreatedAt.UTC()
	out.UpdatedAt = out.UpdatedAt.UTC()
	return out, nil
}

func stringsTrim(value string) string {
	return strings.TrimSpace(value)
}
