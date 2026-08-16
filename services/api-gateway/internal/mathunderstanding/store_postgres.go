package mathunderstanding

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
)

type PostgresStore struct{ db *sql.DB }

func NewPostgresStore(db *sql.DB) *PostgresStore { return &PostgresStore{db: db} }

func (s *PostgresStore) CreateArtifact(ctx context.Context, tenantID string, input CreateArtifactInput) (Artifact, error) {
	if tenantID == "" || ValidateCreateArtifact(input) != nil {
		return Artifact{}, ErrInvalidInput
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Artifact{}, err
	}
	defer tx.Rollback()
	var lockedSegmentID string
	if err = tx.QueryRowContext(ctx, `
SELECT seg.id::text
FROM answer_segment seg
JOIN question q ON q.tenant_id=seg.tenant_id AND q.id=seg.question_id AND q.deleted_at IS NULL
JOIN exam_question_snapshot snap ON snap.tenant_id=q.tenant_id AND snap.exam_id=q.exam_id AND snap.question_id=q.id
WHERE seg.tenant_id=$1::uuid AND seg.id=$2::uuid AND snap.id=$3::uuid AND snap.subject_code=$4 AND snap.subject_code IN ('mathematics','physics','chemistry') AND seg.deleted_at IS NULL
	FOR UPDATE OF seg`, tenantID, input.AnswerSegmentID, input.ExamQuestionSnapshotID, input.SubjectCode).Scan(&lockedSegmentID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Artifact{}, ErrInvalidInput
		}
		return Artifact{}, err
	}
	var version int64
	if err = tx.QueryRowContext(ctx, `SELECT COALESCE(max(version),0)+1 FROM math_understanding_artifact WHERE tenant_id=$1::uuid AND answer_segment_id=$2::uuid`, tenantID, input.AnswerSegmentID).Scan(&version); err != nil {
		return Artifact{}, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE math_understanding_artifact SET is_current=false WHERE tenant_id=$1::uuid AND answer_segment_id=$2::uuid AND is_current`, tenantID, input.AnswerSegmentID); err != nil {
		return Artifact{}, err
	}
	blocks, _ := json.Marshal(input.Blocks)
	formulas, _ := json.Marshal(input.Formulas)
	relations, _ := json.Marshal(input.Relations)
	graph, _ := json.Marshal(input.SolutionGraph)
	checks, _ := json.Marshal(input.Verifications)
	evidence, _ := json.Marshal(input.RubricEvidence)
	row := tx.QueryRowContext(ctx, `
INSERT INTO math_understanding_artifact(
 tenant_id,subject_code,answer_segment_id,exam_question_snapshot_id,version,input_hash,engine_version,
 blocks_json,formulas_json,relations_json,solution_graph_json,verifications_json,rubric_evidence_json,is_current)
VALUES($1::uuid,$2,$3::uuid,$4::uuid,$5,$6,$7,$8::jsonb,$9::jsonb,$10::jsonb,$11::jsonb,$12::jsonb,$13::jsonb,true)
RETURNING id::text,created_at`, tenantID, input.SubjectCode, input.AnswerSegmentID, input.ExamQuestionSnapshotID, version, input.InputHash, input.EngineVersion, blocks, formulas, relations, graph, checks, evidence)
	item := Artifact{TenantID: tenantID, AnswerSegmentID: input.AnswerSegmentID, ExamQuestionSnapshotID: input.ExamQuestionSnapshotID, Version: version, InputHash: input.InputHash, EngineVersion: input.EngineVersion, IsCurrent: true, CreateArtifactInput: cloneInput(input)}
	if err = row.Scan(&item.ID, &item.CreatedAt); err != nil {
		return Artifact{}, err
	}
	if err = tx.Commit(); err != nil {
		return Artifact{}, err
	}
	return item, nil
}

func (s *PostgresStore) GetLatestArtifact(ctx context.Context, tenantID string, answerSegmentID string) (Artifact, error) {
	return s.getArtifact(ctx, tenantID, "answer_segment_id=$2::uuid AND is_current", answerSegmentID)
}

func (s *PostgresStore) GetArtifact(ctx context.Context, tenantID string, artifactID string) (Artifact, error) {
	return s.getArtifact(ctx, tenantID, "id=$2::uuid", artifactID)
}

func (s *PostgresStore) getArtifact(ctx context.Context, tenantID, predicate, id string) (Artifact, error) {
	var item Artifact
	var blocks, formulas, relations, graph, checks, evidence []byte
	err := s.db.QueryRowContext(ctx, `
SELECT id::text,tenant_id::text,subject_code,answer_segment_id::text,exam_question_snapshot_id::text,version,input_hash,engine_version,is_current,
 blocks_json,formulas_json,relations_json,solution_graph_json,verifications_json,rubric_evidence_json,created_at
FROM math_understanding_artifact
WHERE tenant_id=$1::uuid AND `+predicate, tenantID, id).Scan(
		&item.ID, &item.TenantID, &item.SubjectCode, &item.AnswerSegmentID, &item.ExamQuestionSnapshotID, &item.Version, &item.InputHash, &item.EngineVersion, &item.IsCurrent,
		&blocks, &formulas, &relations, &graph, &checks, &evidence, &item.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Artifact{}, ErrNotFound
	}
	if err != nil {
		return Artifact{}, err
	}
	item.CreateArtifactInput.AnswerSegmentID = item.AnswerSegmentID
	item.CreateArtifactInput.ExamQuestionSnapshotID = item.ExamQuestionSnapshotID
	item.CreateArtifactInput.InputHash = item.InputHash
	item.CreateArtifactInput.EngineVersion = item.EngineVersion
	if json.Unmarshal(blocks, &item.Blocks) != nil || json.Unmarshal(formulas, &item.Formulas) != nil || json.Unmarshal(relations, &item.Relations) != nil || json.Unmarshal(graph, &item.SolutionGraph) != nil || json.Unmarshal(checks, &item.Verifications) != nil || json.Unmarshal(evidence, &item.RubricEvidence) != nil {
		return Artifact{}, errors.New("decode math understanding artifact")
	}
	return item, nil
}
