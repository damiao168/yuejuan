package paper

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
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

func (s *PostgresStore) CreatePaperImport(ctx context.Context, tenantID, examID, userID string, input CreatePaperImportInput) (PaperImportJob, error) {
	var valid bool
	err := s.db.QueryRowContext(ctx, `SELECT EXISTS(
SELECT 1 FROM exam_paper p
JOIN file_asset pf ON pf.tenant_id=p.tenant_id AND pf.id=$4::uuid AND pf.deleted_at IS NULL
JOIN file_asset af ON af.tenant_id=p.tenant_id AND af.id=$5::uuid AND af.deleted_at IS NULL
WHERE p.tenant_id=$1 AND p.exam_id=$2 AND p.id=$3::uuid AND p.deleted_at IS NULL
  AND (pf.exam_id IS NULL OR pf.exam_id=$2) AND (af.exam_id IS NULL OR af.exam_id=$2))`, tenantID, examID, input.ExamPaperID, input.PaperFileAssetID, input.AnswerFileAssetID).Scan(&valid)
	if err != nil {
		return PaperImportJob{}, err
	}
	if !valid {
		return PaperImportJob{}, ErrInvalidInput
	}
	row := s.db.QueryRowContext(ctx, `INSERT INTO paper_import_job
(tenant_id, exam_id, exam_paper_id, paper_file_asset_id, answer_file_asset_id, status, subject, created_by)
VALUES ($1,$2,$3,$4,$5,'processing',$6,$7)
RETURNING id::text,tenant_id::text,exam_id::text,exam_paper_id::text,paper_file_asset_id::text,answer_file_asset_id::text,status,subject,draft_questions,issues,error_code,created_by::text,created_at,updated_at,applied_at`, tenantID, examID, input.ExamPaperID, input.PaperFileAssetID, input.AnswerFileAssetID, input.Subject, userID)
	return scanPaperImport(row)
}

func (s *PostgresStore) CompletePaperImport(ctx context.Context, tenantID, id string, questions []PaperImportDraftQuestion, issues []string) (PaperImportJob, error) {
	var examID string
	if err := s.db.QueryRowContext(ctx, `SELECT exam_id::text FROM paper_import_job WHERE tenant_id=$1 AND id=$2::uuid AND deleted_at IS NULL`, tenantID, id).Scan(&examID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return PaperImportJob{}, ErrNotFound
		}
		return PaperImportJob{}, err
	}
	var err error
	questions, issues, err = reconcilePaperImportQuestions(ctx, s.db, tenantID, examID, questions, issues)
	if err != nil {
		return PaperImportJob{}, err
	}
	q, _ := json.Marshal(questions)
	i, _ := json.Marshal(issues)
	row := s.db.QueryRowContext(ctx, `UPDATE paper_import_job SET status='review_required',draft_questions=$3,issues=$4,error_code='',updated_at=now() WHERE tenant_id=$1 AND id=$2::uuid AND deleted_at IS NULL RETURNING id::text,tenant_id::text,exam_id::text,exam_paper_id::text,paper_file_asset_id::text,answer_file_asset_id::text,status,subject,draft_questions,issues,error_code,created_by::text,created_at,updated_at,applied_at`, tenantID, id, q, i)
	return scanPaperImport(row)
}

type paperImportQuestionReader interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

type paperImportExistingQuestion struct {
	id, number, kind string
	score            float64
	sortOrder        int
}

func reconcilePaperImportQuestions(ctx context.Context, reader paperImportQuestionReader, tenantID, examID string, drafts []PaperImportDraftQuestion, baseIssues []string) ([]PaperImportDraftQuestion, []string, error) {
	rows, err := reader.QueryContext(ctx, `SELECT id::text,question_no,question_type,score::float8,sort_order FROM question WHERE tenant_id=$1 AND exam_id=$2::uuid AND deleted_at IS NULL AND status<>'deleted' ORDER BY sort_order,question_no`, tenantID, examID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	existing := []paperImportExistingQuestion{}
	for rows.Next() {
		var item paperImportExistingQuestion
		if err := rows.Scan(&item.id, &item.number, &item.kind, &item.score, &item.sortOrder); err != nil {
			return nil, nil, err
		}
		existing = append(existing, item)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	issues := append([]string{}, baseIssues...)
	if len(existing) == 0 {
		for i := range drafts {
			drafts[i].MatchStatus = "create"
			drafts[i].MatchedQuestionID = ""
		}
		return drafts, dedupeStrings(issues), nil
	}
	byNumber := map[string][]paperImportExistingQuestion{}
	for _, item := range existing {
		key := strings.TrimSpace(item.number)
		byNumber[key] = append(byNumber[key], item)
	}
	used := map[string]bool{}
	seenDraftNumbers := map[string]bool{}
	for index := range drafts {
		draft := &drafts[index]
		draft.MatchedQuestionID = ""
		draft.MatchStatus = "extra"
		key := strings.TrimSpace(draft.QuestionNo)
		if key != "" && seenDraftNumbers[key] {
			draft.MatchStatus = "ambiguous"
			draft.Issues = append(draft.Issues, "AI 结果包含重复题号 "+key)
			issues = append(issues, "AI 结果包含重复题号 "+key)
			continue
		}
		seenDraftNumbers[key] = key != ""
		matches := byNumber[key]
		var matched *paperImportExistingQuestion
		if len(matches) == 1 && !used[matches[0].id] {
			candidate := matches[0]
			matched = &candidate
		} else if len(matches) > 1 {
			draft.MatchStatus = "ambiguous"
			draft.Issues = append(draft.Issues, "题号 "+key+" 无法唯一匹配蓝图题目")
			issues = append(issues, "题号 "+key+" 无法唯一匹配蓝图题目")
			continue
		} else if len(drafts) == len(existing) && index < len(existing) && !used[existing[index].id] {
			candidate := existing[index]
			matched = &candidate
			draft.MatchStatus = "matched_by_order"
			draft.Issues = append(draft.Issues, fmt.Sprintf("题号 %s 未直接命中，按第 %d 题候选匹配蓝图题号 %s，请核对", draft.QuestionNo, index+1, candidate.number))
		}
		if matched == nil {
			draft.Issues = append(draft.Issues, "未找到可唯一匹配的蓝图题目")
			issues = append(issues, "AI 多识别题目 "+draft.QuestionNo)
			continue
		}
		used[matched.id] = true
		draft.MatchedQuestionID = matched.id
		if draft.MatchStatus == "extra" {
			draft.MatchStatus = "matched"
		}
		mismatch := false
		if draft.QuestionType != matched.kind {
			mismatch = true
			draft.Issues = append(draft.Issues, fmt.Sprintf("题型不一致：AI=%s，蓝图=%s；将保留蓝图题型", draft.QuestionType, matched.kind))
		}
		if !scoreEqual(draft.Score, matched.score) {
			mismatch = true
			draft.Issues = append(draft.Issues, fmt.Sprintf("分值不一致：AI=%.2f，蓝图=%.2f；将保留蓝图分值", draft.Score, matched.score))
		}
		if mismatch {
			draft.MatchStatus = "mismatch"
			issues = append(issues, "题目 "+matched.number+" 的题型或分值与蓝图不一致")
		}
		draft.Issues = dedupeStrings(draft.Issues)
	}
	for _, item := range existing {
		if !used[item.id] {
			issues = append(issues, "AI 漏识别蓝图题目 "+item.number)
		}
	}
	var draftTotal, existingTotal float64
	for _, item := range drafts {
		draftTotal += item.Score
	}
	for _, item := range existing {
		existingTotal += item.score
	}
	if !scoreEqual(draftTotal, existingTotal) {
		issues = append(issues, fmt.Sprintf("AI 识别总分 %.2f 与蓝图总分 %.2f 不一致", draftTotal, existingTotal))
	}
	return drafts, dedupeStrings(issues), nil
}

func dedupeStrings(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	return out
}

func (s *PostgresStore) FailPaperImport(ctx context.Context, tenantID, id, code string, issues []string) (PaperImportJob, error) {
	i, _ := json.Marshal(issues)
	row := s.db.QueryRowContext(ctx, `UPDATE paper_import_job SET status='failed',issues=$3,error_code=$4,updated_at=now() WHERE tenant_id=$1 AND id=$2::uuid AND deleted_at IS NULL RETURNING id::text,tenant_id::text,exam_id::text,exam_paper_id::text,paper_file_asset_id::text,answer_file_asset_id::text,status,subject,draft_questions,issues,error_code,created_by::text,created_at,updated_at,applied_at`, tenantID, id, i, code)
	return scanPaperImport(row)
}

func (s *PostgresStore) GetPaperImport(ctx context.Context, tenantID, id string) (PaperImportJob, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id::text,tenant_id::text,exam_id::text,exam_paper_id::text,paper_file_asset_id::text,answer_file_asset_id::text,status,subject,draft_questions,issues,error_code,created_by::text,created_at,updated_at,applied_at FROM paper_import_job WHERE tenant_id=$1 AND id=$2::uuid AND deleted_at IS NULL`, tenantID, id)
	return scanPaperImport(row)
}

func (s *PostgresStore) ListPaperImports(ctx context.Context, tenantID, examID string) ([]PaperImportJob, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id::text,tenant_id::text,exam_id::text,exam_paper_id::text,paper_file_asset_id::text,answer_file_asset_id::text,status,subject,draft_questions,issues,error_code,created_by::text,created_at,updated_at,applied_at FROM paper_import_job WHERE tenant_id=$1 AND exam_id=$2 AND deleted_at IS NULL ORDER BY created_at DESC`, tenantID, examID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PaperImportJob{}
	for rows.Next() {
		item, err := scanPaperImport(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *PostgresStore) ApplyPaperImport(ctx context.Context, tenantID, id, userID string) (PaperImportJob, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return PaperImportJob{}, err
	}
	defer tx.Rollback()
	job, err := scanPaperImport(tx.QueryRowContext(ctx, `SELECT id::text,tenant_id::text,exam_id::text,exam_paper_id::text,paper_file_asset_id::text,answer_file_asset_id::text,status,subject,draft_questions,issues,error_code,created_by::text,created_at,updated_at,applied_at FROM paper_import_job WHERE tenant_id=$1 AND id=$2::uuid AND deleted_at IS NULL FOR UPDATE`, tenantID, id))
	if errors.Is(err, sql.ErrNoRows) {
		return PaperImportJob{}, ErrNotFound
	}
	if err != nil {
		return PaperImportJob{}, err
	}
	if job.Status != "review_required" {
		return PaperImportJob{}, ErrConflict
	}
	if err := ensureExamPaperMutableTx(ctx, tx, tenantID, job.ExamID); err != nil {
		return PaperImportJob{}, err
	}
	var existing int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM question WHERE tenant_id=$1 AND exam_id=$2 AND deleted_at IS NULL AND status<>'deleted'`, tenantID, job.ExamID).Scan(&existing); err != nil {
		return PaperImportJob{}, err
	}
	job.Questions, job.Issues, err = reconcilePaperImportQuestions(ctx, tx, tenantID, job.ExamID, job.Questions, job.Issues)
	if err != nil {
		return PaperImportJob{}, err
	}
	for index, draft := range job.Questions {
		if err := validateQuestionInput(draft.QuestionNo, draft.QuestionType, draft.Score); err != nil {
			return PaperImportJob{}, ErrInvalidInput
		}
		if existing > 0 {
			if draft.MatchedQuestionID == "" || draft.MatchStatus == "extra" || draft.MatchStatus == "ambiguous" {
				continue
			}
			kp, _ := json.Marshal(draft.KnowledgePoints)
			result, updateErr := tx.ExecContext(ctx, `UPDATE question SET exam_paper_id=$3::uuid,stem=$4,knowledge_points=$5,updated_at=now() WHERE tenant_id=$1 AND id=$2::uuid AND exam_id=$6::uuid AND deleted_at IS NULL AND status<>'deleted'`, tenantID, draft.MatchedQuestionID, job.ExamPaperID, draft.Stem, kp, job.ExamID)
			if updateErr != nil {
				return PaperImportJob{}, updateErr
			}
			if affected, _ := result.RowsAffected(); affected != 1 {
				return PaperImportJob{}, ErrConflict
			}
			// A core mismatch is intentionally review-only. The blueprint score and
			// type remain authoritative, and importing an answer/rubric for a
			// different question shape would create a second inconsistency.
			if draft.MatchStatus == "mismatch" {
				continue
			}
			if err := s.applyImportedAnswerAndRubric(ctx, tx, tenantID, draft.MatchedQuestionID, userID, draft); err != nil {
				return PaperImportJob{}, err
			}
			continue
		}
		kp, _ := json.Marshal(draft.KnowledgePoints)
		area := []byte(`{}`)
		var questionID string
		if err := tx.QueryRowContext(ctx, `INSERT INTO question (tenant_id,exam_id,exam_paper_id,question_no,question_type,score,stem,knowledge_points,answer_area,sort_order,status) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,'active') RETURNING id::text`, tenantID, job.ExamID, job.ExamPaperID, draft.QuestionNo, draft.QuestionType, draft.Score, draft.Stem, kp, area, index+1).Scan(&questionID); err != nil {
			return PaperImportJob{}, err
		}
		if err := s.applyImportedAnswerAndRubric(ctx, tx, tenantID, questionID, userID, draft); err != nil {
			return PaperImportJob{}, err
		}
	}
	questionsJSON, _ := json.Marshal(job.Questions)
	issuesJSON, _ := json.Marshal(job.Issues)
	job, err = scanPaperImport(tx.QueryRowContext(ctx, `UPDATE paper_import_job SET status='applied',draft_questions=$3,issues=$4,applied_at=now(),updated_at=now() WHERE tenant_id=$1 AND id=$2::uuid RETURNING id::text,tenant_id::text,exam_id::text,exam_paper_id::text,paper_file_asset_id::text,answer_file_asset_id::text,status,subject,draft_questions,issues,error_code,created_by::text,created_at,updated_at,applied_at`, tenantID, id, questionsJSON, issuesJSON))
	if err != nil {
		return PaperImportJob{}, err
	}
	if err := tx.Commit(); err != nil {
		return PaperImportJob{}, err
	}
	return job, nil
}

func (s *PostgresStore) applyImportedAnswerAndRubric(ctx context.Context, tx *sql.Tx, tenantID, questionID, userID string, draft PaperImportDraftQuestion) error {
	if draft.AnswerKey != nil {
		var count int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM question_answer_key WHERE tenant_id=$1 AND question_id=$2::uuid AND deleted_at IS NULL`, tenantID, questionID).Scan(&count); err != nil {
			return err
		}
		if _, err := s.insertAnswerKey(ctx, tx, tenantID, questionID, userID, fmt.Sprintf("v%d", count+1), *draft.AnswerKey); err != nil {
			return err
		}
	}
	if draft.Rubric == nil {
		return nil
	}
	if !scoreEqual(SumRubricPoints(draft.Rubric.Points), draft.Score) || !scoreEqual(draft.Rubric.MaxScore, draft.Score) {
		return ErrRubricMismatch
	}
	var count int
	var locked bool
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*),COALESCE(BOOL_OR(status='locked'),false) FROM rubric_version WHERE tenant_id=$1 AND question_id=$2::uuid AND deleted_at IS NULL`, tenantID, questionID).Scan(&count, &locked); err != nil {
		return err
	}
	if locked {
		return ErrRubricLocked
	}
	points, _ := json.Marshal(draft.Rubric.Points)
	deductions, _ := json.Marshal(draft.Rubric.Deductions)
	examples, _ := json.Marshal(draft.Rubric.Examples)
	hash := contentHash(points, deductions, examples)
	var versionID string
	if err := tx.QueryRowContext(ctx, `INSERT INTO rubric_version (tenant_id,question_id,version,status,content_hash,created_by) VALUES ($1,$2,$3,'draft',$4,$5) RETURNING id::text`, tenantID, questionID, fmt.Sprintf("v%d", count+1), hash, userID).Scan(&versionID); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO question_rubric (tenant_id,question_id,rubric_version_id,status,max_score,points,deductions,examples,created_by) VALUES ($1,$2,$3,'draft',$4,$5,$6,$7,$8)`, tenantID, questionID, versionID, draft.Rubric.MaxScore, points, deductions, examples, userID)
	return err
}

type paperImportScanner interface{ Scan(...any) error }

func scanPaperImport(row paperImportScanner) (PaperImportJob, error) {
	var out PaperImportJob
	var questions, issues []byte
	var applied sql.NullTime
	err := row.Scan(&out.ID, &out.TenantID, &out.ExamID, &out.ExamPaperID, &out.PaperFileAssetID, &out.AnswerFileAssetID, &out.Status, &out.Subject, &questions, &issues, &out.ErrorCode, &out.CreatedBy, &out.CreatedAt, &out.UpdatedAt, &applied)
	if errors.Is(err, sql.ErrNoRows) {
		return PaperImportJob{}, ErrNotFound
	}
	if err != nil {
		return PaperImportJob{}, err
	}
	if len(questions) > 0 {
		_ = json.Unmarshal(questions, &out.Questions)
	}
	if out.Questions == nil {
		out.Questions = []PaperImportDraftQuestion{}
	}
	if len(issues) > 0 {
		_ = json.Unmarshal(issues, &out.Issues)
	}
	if out.Issues == nil {
		out.Issues = []string{}
	}
	if applied.Valid {
		out.AppliedAt = &applied.Time
	}
	return out, nil
}

func (s *PostgresStore) CreateQuestion(ctx context.Context, tenantID string, examID string, userID string, input CreateQuestionInput) (Question, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Question{}, err
	}
	defer tx.Rollback()
	if err := ensureExamPaperMutableTx(ctx, tx, tenantID, examID); err != nil {
		return Question{}, err
	}
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
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Question{}, err
	}
	defer tx.Rollback()
	var current Question
	if err := scanQuestion(tx.QueryRowContext(ctx, `SELECT id::text,tenant_id::text,exam_id::text,COALESCE(exam_paper_id::text,''),question_no,question_type,score::float8,COALESCE(stem,''),knowledge_points,answer_area,sort_order,status FROM question WHERE tenant_id=$1 AND id=$2::uuid AND deleted_at IS NULL AND status<>'deleted' FOR UPDATE`, tenantID, id), &current); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Question{}, ErrNotFound
		}
		return Question{}, err
	}
	if err := ensureExamPaperMutableTx(ctx, tx, tenantID, current.ExamID); err != nil {
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
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var examID string
	if err := tx.QueryRowContext(ctx, `SELECT exam_id::text FROM question WHERE tenant_id=$1 AND id=$2::uuid AND deleted_at IS NULL AND status<>'deleted' FOR UPDATE`, tenantID, id).Scan(&examID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if err := ensureExamPaperMutableTx(ctx, tx, tenantID, examID); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `UPDATE question SET status = 'deleted', deleted_at = now(), updated_at = now() WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`, tenantID, id)
	if err != nil {
		return err
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return ErrNotFound
	}
	return tx.Commit()
}

func (s *PostgresStore) CreateRubric(ctx context.Context, tenantID string, questionID string, userID string, input RubricInput) (Rubric, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Rubric{}, err
	}
	defer tx.Rollback()
	var question Question
	if err := scanQuestion(tx.QueryRowContext(ctx, `SELECT id::text,tenant_id::text,exam_id::text,COALESCE(exam_paper_id::text,''),question_no,question_type,score::float8,COALESCE(stem,''),knowledge_points,answer_area,sort_order,status FROM question WHERE tenant_id=$1 AND id=$2::uuid AND deleted_at IS NULL AND status<>'deleted' FOR UPDATE`, tenantID, questionID), &question); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Rubric{}, ErrNotFound
		}
		return Rubric{}, err
	}
	if err := ensureExamPaperMutableTx(ctx, tx, tenantID, question.ExamID); err != nil {
		return Rubric{}, err
	}
	var locked bool
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(BOOL_OR(status='locked'),false) FROM rubric_version WHERE tenant_id=$1 AND question_id=$2::uuid AND deleted_at IS NULL`, tenantID, questionID).Scan(&locked); err != nil {
		return Rubric{}, err
	}
	if locked {
		return Rubric{}, ErrRubricLocked
	}
	if !ValidRubricEvidenceRequirements(input.Points) {
		return Rubric{}, ErrInvalidInput
	}
	if !scoreEqual(SumRubricPoints(input.Points), question.Score) || !scoreEqual(input.MaxScore, question.Score) {
		return Rubric{}, ErrRubricMismatch
	}
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

func ensureExamPaperMutableTx(ctx context.Context, tx *sql.Tx, tenantID, examID string) error {
	var status string
	if err := tx.QueryRowContext(ctx, `SELECT status FROM exam WHERE tenant_id=$1 AND id=$2::uuid AND deleted_at IS NULL FOR UPDATE`, tenantID, examID).Scan(&status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if status != "draft" && status != "configured" {
		return ErrExamFrozen
	}
	return nil
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
