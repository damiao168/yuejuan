package paper

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"edugrade-enterprise/services/api-gateway/internal/workerruntime"
)

func (s *PostgresStore) QueuePaperImportOCR(ctx context.Context, tenantID string, job PaperImportJob, actorID string, assets []PaperImportOCRAsset) error {
	if len(assets) == 0 {
		return ErrInvalidInput
	}
	documents := make([]map[string]any, 0, len(assets))
	for _, asset := range assets {
		if asset.SourceID == "" || asset.DocumentIndex < 0 || !validPaperImportRole(asset.RoleHint, true) {
			return ErrInvalidInput
		}
		documents = append(documents, map[string]any{
			"source_id": asset.SourceID, "document_index": asset.DocumentIndex, "role_hint": asset.RoleHint, "file_asset_id": asset.FileAssetID,
			"download_url": "/api/v1/files/" + asset.FileAssetID + "/download",
			"content_type": asset.ContentType, "render_dpi": 220, "max_pages": 100,
		})
	}
	sourceRevision := paperImportSourceConfigurationHash(job.Sources)
	payload, _ := json.Marshal(map[string]any{"paper_import_id": job.ID, "exam_id": job.ExamID, "run_id": job.RunID, "generation": job.Generation, "source_revision": sourceRevision, "documents": documents})
	idempotencyKey := fmt.Sprintf("paper-import:%s:g:%d:decode:%s", job.ID, job.Generation, contentHash(payload))
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	runID, generation, currentRevision, runErr := currentPaperImportRunInTx(ctx, tx, tenantID, job.ID)
	if runErr != nil {
		return runErr
	}
	if generation != job.Generation || runID != job.RunID || currentRevision != sourceRevision {
		return ErrConflict
	}
	if err = validatePaperImportDispatchLease(ctx, tx, tenantID, job); err != nil {
		return err
	}
	if err = lockPaperImportTasksInTx(ctx, tx, tenantID, job.ID); err != nil {
		return err
	}
	var taskID string
	if err = tx.QueryRowContext(ctx, `INSERT INTO agent_worker_task(tenant_id,task_type,queue_name,source_type,source_id,priority,payload,payload_schema_version,idempotency_key,dedupe_key,max_attempts,retry_backoff_seconds,created_by,paper_import_run_id,paper_import_generation,task_protocol_version)
VALUES($1,'layout','page-processing','paper_import_job',$2::uuid,55,$3,'paper-import-decode-v3',$4,$4,3,10,$5::uuid,$6::uuid,$7,2)
ON CONFLICT(tenant_id,task_type,idempotency_key) DO UPDATE SET updated_at=agent_worker_task.updated_at
RETURNING id::text`, tenantID, job.ID, payload, idempotencyKey, actorID, runID, generation).Scan(&taskID); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `
UPDATE agent_worker_task_attempt AS attempt
SET status='cancelled',completed_at=now(),error_code='cancelled'
FROM agent_worker_task AS task
WHERE attempt.tenant_id=$1 AND attempt.task_id=task.id AND attempt.completed_at IS NULL
  AND task.tenant_id=$1 AND task.source_type='paper_import_job' AND task.source_id=$2::uuid
  AND task.id<>$3::uuid AND task.status IN ('queued','leased','running')`, tenantID, job.ID, taskID); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `
UPDATE agent_worker_task
SET status='cancelled',cancelled_at=now(),completed_at=now(),updated_at=now(),revision=revision+1
WHERE tenant_id=$1 AND source_type='paper_import_job' AND source_id=$2::uuid
  AND id<>$3::uuid AND status IN ('queued','leased','running')`, tenantID, job.ID, taskID); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE paper_import_run SET dispatch_status='queued',dispatch_lease_owner=NULL,dispatch_lease_expires_at=NULL,initial_task_id=COALESCE(initial_task_id,$4::uuid),updated_at=now() WHERE tenant_id=$1 AND id=$2::uuid AND generation=$3 AND status='processing'`, tenantID, runID, generation, taskID); err != nil {
		return err
	}
	return tx.Commit()
}

func paperImportSourceConfigurationHash(sources []PaperImportSource) string {
	type sourceConfiguration struct {
		ID            string `json:"id"`
		FileAssetID   string `json:"file_asset_id"`
		DocumentIndex int    `json:"document_index"`
		RoleHint      string `json:"role_hint"`
	}
	ordered := append([]PaperImportSource{}, sources...)
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].DocumentIndex == ordered[j].DocumentIndex {
			return ordered[i].ID < ordered[j].ID
		}
		return ordered[i].DocumentIndex < ordered[j].DocumentIndex
	})
	configurations := make([]sourceConfiguration, 0, len(ordered))
	for _, source := range ordered {
		configurations = append(configurations, sourceConfiguration{ID: source.ID, FileAssetID: source.FileAssetID, DocumentIndex: source.DocumentIndex, RoleHint: source.RoleHint})
	}
	raw, _ := json.Marshal(configurations)
	return contentHash(raw)
}

func (s *PostgresStore) QueuePaperImportParse(ctx context.Context, tenantID string, job PaperImportJob, actorID string, input PaperImportParseRequest) error {
	if len(input.Documents) == 0 || actorID == "" {
		return ErrInvalidInput
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = queuePaperImportParseInTx(ctx, tx, tenantID, job, actorID, input); err != nil {
		return err
	}
	return tx.Commit()
}

func queuePaperImportParseInTx(ctx context.Context, tx *sql.Tx, tenantID string, job PaperImportJob, actorID string, input PaperImportParseRequest) error {
	if tx == nil || tenantID == "" || job.ID == "" || actorID == "" || len(input.Documents) == 0 {
		return ErrInvalidInput
	}
	current, err := getPaperImportInTx(ctx, tx, tenantID, job.ID)
	if err != nil {
		return err
	}
	runID, generation, persistedRevision, err := currentPaperImportRunInTx(ctx, tx, tenantID, job.ID)
	if err != nil {
		return err
	}
	sourceRevision := paperImportSourceConfigurationHash(current.Sources)
	if current.Status != "processing" || sourceRevision != paperImportSourceConfigurationHash(job.Sources) || persistedRevision != sourceRevision || (job.Generation > 0 && job.Generation != generation) {
		return ErrConflict
	}
	if err = validatePaperImportDispatchLease(ctx, tx, tenantID, job); err != nil {
		return err
	}
	if err = lockPaperImportTasksInTx(ctx, tx, tenantID, job.ID); err != nil {
		return err
	}
	sources := make(map[string]PaperImportSource, len(current.Sources))
	for _, source := range current.Sources {
		sources[source.ID] = source
	}
	seen := make(map[string]bool, len(input.Documents))
	for _, document := range input.Documents {
		source, ok := sources[document.SourceID]
		if !ok || seen[document.SourceID] || source.FileAssetID != document.FileAssetID || source.DocumentIndex != document.DocumentIndex || strings.TrimSpace(document.Content) == "" {
			return ErrInvalidInput
		}
		seen[document.SourceID] = true
	}
	if len(seen) != len(sources) {
		return ErrInvalidInput
	}
	documents, err := json.Marshal(input.Documents)
	if err != nil {
		return ErrInvalidInput
	}
	issues, err := json.Marshal(input.ExtraIssues)
	if err != nil {
		return ErrInvalidInput
	}
	serializedInput, _ := json.Marshal(input)
	inputHash := contentHash(serializedInput)
	var inputID string
	err = tx.QueryRowContext(ctx, `
INSERT INTO paper_import_parse_input
  (tenant_id,paper_import_id,run_id,generation,source_revision,input_hash,documents,extra_issues,created_by,protocol_version)
VALUES ($1,$2::uuid,$3::uuid,$4,$5,$6,$7,$8,$9::uuid,2)
ON CONFLICT (tenant_id,run_id,input_hash)
DO UPDATE SET input_hash=EXCLUDED.input_hash
RETURNING id::text`, tenantID, job.ID, runID, generation, sourceRevision, inputHash, documents, issues, actorID).Scan(&inputID)
	if err != nil {
		return err
	}
	key := fmt.Sprintf("paper-import:%s:g:%d:parse:%s", job.ID, generation, inputHash)
	task, err := workerruntime.CreateTaskInTx(ctx, tx, tenantID, actorID, workerruntime.CreateTaskInput{
		TaskType: "paper_parse", QueueName: "paper-parse", SourceType: "paper_import_parse", SourceID: inputID,
		Priority: 55, PayloadSchemaVersion: "paper-import-parse-v1", IdempotencyKey: key, DedupeKey: key,
		Payload:     map[string]any{"paper_import_id": job.ID, "parse_input_id": inputID, "source_revision": sourceRevision},
		MaxAttempts: 3, RetryBackoffSeconds: 15,
	})
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE agent_worker_task SET paper_import_run_id=$3::uuid,paper_import_generation=$4::bigint,task_protocol_version=2,payload_schema_version='paper-import-parse-v2',payload=payload || jsonb_build_object('run_id',$3::text,'generation',$4::bigint) WHERE tenant_id=$1 AND id=$2::uuid`, tenantID, task.ID, runID, generation); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE paper_import_run SET dispatch_status='queued',dispatch_lease_owner=NULL,dispatch_lease_expires_at=NULL,initial_task_id=COALESCE(initial_task_id,$4::uuid),updated_at=now() WHERE tenant_id=$1 AND id=$2::uuid AND generation=$3 AND status='processing'`, tenantID, runID, generation, task.ID); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `
UPDATE agent_worker_task_attempt AS attempt
SET status='cancelled',completed_at=now(),error_code='cancelled'
FROM agent_worker_task AS prior
JOIN paper_import_parse_input AS parse_input
  ON parse_input.tenant_id=prior.tenant_id AND parse_input.id=prior.source_id
WHERE attempt.tenant_id=$1 AND attempt.task_id=prior.id AND attempt.completed_at IS NULL
  AND prior.tenant_id=$1 AND prior.source_type='paper_import_parse'
  AND parse_input.paper_import_id=$2::uuid AND prior.id<>$3::uuid
  AND prior.status IN ('queued','leased','running')`, tenantID, job.ID, task.ID); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
UPDATE agent_worker_task AS prior
SET status='cancelled',cancelled_at=now(),completed_at=now(),updated_at=now(),revision=revision+1
FROM paper_import_parse_input AS parse_input
WHERE prior.tenant_id=$1 AND prior.source_type='paper_import_parse'
  AND parse_input.tenant_id=prior.tenant_id AND parse_input.id=prior.source_id
  AND parse_input.paper_import_id=$2::uuid AND prior.id<>$3::uuid
  AND prior.status IN ('queued','leased','running')`, tenantID, job.ID, task.ID)
	return err
}

func (s *PostgresStore) CompletePaperImportDecode(ctx context.Context, tenantID, importID string, input PaperImportDecodeResult) error {
	if input.TaskID == "" || input.LeaseToken == "" || len(input.Pages) == 0 {
		return ErrInvalidInput
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	runID, generation, sourceRevision, err := currentPaperImportRunInTx(ctx, tx, tenantID, importID)
	if err != nil {
		return err
	}
	if err = validatePaperImportStageBinding(ctx, tx, tenantID, input.TaskID, runID, generation, "layout", "paper_import_job", importID); err != nil {
		return err
	}
	result := mapFromJSON(input)
	if _, err = workerruntime.CompleteTaskInTx(ctx, tx, tenantID, input.TaskID, workerruntime.CompleteInput{LeaseToken: input.LeaseToken, ResultSchemaVersion: "paper-import-decode-result-v3", Result: result, DurationMS: input.DurationMS}); err != nil {
		return err
	}
	var examID, actorID string
	if err = tx.QueryRowContext(ctx, `SELECT exam_id::text,created_by::text FROM paper_import_job WHERE tenant_id=$1 AND id=$2::uuid AND status='processing' AND deleted_at IS NULL FOR UPDATE`, tenantID, importID).Scan(&examID, &actorID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	pages := make([]map[string]any, 0, len(input.Pages))
	for _, page := range input.Pages {
		if page.SourceID == "" || page.DocumentIndex < 0 || page.PageNo <= 0 || page.FileAssetID == "" {
			return ErrInvalidInput
		}
		pages = append(pages, map[string]any{"source_id": page.SourceID, "document_index": page.DocumentIndex, "page_no": page.PageNo, "file_asset_id": page.FileAssetID, "download_url": "/api/v1/files/" + page.FileAssetID + "/download", "sha256": page.SHA256})
	}
	payload, _ := json.Marshal(map[string]any{"paper_import_id": importID, "exam_id": examID, "run_id": runID, "generation": generation, "source_revision": sourceRevision, "engine": "paddleocr", "engine_version": "pp-ocrv5", "pages": pages})
	key := fmt.Sprintf("paper-import:%s:g:%d:ocr:%s", importID, generation, contentHash(payload))
	_, err = tx.ExecContext(ctx, `INSERT INTO agent_worker_task(tenant_id,task_type,queue_name,source_type,source_id,priority,payload,payload_schema_version,idempotency_key,dedupe_key,max_attempts,retry_backoff_seconds,created_by,paper_import_run_id,paper_import_generation,task_protocol_version)
VALUES($1,'ocr','ocr','paper_import_job',$2::uuid,55,$3,'paper-import-ocr-v3',$4,$4,3,10,$5::uuid,$6::uuid,$7,2)
ON CONFLICT(tenant_id,task_type,idempotency_key) DO NOTHING`, tenantID, importID, payload, key, actorID, runID, generation)
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE paper_import_source SET processing_status='processing',updated_at=now() WHERE tenant_id=$1 AND paper_import_id=$2::uuid AND deleted_at IS NULL`, tenantID, importID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *PostgresStore) CompletePaperImportOCR(ctx context.Context, tenantID, importID string, input PaperImportOCRResult, parseInput PaperImportParseRequest) error {
	if input.TaskID == "" || input.LeaseToken == "" || len(input.Blocks) == 0 || len(parseInput.Documents) == 0 {
		return ErrInvalidInput
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	runID, generation, _, err := currentPaperImportRunInTx(ctx, tx, tenantID, importID)
	if err != nil {
		return err
	}
	if err = validatePaperImportStageBinding(ctx, tx, tenantID, input.TaskID, runID, generation, "ocr", "paper_import_job", importID); err != nil {
		return err
	}
	if _, err = workerruntime.CompleteTaskInTx(ctx, tx, tenantID, input.TaskID, workerruntime.CompleteInput{LeaseToken: input.LeaseToken, ResultSchemaVersion: "paper-import-ocr-result-v3", Result: mapFromJSON(input), DurationMS: input.DurationMS}); err != nil {
		return err
	}
	var status, actorID string
	if err = tx.QueryRowContext(ctx, `SELECT status,created_by::text FROM paper_import_job WHERE tenant_id=$1 AND id=$2::uuid AND deleted_at IS NULL FOR UPDATE`, tenantID, importID).Scan(&status, &actorID); err != nil {
		return err
	}
	if status != "processing" {
		return fmt.Errorf("%w: paper import is %s", ErrConflict, status)
	}
	if _, err = tx.ExecContext(ctx, `UPDATE paper_import_source SET processing_status='processed',updated_at=now() WHERE tenant_id=$1 AND paper_import_id=$2::uuid AND deleted_at IS NULL`, tenantID, importID); err != nil {
		return err
	}
	job, err := getPaperImportInTx(ctx, tx, tenantID, importID)
	if err != nil {
		return err
	}
	if err = queuePaperImportParseInTx(ctx, tx, tenantID, job, actorID, parseInput); err != nil {
		return err
	}
	return tx.Commit()
}

func getPaperImportInTx(ctx context.Context, tx *sql.Tx, tenantID, importID string) (PaperImportJob, error) {
	job, err := scanPaperImport(tx.QueryRowContext(ctx, `SELECT id::text,tenant_id::text,exam_id::text,COALESCE(exam_paper_id::text,''),COALESCE(paper_file_asset_id::text,''),COALESCE(answer_file_asset_id::text,''),status,subject,draft_questions,issues,error_code,created_by::text,created_at,updated_at,applied_at FROM paper_import_job WHERE tenant_id=$1 AND id=$2::uuid AND deleted_at IS NULL`, tenantID, importID))
	if err != nil {
		return PaperImportJob{}, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT id::text,file_asset_id::text,document_index,role_hint,detected_role,role_confidence::float8,processing_status,created_at FROM paper_import_source WHERE tenant_id=$1 AND paper_import_id=$2::uuid AND deleted_at IS NULL ORDER BY document_index,id`, tenantID, importID)
	if err != nil {
		return PaperImportJob{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var source PaperImportSource
		if err = rows.Scan(&source.ID, &source.FileAssetID, &source.DocumentIndex, &source.RoleHint, &source.DetectedRole, &source.RoleConfidence, &source.ProcessingStatus, &source.CreatedAt); err != nil {
			return PaperImportJob{}, err
		}
		job.Sources = append(job.Sources, source)
	}
	return job, rows.Err()
}

func (s *PostgresStore) LoadPaperImportParseInput(ctx context.Context, tenantID, inputID string) (string, PaperImportParseRequest, error) {
	var importID string
	var documents, issues []byte
	err := s.db.QueryRowContext(ctx, `SELECT paper_import_id::text,documents,extra_issues FROM paper_import_parse_input WHERE tenant_id=$1 AND id=$2::uuid`, tenantID, inputID).Scan(&importID, &documents, &issues)
	if errors.Is(err, sql.ErrNoRows) {
		return "", PaperImportParseRequest{}, ErrNotFound
	}
	if err != nil {
		return "", PaperImportParseRequest{}, err
	}
	var input PaperImportParseRequest
	if json.Unmarshal(documents, &input.Documents) != nil || json.Unmarshal(issues, &input.ExtraIssues) != nil || len(input.Documents) == 0 {
		return "", PaperImportParseRequest{}, ErrInvalidInput
	}
	return importID, input, nil
}

func (s *PostgresStore) LoadPaperImportParseRunInput(ctx context.Context, tenantID, inputID string) (PaperImportRunBinding, error) {
	var binding PaperImportRunBinding
	var documents, issues []byte
	err := s.db.QueryRowContext(ctx, `SELECT paper_import_id::text,run_id::text,generation,source_revision,input_hash,documents,extra_issues
FROM paper_import_parse_input WHERE tenant_id=$1 AND id=$2::uuid AND protocol_version=2`, tenantID, inputID).Scan(
		&binding.ImportID, &binding.RunID, &binding.Generation, &binding.SourceRevision, &binding.InputHash, &documents, &issues,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return PaperImportRunBinding{}, ErrNotFound
	}
	if err != nil {
		return PaperImportRunBinding{}, err
	}
	binding.InputID = inputID
	if json.Unmarshal(documents, &binding.Input.Documents) != nil || json.Unmarshal(issues, &binding.Input.ExtraIssues) != nil || len(binding.Input.Documents) == 0 {
		return PaperImportRunBinding{}, ErrInvalidInput
	}
	serialized, err := json.Marshal(binding.Input)
	if err != nil || contentHash(serialized) != binding.InputHash {
		return PaperImportRunBinding{}, ErrConflict
	}
	return binding, nil
}

func (s *PostgresStore) FailPaperImportRuntime(ctx context.Context, tenantID, importID string, input PaperImportRuntimeFailure) error {
	if input.TaskID == "" || input.LeaseToken == "" || input.ErrorCode == "" {
		return ErrInvalidInput
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	runID, generation, _, err := currentPaperImportRunInTx(ctx, tx, tenantID, importID)
	if err != nil {
		return err
	}
	if err = validatePaperImportTaskBinding(ctx, tx, tenantID, input.TaskID, runID, generation); err != nil {
		return err
	}
	task, err := workerruntime.FailTaskInTx(ctx, tx, tenantID, input.TaskID, workerruntime.FailInput{LeaseToken: input.LeaseToken, Retryable: input.Retryable, ErrorCode: input.ErrorCode, ErrorDetail: input.ErrorDetail, DurationMS: input.DurationMS})
	if err != nil {
		return err
	}
	if task.Status == workerruntime.StatusQueued {
		return tx.Commit()
	}
	jobErrorCode := "paper_ocr_failed"
	issuePrefix := "扫描文档文字识别失败："
	if input.ErrorCode == "paper_parse_failed" {
		jobErrorCode = "ai_parse_failed"
		issuePrefix = "AI 解析服务暂不可用："
	}
	issues, _ := json.Marshal([]string{issuePrefix + paperImportRuntimeErrorMessage(input.ErrorCode)})
	if _, err = tx.ExecContext(ctx, `UPDATE paper_import_job SET status='failed',error_code=$3,issues=$4,updated_at=now() WHERE tenant_id=$1 AND id=$2::uuid AND status='processing' AND deleted_at IS NULL`, tenantID, importID, jobErrorCode, issues); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE paper_import_source SET processing_status='failed',updated_at=now() WHERE tenant_id=$1 AND paper_import_id=$2::uuid AND deleted_at IS NULL`, tenantID, importID); err != nil {
		return err
	}
	detail, _ := json.Marshal(input.ErrorDetail)
	if _, err = tx.ExecContext(ctx, `UPDATE paper_import_run SET status='failed',error_code=$4,error_detail=$5,completed_at=now(),updated_at=now() WHERE tenant_id=$1 AND id=$2::uuid AND generation=$3 AND status='processing'`, tenantID, runID, generation, input.ErrorCode, detail); err != nil {
		return err
	}
	return tx.Commit()
}

func paperImportRuntimeErrorMessage(code string) string {
	switch strings.TrimSpace(code) {
	case "page_processing_failed":
		return "页面处理失败"
	case "source_ocr_failed":
		return "图片文字识别失败"
	case "paper_ocr_unavailable":
		return "文字识别服务暂不可用"
	case "paper_parse_failed":
		return "结构化解析失败，请稍后重试"
	case "source_download_forbidden":
		return "源文件访问被拒绝，请重新提交资料或联系管理员"
	case "source_not_found":
		return "源文件不存在或已删除"
	case "source_download_failed":
		return "源文件读取失败，请稍后重试"
	default:
		return "处理失败，请检查资料后重试"
	}
}

func mapFromJSON(value any) map[string]any {
	raw, _ := json.Marshal(value)
	out := map[string]any{}
	_ = json.Unmarshal(raw, &out)
	return out
}
