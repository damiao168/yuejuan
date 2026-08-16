package mathunderstanding

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
)

type PostgresCorrectionStore struct {
	db *sql.DB
}

func NewPostgresCorrectionStore(db *sql.DB, _ Store) *PostgresCorrectionStore {
	return &PostgresCorrectionStore{db: db}
}
func (s *PostgresCorrectionStore) CreateCorrection(ctx context.Context, tenantID, artifactID, actorID string, input CreateCorrectionInput) (Correction, error) {
	if tenantID == "" || actorID == "" || validateCorrection(input) != nil {
		return Correction{}, ErrInvalidInput
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Correction{}, err
	}
	defer tx.Rollback()
	var answerSegmentID, snapshotID string
	var version int64
	err = tx.QueryRowContext(ctx, `SELECT answer_segment_id::text,exam_question_snapshot_id::text,version FROM math_understanding_artifact WHERE tenant_id=$1::uuid AND id=$2::uuid FOR UPDATE`, tenantID, artifactID).Scan(&answerSegmentID, &snapshotID, &version)
	if errors.Is(err, sql.ErrNoRows) {
		return Correction{}, ErrNotFound
	}
	if err != nil {
		return Correction{}, err
	}
	if version != input.ExpectedArtifactVersion || input.CorrectedContract.AnswerSegmentID != answerSegmentID || input.CorrectedContract.ExamQuestionSnapshotID != snapshotID {
		return Correction{}, ErrRevisionConflict
	}
	operations, _ := json.Marshal(input.Operations)
	contract, _ := json.Marshal(input.CorrectedContract)
	var item Correction
	var opRaw, contractRaw []byte
	err = tx.QueryRowContext(ctx, `INSERT INTO math_understanding_correction(tenant_id,artifact_id,answer_segment_id,revision,operations_json,corrected_contract_json,reason,created_by) VALUES($1::uuid,$2::uuid,$3::uuid,COALESCE((SELECT max(revision)+1 FROM math_understanding_correction WHERE tenant_id=$1::uuid AND artifact_id=$2::uuid),1),$4::jsonb,$5::jsonb,$6,$7::uuid) RETURNING id::text,tenant_id::text,artifact_id::text,answer_segment_id::text,revision,operations_json,corrected_contract_json,reason,created_by::text,created_at`, tenantID, artifactID, answerSegmentID, operations, contract, input.Reason, actorID).Scan(&item.ID, &item.TenantID, &item.ArtifactID, &item.AnswerSegmentID, &item.Revision, &opRaw, &contractRaw, &item.Reason, &item.CreatedBy, &item.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Correction{}, ErrRevisionConflict
	}
	if err != nil {
		return Correction{}, err
	}
	if json.Unmarshal(opRaw, &item.Operations) != nil || json.Unmarshal(contractRaw, &item.CorrectedContract) != nil {
		return Correction{}, errors.New("decode math correction")
	}
	if err = tx.Commit(); err != nil {
		return Correction{}, err
	}
	return item, nil
}
func (s *PostgresCorrectionStore) ListCorrections(ctx context.Context, tenantID, artifactID string) ([]Correction, error) {
	return s.list(ctx, tenantID, "artifact_id=$2::uuid", artifactID, 500)
}
func (s *PostgresCorrectionStore) ExportCorrections(ctx context.Context, tenantID, subject string, limit int) ([]Correction, error) {
	if limit <= 0 || limit > 5000 {
		limit = 1000
	}
	return s.list(ctx, tenantID, "corrected_contract_json->>'subject_code'=$2", subject, limit)
}
func (s *PostgresCorrectionStore) list(ctx context.Context, tenantID, predicate, value string, limit int) ([]Correction, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id::text,tenant_id::text,artifact_id::text,answer_segment_id::text,revision,operations_json,corrected_contract_json,reason,created_by::text,created_at FROM math_understanding_correction WHERE tenant_id=$1::uuid AND `+predicate+` ORDER BY created_at,id LIMIT $3`, tenantID, value, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Correction{}
	for rows.Next() {
		var item Correction
		var operations, contract []byte
		if err = rows.Scan(&item.ID, &item.TenantID, &item.ArtifactID, &item.AnswerSegmentID, &item.Revision, &operations, &contract, &item.Reason, &item.CreatedBy, &item.CreatedAt); err != nil {
			return nil, err
		}
		if json.Unmarshal(operations, &item.Operations) != nil || json.Unmarshal(contract, &item.CorrectedContract) != nil {
			return nil, errors.New("decode math correction")
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
