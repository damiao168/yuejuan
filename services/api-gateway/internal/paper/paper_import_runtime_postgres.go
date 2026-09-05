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
	payload, _ := json.Marshal(map[string]any{"paper_import_id": job.ID, "exam_id": job.ExamID, "source_revision": sourceRevision, "documents": documents})
	// UpdatedAt is the durable run revision. Replacing the same sources to retry
	// a failed import must enqueue a fresh task, while duplicate dispatch inside
	// one run remains idempotent.
	idempotencyKey := "paper-import-decode:" + job.ID + ":" + contentHash(payload) + ":" + job.UpdatedAt.UTC().Format("20060102T150405.000000000Z") + ":v3"
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var taskID string
	if err = tx.QueryRowContext(ctx, `INSERT INTO agent_worker_task(tenant_id,task_type,queue_name,source_type,source_id,priority,payload,payload_schema_version,idempotency_key,dedupe_key,max_attempts,retry_backoff_seconds,created_by)
VALUES($1,'layout','page-processing','paper_import_job',$2::uuid,55,$3,'paper-import-decode-v2',$4,$4,3,10,$5::uuid)
ON CONFLICT(tenant_id,task_type,idempotency_key) DO UPDATE SET updated_at=agent_worker_task.updated_at
RETURNING id::text`, tenantID, job.ID, payload, idempotencyKey, actorID).Scan(&taskID); err != nil {
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

func (s *PostgresStore) CompletePaperImportDecode(ctx context.Context, tenantID, importID string, input PaperImportDecodeResult) error {
	if input.TaskID == "" || input.LeaseToken == "" || len(input.Pages) == 0 {
		return ErrInvalidInput
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result := mapFromJSON(input)
	if _, err = workerruntime.CompleteTaskInTx(ctx, tx, tenantID, input.TaskID, workerruntime.CompleteInput{LeaseToken: input.LeaseToken, ResultSchemaVersion: "paper-import-decode-result-v2", Result: result, DurationMS: input.DurationMS}); err != nil {
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
	payload, _ := json.Marshal(map[string]any{"paper_import_id": importID, "exam_id": examID, "engine": "paddleocr", "engine_version": "pp-ocrv5", "pages": pages})
	_, err = tx.ExecContext(ctx, `INSERT INTO agent_worker_task(tenant_id,task_type,queue_name,source_type,source_id,priority,payload,payload_schema_version,idempotency_key,dedupe_key,max_attempts,retry_backoff_seconds,created_by)
VALUES($1,'ocr','ocr','paper_import_job',$2::uuid,55,$3,'paper-import-ocr-v2',$4,$4,3,10,$5::uuid)
ON CONFLICT(tenant_id,task_type,idempotency_key) DO NOTHING`, tenantID, importID, payload, "paper-import-ocr:"+importID+":"+input.TaskID+":v2", actorID)
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE paper_import_source SET processing_status='processing',updated_at=now() WHERE tenant_id=$1 AND paper_import_id=$2::uuid AND deleted_at IS NULL`, tenantID, importID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *PostgresStore) CompletePaperImportOCR(ctx context.Context, tenantID, importID string, input PaperImportOCRResult) error {
	if input.TaskID == "" || input.LeaseToken == "" || len(input.Blocks) == 0 {
		return ErrInvalidInput
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = workerruntime.CompleteTaskInTx(ctx, tx, tenantID, input.TaskID, workerruntime.CompleteInput{LeaseToken: input.LeaseToken, ResultSchemaVersion: "paper-import-ocr-result-v2", Result: mapFromJSON(input), DurationMS: input.DurationMS}); err != nil {
		return err
	}
	var status string
	if err = tx.QueryRowContext(ctx, `SELECT status FROM paper_import_job WHERE tenant_id=$1 AND id=$2::uuid AND deleted_at IS NULL FOR UPDATE`, tenantID, importID).Scan(&status); err != nil {
		return err
	}
	if status != "processing" {
		return fmt.Errorf("%w: paper import is %s", ErrConflict, status)
	}
	if _, err = tx.ExecContext(ctx, `UPDATE paper_import_source SET processing_status='processed',updated_at=now() WHERE tenant_id=$1 AND paper_import_id=$2::uuid AND deleted_at IS NULL`, tenantID, importID); err != nil {
		return err
	}
	return tx.Commit()
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
	if _, err = workerruntime.FailTaskInTx(ctx, tx, tenantID, input.TaskID, workerruntime.FailInput{LeaseToken: input.LeaseToken, Retryable: input.Retryable, ErrorCode: input.ErrorCode, ErrorDetail: input.ErrorDetail, DurationMS: input.DurationMS}); err != nil {
		return err
	}
	issues, _ := json.Marshal([]string{"扫描文档文字识别失败：" + paperImportRuntimeErrorMessage(input.ErrorCode)})
	if _, err = tx.ExecContext(ctx, `UPDATE paper_import_job SET status='failed',error_code='paper_ocr_failed',issues=$3,updated_at=now() WHERE tenant_id=$1 AND id=$2::uuid AND status='processing' AND deleted_at IS NULL`, tenantID, importID, issues); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE paper_import_source SET processing_status='failed',updated_at=now() WHERE tenant_id=$1 AND paper_import_id=$2::uuid AND deleted_at IS NULL`, tenantID, importID); err != nil {
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
