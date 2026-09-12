package paper

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"edugrade-enterprise/services/api-gateway/internal/workerruntime"
	"golang.org/x/text/unicode/norm"
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
	var examID, actorID, subject string
	if err = tx.QueryRowContext(ctx, `SELECT exam_id::text,created_by::text,subject FROM paper_import_job WHERE tenant_id=$1 AND id=$2::uuid AND status='processing' AND deleted_at IS NULL FOR UPDATE`, tenantID, importID).Scan(&examID, &actorID, &subject); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	policy, ok := paperRecognitionPolicy(subject)
	if !ok {
		return ErrInvalidInput
	}
	policyJSON, policyHash := paperRecognitionPolicyJSON(policy)
	if _, err = tx.ExecContext(ctx, `UPDATE paper_import_run SET authoritative_subject_code=$3,recognition_policy_snapshot=$4::jsonb,recognition_policy_hash=$5,updated_at=now() WHERE tenant_id=$1 AND id=$2::uuid`, tenantID, runID, policy.SubjectCode, policyJSON, policyHash); err != nil {
		return err
	}
	pages := make([]map[string]any, 0, len(input.Pages))
	for _, page := range input.Pages {
		if page.SourceID == "" || page.DocumentIndex < 0 || page.PageNo <= 0 || page.FileAssetID == "" {
			return ErrInvalidInput
		}
		pages = append(pages, map[string]any{"source_id": page.SourceID, "document_index": page.DocumentIndex, "page_no": page.PageNo, "file_asset_id": page.FileAssetID, "download_url": "/api/v1/files/" + page.FileAssetID + "/download", "sha256": page.SHA256, "width": page.Width, "height": page.Height})
	}
	payload, _ := json.Marshal(map[string]any{"paper_import_id": importID, "exam_id": examID, "run_id": runID, "generation": generation, "source_revision": sourceRevision, "subject_code": policy.SubjectCode, "recognition_policy_hash": policyHash, "engine": "paddleocr", "engine_version": "pp-ocrv5", "pages": pages})
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
	runID, generation, sourceRevision, err := currentPaperImportRunInTx(ctx, tx, tenantID, importID)
	if err != nil {
		return err
	}
	if err = validatePaperImportStageBinding(ctx, tx, tenantID, input.TaskID, runID, generation, "ocr", "paper_import_job", importID); err != nil {
		return err
	}
	var policyJSON []byte
	var policyHash string
	if err = tx.QueryRowContext(ctx, `SELECT recognition_policy_snapshot,recognition_policy_hash FROM paper_import_run WHERE tenant_id=$1 AND id=$2::uuid`, tenantID, runID).Scan(&policyJSON, &policyHash); err != nil {
		return err
	}
	var policy PaperRecognitionPolicy
	if json.Unmarshal(policyJSON, &policy) != nil {
		return ErrInvalidInput
	}
	var ocrPayload []byte
	if err = tx.QueryRowContext(ctx, `SELECT payload FROM agent_worker_task WHERE tenant_id=$1 AND id=$2::uuid`, tenantID, input.TaskID).Scan(&ocrPayload); err != nil {
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
	if policy.FormulaEnabled {
		var taskPayload map[string]any
		if json.Unmarshal(ocrPayload, &taskPayload) != nil {
			return ErrInvalidInput
		}
		pages, ok := taskPayload["pages"]
		if !ok {
			return ErrInvalidInput
		}
		documents, _ := json.Marshal(parseInput.Documents)
		issues, _ := json.Marshal(parseInput.ExtraIssues)
		pagesJSON, _ := json.Marshal(pages)
		inputHash := contentHash(pagesJSON, documents, issues, policyJSON)
		var formulaInputID string
		if err = tx.QueryRowContext(ctx, `INSERT INTO paper_import_formula_input(tenant_id,paper_import_id,run_id,generation,source_revision,policy_hash,pages,documents,extra_issues,created_by)
VALUES($1,$2::uuid,$3::uuid,$4,$5,$6,$7::jsonb,$8::jsonb,$9::jsonb,$10::uuid)
ON CONFLICT(tenant_id,run_id) DO UPDATE SET policy_hash=EXCLUDED.policy_hash
RETURNING id::text`, tenantID, importID, runID, generation, sourceRevision, policyHash, pagesJSON, documents, issues, actorID).Scan(&formulaInputID); err != nil {
			return err
		}
		formulaPayload, _ := json.Marshal(map[string]any{"paper_import_id": importID, "exam_id": job.ExamID, "formula_input_id": formulaInputID, "run_id": runID, "generation": generation, "source_revision": sourceRevision, "subject_code": policy.SubjectCode, "recognition_policy": policy, "pages": pages, "input_hash": inputHash})
		key := fmt.Sprintf("paper-import:%s:g:%d:formula:%s", importID, generation, inputHash)
		if _, err = tx.ExecContext(ctx, `INSERT INTO agent_worker_task(tenant_id,task_type,queue_name,source_type,source_id,priority,payload,payload_schema_version,idempotency_key,dedupe_key,max_attempts,retry_backoff_seconds,created_by,paper_import_run_id,paper_import_generation,task_protocol_version)
VALUES($1,'paper_formula','paper-formula','paper_import_job',$2::uuid,58,$3,'paper-import-formula-v2',$4,$4,2,30,$5::uuid,$6::uuid,$7,2)
ON CONFLICT(tenant_id,task_type,idempotency_key) DO NOTHING`, tenantID, importID, formulaPayload, key, actorID, runID, generation); err != nil {
			return err
		}
	} else if err = queuePaperImportParseInTx(ctx, tx, tenantID, job, actorID, parseInput); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *PostgresStore) CompletePaperImportFormula(ctx context.Context, tenantID, importID string, input PaperImportFormulaResult) error {
	if input.TaskID == "" || input.LeaseToken == "" {
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
	if err = validatePaperImportStageBinding(ctx, tx, tenantID, input.TaskID, runID, generation, "paper_formula", "paper_import_job", importID); err != nil {
		return err
	}
	if _, err = workerruntime.CompleteTaskInTx(ctx, tx, tenantID, input.TaskID, workerruntime.CompleteInput{LeaseToken: input.LeaseToken, ResultSchemaVersion: "paper-import-formula-result-v2", Result: mapFromJSON(input), DurationMS: input.DurationMS}); err != nil {
		return err
	}
	var documentsJSON, issuesJSON []byte
	if err = tx.QueryRowContext(ctx, `SELECT documents,extra_issues FROM paper_import_formula_input WHERE tenant_id=$1 AND paper_import_id=$2::uuid AND run_id=$3::uuid FOR UPDATE`, tenantID, importID, runID).Scan(&documentsJSON, &issuesJSON); err != nil {
		return err
	}
	var parseInput PaperImportParseRequest
	if json.Unmarshal(documentsJSON, &parseInput.Documents) != nil || json.Unmarshal(issuesJSON, &parseInput.ExtraIssues) != nil {
		return ErrInvalidInput
	}
	if !validPaperFormulaResult(parseInput.Documents, input.Regions) {
		return ErrInvalidInput
	}
	parseInput = mergePaperFormulaResult(parseInput, input.Regions)
	resultJSON, _ := json.Marshal(input)
	reviewRequired := false
	for _, region := range input.Regions {
		if region.Status != "accepted" {
			reviewRequired = true
			break
		}
	}
	formulaStatus := "succeeded"
	if reviewRequired {
		formulaStatus = "review_required"
	}
	if _, err = tx.ExecContext(ctx, `UPDATE paper_import_formula_input SET result=$4::jsonb,status=$5,completed_at=now() WHERE tenant_id=$1 AND paper_import_id=$2::uuid AND run_id=$3::uuid`, tenantID, importID, runID, resultJSON, formulaStatus); err != nil {
		return err
	}
	var actorID string
	if err = tx.QueryRowContext(ctx, `SELECT created_by::text FROM paper_import_job WHERE tenant_id=$1 AND id=$2::uuid AND status='processing' AND deleted_at IS NULL FOR UPDATE`, tenantID, importID).Scan(&actorID); err != nil {
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

func mergePaperFormulaResult(input PaperImportParseRequest, regions []PaperImportFormulaRegion) PaperImportParseRequest {
	for documentIndex := range input.Documents {
		document := &input.Documents[documentIndex]
		for _, region := range regions {
			if region.SourceID != document.SourceID || region.DocumentIndex != document.DocumentIndex {
				continue
			}
			if region.Status != "accepted" || strings.TrimSpace(region.SelectedLatex) == "" {
				input.ExtraIssues = append(input.ExtraIssues, PaperImportIssue{Code: "FORMULA_REVIEW_REQUIRED", Severity: "warning", Certainty: "confirmed", Message: fmt.Sprintf("第 %d 页有数学公式需要人工核对", region.PageNo), SourceRefs: []PaperImportSourceRef{{SourceID: document.SourceID, FileAssetID: document.FileAssetID, DocumentIndex: document.DocumentIndex, PageNo: region.PageNo, BlockID: region.RegionID, BBox: region.BBox}}, ResolutionHint: "请对照原图核对公式"})
				continue
			}
			formulaBlock := PaperImportOCRBlock{SourceID: region.SourceID, DocumentIndex: region.DocumentIndex, BlockID: region.RegionID, PageNo: region.PageNo, Text: "\\(" + region.SelectedLatex + "\\)", BBox: region.BBox, Confidence: selectedFormulaConfidence(region), Kind: "formula", CanonicalLatex: region.SelectedLatex, Engine: "paddle-formula", EngineVersion: region.SelectedModel, ReviewStatus: "accepted", Segments: []PaperImportContentSegment{{Kind: "formula", Latex: region.SelectedLatex, BBox: region.BBox, SourceBlockID: region.RegionID}}}
			kept := make([]PaperImportOCRBlock, 0, len(document.Blocks)+1)
			insertAt := -1
			mergedInline := false
			for _, block := range document.Blocks {
				if block.PageNo == region.PageNo && paperBBoxOverlap(block.BBox, region.BBox) {
					if paperFormulaReplacementSafe(block, region) {
						if insertAt < 0 {
							insertAt = len(kept)
						}
						continue
					}
					if !mergedInline {
						if mixed, ok := mergeInlineFormulaSegment(block, region); ok {
							kept = append(kept, mixed)
							mergedInline = true
							continue
						}
					}
					if insertAt < 0 {
						insertAt = len(kept) + 1
					}
				}
				kept = append(kept, block)
			}
			if mergedInline {
				document.Blocks = kept
				continue
			}
			if insertAt < 0 {
				insertAt = paperFormulaGeometryInsertAt(kept, region)
			}
			insertAt = max(0, min(insertAt, len(kept)))
			kept = append(kept, PaperImportOCRBlock{})
			copy(kept[insertAt+1:], kept[insertAt:])
			kept[insertAt] = formulaBlock
			document.Blocks = kept
		}
		sortPaperImportOCRBlocks(document.Blocks)
		parts := make([]string, 0, len(document.Blocks))
		for _, block := range document.Blocks {
			if strings.TrimSpace(block.Text) != "" {
				parts = append(parts, block.Text)
			}
		}
		document.Content = strings.Join(parts, "\n")
	}
	return input
}

func validPaperFormulaResult(documents []PaperImportParseDocument, regions []PaperImportFormulaRegion) bool {
	documentKeys := map[string]bool{}
	for _, document := range documents {
		documentKeys[fmt.Sprintf("%s:%d", document.SourceID, document.DocumentIndex)] = true
	}
	seen := map[string]bool{}
	for _, region := range regions {
		key := fmt.Sprintf("%s:%d:%d:%s", region.SourceID, region.DocumentIndex, region.PageNo, region.RegionID)
		if seen[key] || !documentKeys[fmt.Sprintf("%s:%d", region.SourceID, region.DocumentIndex)] || region.PageNo <= 0 || region.RegionID == "" || len(region.BBox) != 4 || region.DetectorModel != "PP-DocLayout_plus-L" || !finiteUnit(region.DetectorConfidence) || !finiteUnit(region.EdgeInkRatio) || len(region.CropSHA256) != 64 || (region.Status != "accepted" && region.Status != "review_required") || len(region.Candidates) > 2 || region.RecropCount < 0 || region.RecropCount > 8 || region.ValidationVersion != "latex-structure-render-v1" {
			return false
		}
		if region.Status == "accepted" && (!region.CropComplete || len(region.Candidates) == 0) {
			return false
		}
		seen[key] = true
		for index, value := range region.BBox {
			if !math.IsNaN(value) && !math.IsInf(value, 0) && ((index < 2 && value >= 0) || (index >= 2 && value > 0)) {
				continue
			}
			return false
		}
		models := map[string]bool{}
		selectedMatches := false
		for _, candidate := range region.Candidates {
			if models[candidate.ModelVersion] || (candidate.ModelVersion != "PP-FormulaNet_plus-M" && candidate.ModelVersion != "PP-FormulaNet_plus-L") || !finiteUnit(candidate.Confidence) || (candidate.RenderSimilarity != nil && !finiteUnit(*candidate.RenderSimilarity)) {
				return false
			}
			if candidate.ValidationAction != "accept" && candidate.ValidationAction != "retry_l" && candidate.ValidationAction != "review" && candidate.ValidationAction != "recrop" {
				return false
			}
			if candidate.Valid != (candidate.SyntaxValid && candidate.StructureValid && candidate.ValidationAction == "accept") {
				return false
			}
			models[candidate.ModelVersion] = true
			if candidate.ModelVersion == region.SelectedModel && candidate.CanonicalLatex == region.SelectedLatex && candidate.Valid {
				selectedMatches = true
			}
		}
		if region.Status == "accepted" && (!selectedMatches || strings.TrimSpace(region.SelectedLatex) == "") {
			return false
		}
	}
	return true
}

func finiteUnit(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= 0 && value <= 1
}

func selectedFormulaConfidence(region PaperImportFormulaRegion) float64 {
	for _, candidate := range region.Candidates {
		if candidate.ModelVersion == region.SelectedModel {
			return candidate.Confidence
		}
	}
	return 0
}

func paperBBoxOverlap(value any, region []float64) bool {
	box, ok := paperBBox(value)
	if !ok || len(region) != 4 {
		return false
	}
	ax2, ay2 := box[0]+box[2], box[1]+box[3]
	bx2, by2 := region[0]+region[2], region[1]+region[3]
	w, h := min(ax2, bx2)-max(box[0], region[0]), min(ay2, by2)-max(box[1], region[1])
	intersection := w * h
	return w > 0 && h > 0 && intersection >= min(box[2]*box[3], region[2]*region[3])*0.5
}

func paperBBox(value any) ([]float64, bool) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, false
	}
	var box []float64
	if json.Unmarshal(raw, &box) != nil || len(box) != 4 || box[2] <= 0 || box[3] <= 0 {
		return nil, false
	}
	return box, true
}

func paperFormulaReplacementSafe(block PaperImportOCRBlock, region PaperImportFormulaRegion) bool {
	box, ok := paperBBox(block.BBox)
	if !ok {
		return false
	}
	ax2, ay2 := box[0]+box[2], box[1]+box[3]
	bx2, by2 := region.BBox[0]+region.BBox[2], region.BBox[1]+region.BBox[3]
	w, h := min(ax2, bx2)-max(box[0], region.BBox[0]), min(ay2, by2)-max(box[1], region.BBox[1])
	coverage := max(0, w) * max(0, h) / (box[2] * box[3])
	if coverage < 0.82 {
		return false
	}
	plainBlock := normalizeFormulaSearchText(block.Text)
	plainFormula := formulaSearchText(region.SelectedLatex)
	if len([]rune(plainFormula)) >= 2 && plainBlock == plainFormula {
		return true
	}
	for _, char := range block.Text {
		if unicode.In(char, unicode.Han) {
			return false
		}
	}
	formulaLike := 0
	total := 0
	for _, char := range plainBlock {
		if unicode.IsSpace(char) {
			continue
		}
		total++
		if unicode.IsDigit(char) || unicode.IsLetter(char) || strings.ContainsRune("+-=*/()[]{}<>.,:|", char) {
			formulaLike++
		}
	}
	return coverage >= 0.94 && total > 0 && float64(formulaLike)/float64(total) >= 0.9
}

func mergeInlineFormulaSegment(block PaperImportOCRBlock, region PaperImportFormulaRegion) (PaperImportOCRBlock, bool) {
	plainFormula := formulaSearchText(region.SelectedLatex)
	if len([]rune(plainFormula)) < 3 {
		return block, false
	}
	searchable, starts, ends := normalizeFormulaSearchWithOffsets(block.Text)
	start := strings.Index(searchable, plainFormula)
	if start < 0 {
		return block, false
	}
	end := start + len(plainFormula)
	if start >= len(starts) || end <= 0 || end > len(ends) {
		return block, false
	}
	sourceStart, sourceEnd := starts[start], ends[end-1]
	if block.RawText == "" {
		block.RawText = block.Text
	}
	block.Text = block.Text[:sourceStart] + "\\(" + region.SelectedLatex + "\\)" + block.Text[sourceEnd:]
	block.Kind = "mixed"
	block.ReviewStatus = "accepted"
	block.Segments = inlineFormulaSegments(block.Text, block.BlockID, block.Segments, region)
	return block, true
}

func inlineFormulaSegments(text, sourceBlockID string, previous []PaperImportContentSegment, current PaperImportFormulaRegion) []PaperImportContentSegment {
	knownBoxes := map[string][]float64{current.SelectedLatex: current.BBox}
	for _, segment := range previous {
		if segment.Kind == "formula" && segment.Latex != "" && len(segment.BBox) == 4 {
			knownBoxes[segment.Latex] = segment.BBox
		}
	}
	segments := []PaperImportContentSegment{}
	for len(text) > 0 {
		start := strings.Index(text, "\\(")
		if start < 0 {
			if text != "" {
				segments = append(segments, PaperImportContentSegment{Kind: "text", Text: text, SourceBlockID: sourceBlockID})
			}
			break
		}
		if start > 0 {
			segments = append(segments, PaperImportContentSegment{Kind: "text", Text: text[:start], SourceBlockID: sourceBlockID})
		}
		end := strings.Index(text[start+2:], "\\)")
		if end < 0 {
			segments = append(segments, PaperImportContentSegment{Kind: "text", Text: text[start:], SourceBlockID: sourceBlockID})
			break
		}
		latex := text[start+2 : start+2+end]
		segments = append(segments, PaperImportContentSegment{Kind: "formula", Latex: latex, BBox: knownBoxes[latex], SourceBlockID: sourceBlockID})
		text = text[start+2+end+2:]
	}
	return segments
}

var formulaCommandPattern = regexp.MustCompile(`\\[A-Za-z]+`)

func formulaSearchText(value string) string {
	value = strings.NewReplacer(
		`\times`, "*", `\cdot`, "*", `\div`, "/", `\leq`, "<=", `\geq`, ">=", `\neq`, "!=",
		`\left`, "", `\right`, "", "^{", "{", "_{", "{",
	).Replace(value)
	value = formulaCommandPattern.ReplaceAllString(value, "")
	value = strings.NewReplacer("{", "", "}", "", "^", "", "_", "").Replace(value)
	return normalizeFormulaSearchText(value)
}

func normalizeFormulaSearchText(value string) string {
	value = strings.ToLower(norm.NFKC.String(value))
	value = strings.NewReplacer("×", "*", "·", "*", "÷", "/", "−", "-", " ", "", "\t", "", "\n", "").Replace(value)
	return value
}

func normalizeFormulaSearchWithOffsets(value string) (string, []int, []int) {
	var builder strings.Builder
	starts := []int{}
	ends := []int{}
	for sourceStart, char := range value {
		sourceEnd := sourceStart + len(string(char))
		normalized := strings.ToLower(norm.NFKC.String(string(char)))
		normalized = strings.NewReplacer("×", "*", "·", "*", "÷", "/", "−", "-").Replace(normalized)
		for _, output := range normalized {
			if unicode.IsSpace(output) {
				continue
			}
			encoded := string(output)
			builder.WriteString(encoded)
			for range len(encoded) {
				starts = append(starts, sourceStart)
				ends = append(ends, sourceEnd)
			}
		}
	}
	return builder.String(), starts, ends
}

func paperFormulaGeometryInsertAt(blocks []PaperImportOCRBlock, region PaperImportFormulaRegion) int {
	for index, block := range blocks {
		if block.PageNo != region.PageNo {
			if block.PageNo > region.PageNo {
				return index
			}
			continue
		}
		box, ok := paperBBox(block.BBox)
		if ok && (box[1] > region.BBox[1] || (box[1] == region.BBox[1] && box[0] > region.BBox[0])) {
			return index
		}
	}
	return len(blocks)
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
	if task.TaskType == "paper_formula" {
		detail, _ := json.Marshal(map[string]any{"error_code": input.ErrorCode, "error_detail": input.ErrorDetail})
		if _, err = tx.ExecContext(ctx, `UPDATE paper_import_formula_input SET status='failed',result=$4::jsonb,completed_at=now() WHERE tenant_id=$1 AND paper_import_id=$2::uuid AND run_id=$3::uuid`, tenantID, importID, runID, detail); err != nil {
			return err
		}
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
	case "paper_formula_failed":
		return "数学公式识别失败，请稍后重试"
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
