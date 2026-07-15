package review

import (
	"context"
	"database/sql"
	"errors"
)

func (s *PostgresStore) ClaimNextTask(ctx context.Context, tenantID, reviewerID string, input NextTaskInput) (ReviewTask, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ReviewTask{}, err
	}
	defer tx.Rollback()
	var id string
	err = tx.QueryRowContext(ctx, `SELECT id::text FROM review_task WHERE tenant_id=$1::uuid AND assigned_to=$2::uuid AND status IN ('assigned','in_progress','returned') AND deleted_at IS NULL AND ($3='' OR exam_id::text=$3) AND ($4='' OR question_id::text=$4) ORDER BY last_opened_at DESC NULLS LAST,priority DESC,created_at LIMIT 1 FOR UPDATE SKIP LOCKED`, tenantID, reviewerID, input.ExamID, input.QuestionID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		err = tx.QueryRowContext(ctx, `SELECT id::text FROM review_task WHERE tenant_id=$1::uuid AND ((assigned_to IS NULL AND status='pending') OR (status IN ('assigned','in_progress') AND claim_expires_at<now())) AND deleted_at IS NULL AND ($2='' OR exam_id::text=$2) AND ($3='' OR question_id::text=$3) ORDER BY priority DESC,created_at LIMIT 1 FOR UPDATE SKIP LOCKED`, tenantID, input.ExamID, input.QuestionID).Scan(&id)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return ReviewTask{}, ErrNotFound
	}
	if err != nil {
		return ReviewTask{}, err
	}
	_, err = tx.ExecContext(ctx, `UPDATE review_task SET status=CASE WHEN assigned_to IS DISTINCT FROM $3::uuid OR status='pending' THEN 'assigned' ELSE status END,assigned_to=$3::uuid,claimed_at=now(),claim_expires_at=now()+interval '30 minutes',last_opened_at=now(),revision=revision+1,updated_at=now() WHERE tenant_id=$1::uuid AND id=$2::uuid`, tenantID, id, reviewerID)
	if err != nil {
		return ReviewTask{}, err
	}
	task, err := scanTask(tx.QueryRowContext(ctx, `SELECT id::text, tenant_id::text, exam_id::text, question_id::text, question_no,answer_segment_id::text, submission_id::text, anonymous_code, source, status, priority,COALESCE(assigned_to::text, ''), return_reason, grade_round, due_at, created_by::text, created_at, updated_at FROM review_task WHERE tenant_id=$1::uuid AND id=$2::uuid`, tenantID, id))
	if err != nil {
		return task, err
	}
	return task, tx.Commit()
}

func (s *PostgresStore) GetWorkspace(ctx context.Context, tenantID, taskID string) (Workspace, error) {
	task, err := s.GetTask(ctx, tenantID, taskID)
	if err != nil {
		return Workspace{}, err
	}
	contextValue, err := s.loadContext(ctx, tenantID, task.AnswerSegmentID)
	if err != nil {
		return Workspace{}, err
	}
	var originalID, status string
	var confidence sql.NullFloat64
	err = s.db.QueryRowContext(ctx, `SELECT COALESCE(sp.file_asset_id::text,''),seg.processing_status,seg.confidence FROM answer_segment seg LEFT JOIN submission_page sp ON sp.tenant_id=seg.tenant_id AND sp.id=seg.submission_page_id WHERE seg.tenant_id=$1::uuid AND seg.id=$2::uuid AND seg.deleted_at IS NULL`, tenantID, task.AnswerSegmentID).Scan(&originalID, &status, &confidence)
	if err != nil {
		return Workspace{}, err
	}
	out := Workspace{Task: task, Context: contextValue, SegmentImageURL: "/api/v1/answer-segments/" + task.AnswerSegmentID + "/image", SegmentStatus: status}
	if originalID != "" {
		out.OriginalImageURL = "/api/v1/files/" + originalID + "/download"
	}
	if confidence.Valid {
		value := confidence.Float64
		out.SegmentConfidence = &value
	}
	return out, nil
}

func (s *PostgresStore) RenewTaskClaim(ctx context.Context, tenantID, taskID, reviewerID string) error {
	result, err := s.db.ExecContext(ctx, `UPDATE review_task SET claim_expires_at=now()+interval '30 minutes',last_opened_at=now(),updated_at=now() WHERE tenant_id=$1::uuid AND id=$2::uuid AND assigned_to=$3::uuid AND status IN ('assigned','in_progress','returned') AND deleted_at IS NULL`, tenantID, taskID, reviewerID)
	if err != nil {
		return err
	}
	count, _ := result.RowsAffected()
	if count != 1 {
		return ErrForbidden
	}
	return nil
}

func (s *PostgresStore) ReleaseTaskClaim(ctx context.Context, tenantID, taskID, reviewerID string) (ReviewTask, error) {
	task, err := scanTask(s.db.QueryRowContext(ctx, `
UPDATE review_task
SET assigned_to=NULL,status='pending',claimed_at=NULL,claim_expires_at=NULL,last_opened_at=now(),revision=revision+1,updated_at=now()
WHERE tenant_id=$1::uuid AND id=$2::uuid AND assigned_to=$3::uuid
  AND status IN ('assigned','in_progress','returned') AND deleted_at IS NULL
RETURNING id::text,tenant_id::text,exam_id::text,question_id::text,question_no,
  answer_segment_id::text,submission_id::text,anonymous_code,source,status,priority,
  COALESCE(assigned_to::text,''),return_reason,grade_round,due_at,created_by::text,created_at,updated_at
`, tenantID, taskID, reviewerID))
	if errors.Is(err, ErrNotFound) {
		return ReviewTask{}, ErrForbidden
	}
	return task, err
}
