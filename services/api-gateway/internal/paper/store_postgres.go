package paper

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
)

type PostgresStore struct {
	db *sql.DB
}

func NewPostgresStore(db *sql.DB) *PostgresStore {
	return &PostgresStore{db: db}
}

func (s *PostgresStore) CreatePaper(ctx context.Context, tenantID string, examID string, userID string, input CreatePaperInput) (Paper, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Paper{}, err
	}
	defer tx.Rollback()
	version := 1
	_ = tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(version_no), 0) + 1 FROM exam_paper WHERE tenant_id = $1 AND exam_id = $2`, tenantID, examID).Scan(&version)
	var paperID string
	if err := tx.QueryRowContext(ctx, `SELECT gen_random_uuid()::text`).Scan(&paperID); err != nil {
		return Paper{}, err
	}
	fileID := input.FileAssetID
	fileInput := input.File
	if fileID != "" {
		var linkedExamID string
		err := tx.QueryRowContext(ctx, `
SELECT id::text, COALESCE(exam_id::text, ''), original_name, content_type, size_bytes, hash_sha256, storage_bucket, storage_key
FROM file_asset
WHERE tenant_id = $1 AND id::text = $2 AND deleted_at IS NULL
`, tenantID, fileID).Scan(&fileID, &linkedExamID, &fileInput.OriginalName, &fileInput.ContentType, &fileInput.SizeBytes, &fileInput.HashSHA256, &fileInput.StorageBucket, &fileInput.StorageKey)
		if errors.Is(err, sql.ErrNoRows) {
			return Paper{}, ErrNotFound
		}
		if err != nil {
			return Paper{}, err
		}
		if linkedExamID != "" && linkedExamID != examID {
			return Paper{}, ErrInvalidInput
		}
		if _, err := tx.ExecContext(ctx, `
UPDATE file_asset
SET exam_id = $3, owner_type = 'exam_paper', owner_id = $4::uuid
WHERE tenant_id = $1 AND id::text = $2 AND deleted_at IS NULL
`, tenantID, fileID, examID, paperID); err != nil {
			return Paper{}, err
		}
	} else {
		if err := tx.QueryRowContext(ctx, `SELECT gen_random_uuid()::text`).Scan(&fileID); err != nil {
			return Paper{}, err
		}
		if _, err := tx.ExecContext(ctx, `
INSERT INTO file_asset (id, tenant_id, exam_id, owner_type, owner_id, original_name, content_type, size_bytes, hash_sha256, storage_bucket, storage_key, visibility, uploaded_by)
VALUES ($1, $2, $3, 'exam_paper', $4, $5, $6, $7, $8, $9, $10, 'private', $11)
`, fileID, tenantID, examID, paperID, fileInput.OriginalName, fileInput.ContentType, fileInput.SizeBytes, fileInput.HashSHA256, fileInput.StorageBucket, fileInput.StorageKey, userID); err != nil {
			return Paper{}, err
		}
	}
	row := tx.QueryRowContext(ctx, `
INSERT INTO exam_paper (id, tenant_id, exam_id, file_asset_id, version_no, status, uploaded_by)
VALUES ($1, $2, $3, $4, $5, 'uploaded', $6)
RETURNING id::text, tenant_id::text, exam_id::text, file_asset_id::text, version_no, status
`, paperID, tenantID, examID, fileID, version, userID)
	var out Paper
	if err := row.Scan(&out.ID, &out.TenantID, &out.ExamID, &out.FileAssetID, &out.VersionNo, &out.Status); err != nil {
		return Paper{}, err
	}
	out.File = fileInput
	if err := tx.Commit(); err != nil {
		return Paper{}, err
	}
	return out, nil
}

func (s *PostgresStore) ListPapers(ctx context.Context, tenantID string, examID string) ([]Paper, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT p.id::text, p.tenant_id::text, p.exam_id::text, p.file_asset_id::text, p.version_no, p.status,
       f.original_name, f.content_type, f.size_bytes, f.hash_sha256, f.storage_bucket, f.storage_key
FROM exam_paper p
JOIN file_asset f ON f.tenant_id = p.tenant_id AND f.id = p.file_asset_id
WHERE p.tenant_id = $1 AND p.exam_id = $2 AND p.deleted_at IS NULL
ORDER BY p.version_no DESC
`, tenantID, examID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Paper{}
	for rows.Next() {
		var item Paper
		if err := rows.Scan(&item.ID, &item.TenantID, &item.ExamID, &item.FileAssetID, &item.VersionNo, &item.Status, &item.File.OriginalName, &item.File.ContentType, &item.File.SizeBytes, &item.File.HashSHA256, &item.File.StorageBucket, &item.File.StorageKey); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *PostgresStore) CreateQuestion(ctx context.Context, tenantID string, examID string, userID string, input CreateQuestionInput) (Question, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Question{}, err
	}
	defer tx.Rollback()
	if err := ensurePaperBelongsToExam(ctx, tx, tenantID, examID, input.ExamPaperID); err != nil {
		return Question{}, err
	}
	kp, _ := json.Marshal(input.KnowledgePoints)
	area, _ := json.Marshal(input.AnswerArea)
	sortOrder := input.SortOrder
	if sortOrder == 0 {
		_ = tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(sort_order), 0) + 1 FROM question WHERE tenant_id = $1 AND exam_id = $2`, tenantID, examID).Scan(&sortOrder)
	}
	row := tx.QueryRowContext(ctx, `
INSERT INTO question (tenant_id, exam_id, exam_paper_id, question_no, question_type, score, stem, knowledge_points, answer_area, sort_order, status)
VALUES ($1, $2, NULLIF($3, '')::uuid, $4, $5, $6, $7, $8, $9, $10, 'active')
RETURNING id::text, tenant_id::text, exam_id::text, COALESCE(exam_paper_id::text, ''), question_no, question_type, score::float8, COALESCE(stem, ''), knowledge_points, answer_area, sort_order, status
`, tenantID, examID, input.ExamPaperID, input.QuestionNo, input.QuestionType, input.Score, input.Stem, kp, area, sortOrder)
	var out Question
	if err := scanQuestion(row, &out); err != nil {
		return Question{}, err
	}
	if input.AnswerKey != nil {
		key, err := s.insertAnswerKey(ctx, tx, tenantID, out.ID, userID, "v1", *input.AnswerKey)
		if err != nil {
			return Question{}, err
		}
		out.AnswerKey = &key
	}
	if err := tx.Commit(); err != nil {
		return Question{}, err
	}
	return out, nil
}

func (s *PostgresStore) ListQuestions(ctx context.Context, tenantID string, examID string) ([]Question, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id::text, tenant_id::text, exam_id::text, COALESCE(exam_paper_id::text, ''), question_no, question_type, score::float8, COALESCE(stem, ''), knowledge_points, answer_area, sort_order, status
FROM question
WHERE tenant_id = $1 AND exam_id = $2 AND deleted_at IS NULL AND status <> 'deleted'
ORDER BY sort_order, question_no
`, tenantID, examID)
	if err != nil {
		return nil, err
	}
	out := []Question{}
	for rows.Next() {
		var item Question
		if err := scanQuestion(rows, &item); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	for index := range out {
		if key, ok, err := s.latestAnswerKey(ctx, tenantID, out[index].ID); err != nil {
			return nil, err
		} else if ok {
			out[index].AnswerKey = &key
		}
		if rubric, ok, err := s.latestRubric(ctx, tenantID, out[index].ID); err != nil {
			return nil, err
		} else if ok {
			out[index].Rubric = &rubric
		}
	}
	return out, nil
}

func (s *PostgresStore) UpdateQuestion(ctx context.Context, tenantID string, id string, userID string, input UpdateQuestionInput) (Question, error) {
	current, err := s.question(ctx, tenantID, id)
	if err != nil {
		return Question{}, err
	}
	merged := current
	if input.QuestionNo != nil {
		merged.QuestionNo = *input.QuestionNo
	}
	if input.QuestionType != nil {
		merged.QuestionType = *input.QuestionType
	}
	if input.Score != nil {
		merged.Score = *input.Score
	}
	if input.Stem != nil {
		merged.Stem = *input.Stem
	}
	if input.KnowledgePoints != nil {
		merged.KnowledgePoints = cloneStrings(*input.KnowledgePoints)
	}
	if input.AnswerArea != nil {
		merged.AnswerArea = cloneMap(*input.AnswerArea)
	}
	if input.SortOrder != nil {
		merged.SortOrder = *input.SortOrder
	}
	kp, _ := json.Marshal(merged.KnowledgePoints)
	area, _ := json.Marshal(merged.AnswerArea)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Question{}, err
	}
	defer tx.Rollback()
	row := tx.QueryRowContext(ctx, `
UPDATE question
SET question_no = $3, question_type = $4, score = $5, stem = $6, knowledge_points = $7, answer_area = $8, sort_order = $9, updated_at = now()
WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL
RETURNING id::text, tenant_id::text, exam_id::text, COALESCE(exam_paper_id::text, ''), question_no, question_type, score::float8, COALESCE(stem, ''), knowledge_points, answer_area, sort_order, status
`, tenantID, id, merged.QuestionNo, merged.QuestionType, merged.Score, merged.Stem, kp, area, merged.SortOrder)
	var out Question
	if err := scanQuestion(row, &out); err != nil {
		return Question{}, err
	}
	if input.AnswerKey != nil {
		key, err := s.insertAnswerKey(ctx, tx, tenantID, out.ID, userID, "v2", *input.AnswerKey)
		if err != nil {
			return Question{}, err
		}
		out.AnswerKey = &key
	}
	if err := tx.Commit(); err != nil {
		return Question{}, err
	}
	return out, nil
}

func (s *PostgresStore) DeleteQuestion(ctx context.Context, tenantID string, id string) error {
	result, err := s.db.ExecContext(ctx, `UPDATE question SET status = 'deleted', deleted_at = now(), updated_at = now() WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`, tenantID, id)
	if err != nil {
		return err
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PostgresStore) CreateRubric(ctx context.Context, tenantID string, questionID string, userID string, input RubricInput) (Rubric, error) {
	question, err := s.question(ctx, tenantID, questionID)
	if err != nil {
		return Rubric{}, err
	}
	if latest, ok, err := s.latestRubric(ctx, tenantID, questionID); err != nil {
		return Rubric{}, err
	} else if ok && latest.Status == "locked" {
		return Rubric{}, ErrRubricLocked
	}
	if !ValidRubricEvidenceRequirements(input.Points) {
		return Rubric{}, ErrInvalidInput
	}
	if !scoreEqual(SumRubricPoints(input.Points), question.Score) || !scoreEqual(input.MaxScore, question.Score) {
		return Rubric{}, ErrRubricMismatch
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Rubric{}, err
	}
	defer tx.Rollback()
	versionNo := 1
	_ = tx.QueryRowContext(ctx, `SELECT COUNT(*) + 1 FROM rubric_version WHERE tenant_id = $1 AND question_id = $2`, tenantID, questionID).Scan(&versionNo)
	version := fmt.Sprintf("v%d", versionNo)
	points, _ := json.Marshal(input.Points)
	deductions, _ := json.Marshal(input.Deductions)
	examples, _ := json.Marshal(input.Examples)
	status := input.Status
	if status == "" {
		status = "draft"
	}
	hash := contentHash(points, deductions, examples)
	var versionID string
	if err := tx.QueryRowContext(ctx, `
INSERT INTO rubric_version (tenant_id, question_id, version, status, content_hash, created_by)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id::text
`, tenantID, questionID, version, status, hash, userID).Scan(&versionID); err != nil {
		return Rubric{}, err
	}
	row := tx.QueryRowContext(ctx, `
INSERT INTO question_rubric (tenant_id, question_id, rubric_version_id, status, max_score, points, deductions, examples, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING id::text
`, tenantID, questionID, versionID, status, input.MaxScore, points, deductions, examples, userID)
	var id string
	if err := row.Scan(&id); err != nil {
		return Rubric{}, err
	}
	if err := tx.Commit(); err != nil {
		return Rubric{}, err
	}
	return Rubric{ID: id, QuestionID: questionID, Version: version, Status: status, MaxScore: input.MaxScore, Points: input.Points, Deductions: input.Deductions, Examples: input.Examples}, nil
}

func (s *PostgresStore) ValidateConfig(ctx context.Context, tenantID string, examID string) (ValidationResult, error) {
	result := ValidationResult{Valid: true, Issues: []ValidationIssue{}}
	var examTotal float64
	if err := s.db.QueryRowContext(ctx, `SELECT total_score::float8 FROM exam WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`, tenantID, examID).Scan(&examTotal); err != nil {
		return ValidationResult{}, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT question_no, question_type, score::float8, answer_area FROM question WHERE tenant_id = $1 AND exam_id = $2 AND deleted_at IS NULL AND status <> 'deleted'`, tenantID, examID)
	if err != nil {
		return ValidationResult{}, err
	}
	defer rows.Close()
	count := 0
	total := 0.0
	for rows.Next() {
		var no, kind string
		var score float64
		var area []byte
		if err := rows.Scan(&no, &kind, &score, &area); err != nil {
			return ValidationResult{}, err
		}
		count++
		total += score
		if no == "" || kind == "" {
			result.Issues = append(result.Issues, ValidationIssue{Code: "question_incomplete", Message: "question is missing question_no or question_type"})
		}
		if len(area) == 0 || string(area) == "null" {
			result.Issues = append(result.Issues, ValidationIssue{Code: "answer_area_missing", Message: "question " + no + " is missing answer_area"})
		}
	}
	if count == 0 {
		result.Issues = append(result.Issues, ValidationIssue{Code: "no_questions", Message: "exam has no questions"})
	}
	if !scoreEqual(total, examTotal) {
		result.Issues = append(result.Issues, ValidationIssue{Code: "total_score_mismatch", Message: fmt.Sprintf("question total %.2f does not equal exam total %.2f", total, examTotal)})
	}
	result.Valid = len(result.Issues) == 0
	return result, rows.Err()
}

func (s *PostgresStore) insertAnswerKey(ctx context.Context, tx *sql.Tx, tenantID string, questionID string, userID string, version string, input AnswerKeyInput) (AnswerKey, error) {
	standard, _ := json.Marshal(input.StandardAnswer)
	equiv, _ := json.Marshal(input.EquivalentAnswers)
	tolerance, _ := json.Marshal(input.Tolerance)
	row := tx.QueryRowContext(ctx, `
INSERT INTO question_answer_key (tenant_id, question_id, answer_version, standard_answer, equivalent_answers, tolerance, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id::text
`, tenantID, questionID, version, standard, equiv, tolerance, userID)
	var id string
	if err := row.Scan(&id); err != nil {
		return AnswerKey{}, err
	}
	return AnswerKey{ID: id, QuestionID: questionID, AnswerVersion: version, StandardAnswer: input.StandardAnswer, EquivalentAnswers: input.EquivalentAnswers, Tolerance: input.Tolerance}, nil
}

func ensurePaperBelongsToExam(ctx context.Context, tx *sql.Tx, tenantID string, examID string, paperID string) error {
	if paperID == "" {
		return nil
	}
	var exists int
	err := tx.QueryRowContext(ctx, `
SELECT 1
FROM exam_paper
WHERE tenant_id = $1 AND exam_id = $2 AND id::text = $3 AND deleted_at IS NULL
`, tenantID, examID, paperID).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

func (s *PostgresStore) question(ctx context.Context, tenantID string, id string) (Question, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id::text, tenant_id::text, exam_id::text, COALESCE(exam_paper_id::text, ''), question_no, question_type, score::float8, COALESCE(stem, ''), knowledge_points, answer_area, sort_order, status
FROM question
WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL AND status <> 'deleted'
`, tenantID, id)
	var out Question
	if err := scanQuestion(row, &out); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Question{}, ErrNotFound
		}
		return Question{}, err
	}
	return out, nil
}

func (s *PostgresStore) latestRubric(ctx context.Context, tenantID string, questionID string) (Rubric, bool, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT qr.id::text, qr.question_id::text, rv.version, qr.status, qr.max_score::float8, qr.points, qr.deductions, qr.examples
FROM question_rubric qr
JOIN rubric_version rv ON rv.tenant_id = qr.tenant_id AND rv.id = qr.rubric_version_id
WHERE qr.tenant_id = $1 AND qr.question_id = $2 AND qr.deleted_at IS NULL
ORDER BY qr.created_at DESC
LIMIT 1
`, tenantID, questionID)
	var out Rubric
	var points, deductions, examples []byte
	if err := row.Scan(&out.ID, &out.QuestionID, &out.Version, &out.Status, &out.MaxScore, &points, &deductions, &examples); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Rubric{}, false, nil
		}
		return Rubric{}, false, err
	}
	_ = json.Unmarshal(points, &out.Points)
	_ = json.Unmarshal(deductions, &out.Deductions)
	_ = json.Unmarshal(examples, &out.Examples)
	return out, true, nil
}

func (s *PostgresStore) latestAnswerKey(ctx context.Context, tenantID string, questionID string) (AnswerKey, bool, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id::text, question_id::text, answer_version, standard_answer, equivalent_answers, tolerance
FROM question_answer_key
WHERE tenant_id = $1 AND question_id = $2 AND deleted_at IS NULL
ORDER BY created_at DESC
LIMIT 1
`, tenantID, questionID)
	var out AnswerKey
	var standard, equivalent, tolerance []byte
	if err := row.Scan(&out.ID, &out.QuestionID, &out.AnswerVersion, &standard, &equivalent, &tolerance); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return AnswerKey{}, false, nil
		}
		return AnswerKey{}, false, err
	}
	_ = json.Unmarshal(standard, &out.StandardAnswer)
	_ = json.Unmarshal(equivalent, &out.EquivalentAnswers)
	_ = json.Unmarshal(tolerance, &out.Tolerance)
	return out, true, nil
}

type questionScanner interface {
	Scan(dest ...any) error
}

func scanQuestion(row questionScanner, out *Question) error {
	var kp, area []byte
	if err := row.Scan(&out.ID, &out.TenantID, &out.ExamID, &out.ExamPaperID, &out.QuestionNo, &out.QuestionType, &out.Score, &out.Stem, &kp, &area, &out.SortOrder, &out.Status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	_ = json.Unmarshal(kp, &out.KnowledgePoints)
	if len(area) > 0 {
		_ = json.Unmarshal(area, &out.AnswerArea)
	}
	return nil
}

func contentHash(parts ...[]byte) string {
	hash := sha256.New()
	for _, part := range parts {
		_, _ = hash.Write(part)
	}
	return hex.EncodeToString(hash.Sum(nil))
}
