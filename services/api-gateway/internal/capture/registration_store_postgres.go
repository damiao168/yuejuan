package capture

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
)

type templateLayout struct {
	Pages []struct {
		PageNo          int              `json:"page_no"`
		Width           int              `json:"width"`
		Height          int              `json:"height"`
		QuestionRegions []map[string]any `json:"question_regions"`
	} `json:"pages"`
}

func (s *PostgresStore) QueueSubmissionPages(ctx context.Context, tenantID, submissionID, actorID string) ([]RegistrationRun, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var examID, batchID string
	if err = tx.QueryRowContext(ctx, `SELECT s.exam_id::text,cp.capture_batch_id::text FROM submission s JOIN capture_page cp ON cp.tenant_id=s.tenant_id AND cp.submission_id=s.id AND cp.deleted_at IS NULL WHERE s.tenant_id=$1 AND s.id=$2::uuid AND s.deleted_at IS NULL LIMIT 1`, tenantID, submissionID).Scan(&examID, &batchID); err != nil {
		return nil, mapNotFound(err)
	}
	if err = ensureBatchWritableTx(ctx, tx, tenantID, batchID); err != nil {
		return nil, err
	}
	var templateID, templateHash, templateAssetID, templateContentType string
	var layoutRaw []byte
	err = tx.QueryRowContext(ctx, `SELECT ast.id::text,ast.content_hash,ast.layout,ep.file_asset_id::text,fa.content_type
FROM answer_sheet_template ast
JOIN exam_paper ep ON ep.tenant_id=ast.tenant_id AND ep.id=ast.exam_paper_id
JOIN file_asset fa ON fa.tenant_id=ep.tenant_id AND fa.id=ep.file_asset_id
WHERE ast.tenant_id=$1 AND ast.exam_id=$2::uuid AND ast.status='locked' AND ast.deleted_at IS NULL
ORDER BY ast.version_no DESC LIMIT 1`, tenantID, examID).Scan(&templateID, &templateHash, &layoutRaw, &templateAssetID, &templateContentType)
	if err != nil {
		return nil, mapNotFound(err)
	}
	var layout templateLayout
	if json.Unmarshal(layoutRaw, &layout) != nil || len(layout.Pages) == 0 {
		return nil, ErrInvalidInput
	}
	rows, err := tx.QueryContext(ctx, `SELECT cp.id::text,cp.submission_page_id::text,sp.normalized_file_asset_id::text,cp.assigned_page_no,cp.revision,fa.hash_sha256
	FROM capture_page cp
	JOIN submission_page sp ON sp.tenant_id=cp.tenant_id AND sp.id=cp.submission_page_id
	JOIN file_asset fa ON fa.tenant_id=sp.tenant_id AND fa.id=sp.normalized_file_asset_id
	WHERE cp.tenant_id=$1 AND cp.submission_id=$2::uuid AND cp.status='normalized'
	  AND sp.quality_status='passed' AND sp.normalized_file_asset_id IS NOT NULL
	  AND cp.deleted_at IS NULL ORDER BY cp.sequence_no FOR UPDATE`, tenantID, submissionID)
	if err != nil {
		return nil, err
	}
	type pageRow struct {
		id, submissionPageID, assetID, hash string
		pageNo, revision                    int
	}
	pages := []pageRow{}
	for rows.Next() {
		var p pageRow
		if err = rows.Scan(&p.id, &p.submissionPageID, &p.assetID, &p.pageNo, &p.revision, &p.hash); err != nil {
			rows.Close()
			return nil, err
		}
		pages = append(pages, p)
	}
	if err = rows.Close(); err != nil {
		return nil, err
	}
	if len(pages) == 0 {
		return nil, ErrNotFound
	}
	out := []RegistrationRun{}
	for _, page := range pages {
		var templatePage *struct {
			PageNo          int              `json:"page_no"`
			Width           int              `json:"width"`
			Height          int              `json:"height"`
			QuestionRegions []map[string]any `json:"question_regions"`
		}
		for i := range layout.Pages {
			if layout.Pages[i].PageNo == page.pageNo {
				templatePage = &layout.Pages[i]
				break
			}
		}
		if templatePage == nil {
			_, _ = tx.ExecContext(ctx, `UPDATE capture_page SET status='needs_review',updated_at=now() WHERE tenant_id=$1 AND id=$2::uuid`, tenantID, page.id)
			continue
		}
		if existing, findErr := scanRegistrationRun(tx.QueryRowContext(ctx, `SELECT `+registrationColumns+` FROM page_registration_run WHERE tenant_id=$1 AND capture_page_id=$2::uuid AND source_sha256=$3 AND template_content_hash=$4 AND page_no=$5 AND processing_status IN ('processing','completed') AND deleted_at IS NULL ORDER BY created_at DESC LIMIT 1`, tenantID, page.id, page.hash, templateHash, page.pageNo)); findErr == nil {
			out = append(out, existing)
			continue
		} else if !errors.Is(findErr, ErrNotFound) {
			return nil, findErr
		}
		var runID string
		err = tx.QueryRowContext(ctx, `INSERT INTO page_registration_run (tenant_id,capture_page_id,submission_page_id,source_file_asset_id,source_sha256,template_id,template_content_hash,page_no,processing_status,profile_version,started_at)
VALUES ($1,$2::uuid,$3::uuid,$4::uuid,$5,$6::uuid,$7,$8,'processing','opencv-orb-akaze-v1',now()) RETURNING id::text`, tenantID, page.id, page.submissionPageID, page.assetID, page.hash, templateID, templateHash, page.pageNo).Scan(&runID)
		if err != nil {
			return nil, err
		}
		payload, _ := json.Marshal(map[string]any{
			"registration_run_id": runID, "capture_page_id": page.id, "exam_id": examID,
			"source_file_asset_id": page.assetID, "source_download_url": "/api/v1/files/" + page.assetID + "/download", "source_content_type": "image/png",
			"template_id": templateID, "template_content_hash": templateHash, "template_file_asset_id": templateAssetID, "template_download_url": "/api/v1/files/" + templateAssetID + "/download", "template_content_type": templateContentType,
			"page_no": page.pageNo, "template_page_index": page.pageNo, "template_width": templatePage.Width, "template_height": templatePage.Height, "question_regions": templatePage.QuestionRegions, "render_dpi": 300,
		})
		key := "page-registration:" + page.id + ":" + templateHash + ":r" + strconv.Itoa(page.revision)
		var taskID string
		err = tx.QueryRowContext(ctx, `INSERT INTO agent_worker_task (tenant_id,task_type,queue_name,source_type,source_id,priority,payload,payload_schema_version,idempotency_key,dedupe_key,max_attempts,retry_backoff_seconds,created_by)
VALUES ($1,'page_registration','page-processing','page_registration_run',$2::uuid,60,$3,'page-registration-v1',$4,$4,3,10,$5::uuid) RETURNING id::text`, tenantID, runID, payload, key, actorID).Scan(&taskID)
		if err != nil {
			return nil, err
		}
		if _, err = tx.ExecContext(ctx, `UPDATE page_registration_run SET runtime_task_id=$3::uuid,updated_at=now() WHERE tenant_id=$1 AND id=$2::uuid`, tenantID, runID, taskID); err != nil {
			return nil, err
		}
		if _, err = tx.ExecContext(ctx, `UPDATE capture_page SET status='registration',updated_at=now() WHERE tenant_id=$1 AND id=$2::uuid`, tenantID, page.id); err != nil {
			return nil, err
		}
		run, scanErr := scanRegistrationRun(tx.QueryRowContext(ctx, `SELECT `+registrationColumns+` FROM page_registration_run WHERE tenant_id=$1 AND id=$2::uuid`, tenantID, runID))
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, run)
	}
	if len(out) == 0 {
		return nil, ErrInvalidTransition
	}
	return out, tx.Commit()
}

func (s *PostgresStore) GetRegistrationRun(ctx context.Context, tenantID, runID string) (RegistrationRun, error) {
	return scanRegistrationRun(s.db.QueryRowContext(ctx, `SELECT `+registrationColumns+` FROM page_registration_run WHERE tenant_id=$1 AND id=$2::uuid AND deleted_at IS NULL`, tenantID, runID))
}

func (s *PostgresStore) ListRegistrationRuns(ctx context.Context, tenantID, submissionPageID string) ([]RegistrationRun, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+registrationColumns+` FROM page_registration_run WHERE tenant_id=$1 AND submission_page_id=$2::uuid AND deleted_at IS NULL ORDER BY created_at DESC`, tenantID, submissionPageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []RegistrationRun{}
	for rows.Next() {
		item, scanErr := scanRegistrationRun(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *PostgresStore) ApplyRegistrationResult(ctx context.Context, tenantID, runID string, input RegistrationResultInput) (RegistrationRun, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return RegistrationRun{}, err
	}
	defer tx.Rollback()
	current, err := scanRegistrationRun(tx.QueryRowContext(ctx, `SELECT `+registrationColumns+` FROM page_registration_run WHERE tenant_id=$1 AND id=$2::uuid AND deleted_at IS NULL FOR UPDATE`, tenantID, runID))
	if err != nil {
		return RegistrationRun{}, err
	}
	if current.ProcessingStatus == "completed" {
		return current, tx.Commit()
	}
	if current.ProcessingStatus != "processing" && current.ProcessingStatus != "retryable_error" {
		return RegistrationRun{}, ErrInvalidTransition
	}
	var submissionID string
	if err = tx.QueryRowContext(ctx, `SELECT submission_id::text FROM capture_page WHERE tenant_id=$1 AND id=$2::uuid`, tenantID, current.CapturePageID).Scan(&submissionID); err != nil {
		return RegistrationRun{}, err
	}
	for _, segment := range input.Segments {
		if segment.QuestionID == "" || segment.FileAssetID == "" || segment.SHA256 == "" {
			return RegistrationRun{}, ErrInvalidInput
		}
		var questionNo string
		if err = tx.QueryRowContext(ctx, `SELECT question_no FROM question WHERE tenant_id=$1 AND id=$2::uuid AND deleted_at IS NULL`, tenantID, segment.QuestionID).Scan(&questionNo); err != nil {
			return RegistrationRun{}, mapNotFound(err)
		}
		normalized, _ := json.Marshal(segment.NormalizedBBox)
		pixels, _ := json.Marshal(segment.PixelBBox)
		status := "generated"
		if input.Confidence < 0.75 {
			status = "needs_manual_review"
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO answer_segment (tenant_id,submission_id,submission_page_id,question_id,question_no,bbox,source,status,template_id,template_content_hash,registration_run_id,normalized_bbox,pixel_bbox,crop_file_asset_id,crop_sha256,question_version,processing_status,confidence)
VALUES ($1,$2::uuid,$3::uuid,$4::uuid,$5,$6,'configured_answer_area',$7,$8::uuid,$9,$10::uuid,$6,$11,$12::uuid,$13,1,'completed',$14)
ON CONFLICT (tenant_id,submission_id,question_id) DO UPDATE SET submission_page_id=EXCLUDED.submission_page_id,bbox=EXCLUDED.bbox,status=EXCLUDED.status,template_id=EXCLUDED.template_id,template_content_hash=EXCLUDED.template_content_hash,registration_run_id=EXCLUDED.registration_run_id,normalized_bbox=EXCLUDED.normalized_bbox,pixel_bbox=EXCLUDED.pixel_bbox,crop_file_asset_id=EXCLUDED.crop_file_asset_id,crop_sha256=EXCLUDED.crop_sha256,question_version=EXCLUDED.question_version,processing_status='completed',confidence=EXCLUDED.confidence,deleted_at=NULL,updated_at=now()`, tenantID, submissionID, current.SubmissionPageID, segment.QuestionID, questionNo, normalized, status, current.TemplateID, current.TemplateContentHash, current.ID, pixels, segment.FileAssetID, segment.SHA256, input.Confidence)
		if err != nil {
			return RegistrationRun{}, err
		}
	}
	matchStatus := "matched"
	pageStatus := "ready"
	if input.Confidence < 0.75 {
		matchStatus = "needs_review"
		pageStatus = "needs_review"
	}
	sourceMatrix, _ := json.Marshal(input.SourceToTemplate)
	inverseMatrix, _ := json.Marshal(input.TemplateToSource)
	row := tx.QueryRowContext(ctx, `UPDATE page_registration_run SET processing_status='completed',match_status=$3,confidence=$4,method=$5,source_to_template_matrix=$6,template_to_source_matrix=$7,feature_count=$8,match_count=$9,inlier_count=$10,inlier_ratio=$11,reprojection_error=$12,registered_file_asset_id=$13::uuid,result_version=$14,duration_ms=$15,completed_at=now(),error_code=NULL,error_detail='{}',updated_at=now() WHERE tenant_id=$1 AND id=$2::uuid RETURNING `+registrationColumns, tenantID, runID, matchStatus, input.Confidence, input.Method, sourceMatrix, inverseMatrix, input.FeatureCount, input.MatchCount, input.InlierCount, input.InlierRatio, input.ReprojectionError, input.RegisteredFileAssetID, input.ResultVersion, input.DurationMS)
	out, err := scanRegistrationRun(row)
	if err != nil {
		return RegistrationRun{}, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE capture_page SET status=$3,revision=revision+1,updated_at=now() WHERE tenant_id=$1 AND id=$2::uuid`, tenantID, current.CapturePageID, pageStatus); err != nil {
		return RegistrationRun{}, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE capture_batch b SET
review_count=(SELECT count(*) FROM capture_page p WHERE p.tenant_id=b.tenant_id AND p.capture_batch_id=b.id AND p.status IN ('needs_review','quality_rejected','failed') AND p.deleted_at IS NULL),
normal_count=(SELECT count(*) FROM capture_page p WHERE p.tenant_id=b.tenant_id AND p.capture_batch_id=b.id AND p.status='ready' AND p.deleted_at IS NULL),
status=CASE
  WHEN EXISTS(SELECT 1 FROM capture_page p WHERE p.tenant_id=b.tenant_id AND p.capture_batch_id=b.id AND p.status IN ('needs_review','quality_rejected','failed') AND p.deleted_at IS NULL) THEN 'needs_review'
  WHEN NOT EXISTS(SELECT 1 FROM capture_page p WHERE p.tenant_id=b.tenant_id AND p.capture_batch_id=b.id AND p.status<>'ready' AND p.deleted_at IS NULL) THEN 'ready'
  ELSE 'processing' END,
revision=revision+1,updated_at=now()
WHERE b.tenant_id=$1 AND b.id=(SELECT capture_batch_id FROM capture_page WHERE tenant_id=$1 AND id=$2::uuid)`, tenantID, current.CapturePageID); err != nil {
		return RegistrationRun{}, err
	}
	return out, tx.Commit()
}

func (s *PostgresStore) ApplyRegistrationFailure(ctx context.Context, tenantID, runID, errorCode string, errorDetail map[string]any, retryable bool) (RegistrationRun, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return RegistrationRun{}, err
	}
	defer tx.Rollback()
	status := "terminal_error"
	pageStatus := "needs_review"
	if retryable {
		status = "retryable_error"
		pageStatus = "registration"
	}
	detail, _ := json.Marshal(errorDetail)
	out, err := scanRegistrationRun(tx.QueryRowContext(ctx, `UPDATE page_registration_run SET processing_status=$3,error_code=$4,error_detail=$5,updated_at=now() WHERE tenant_id=$1 AND id=$2::uuid AND deleted_at IS NULL RETURNING `+registrationColumns, tenantID, runID, status, errorCode, detail))
	if err != nil {
		return RegistrationRun{}, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE capture_page SET status=$3,revision=revision+1,updated_at=now() WHERE tenant_id=$1 AND id=$2::uuid`, tenantID, out.CapturePageID, pageStatus); err != nil {
		return RegistrationRun{}, err
	}
	var batchID string
	if err = tx.QueryRowContext(ctx, `SELECT capture_batch_id::text FROM capture_page WHERE tenant_id=$1 AND id=$2::uuid`, tenantID, out.CapturePageID).Scan(&batchID); err != nil {
		return RegistrationRun{}, err
	}
	if err = s.aggregateBatchTx(ctx, tx, tenantID, batchID); err != nil {
		return RegistrationRun{}, err
	}
	return out, tx.Commit()
}

const registrationColumns = `id::text,capture_page_id::text,submission_page_id::text,source_file_asset_id::text,template_id::text,template_content_hash,page_no,processing_status,COALESCE(match_status,''),COALESCE(confidence,0),COALESCE(method,''),profile_version,source_to_template_matrix,template_to_source_matrix,feature_count,match_count,inlier_count,COALESCE(inlier_ratio,0),COALESCE(reprojection_error,0),COALESCE(registered_file_asset_id::text,''),COALESCE(runtime_task_id::text,''),COALESCE(error_code,''),created_at`

func scanRegistrationRun(row scanner) (RegistrationRun, error) {
	var item RegistrationRun
	var source, inverse []byte
	err := row.Scan(&item.ID, &item.CapturePageID, &item.SubmissionPageID, &item.SourceFileAssetID, &item.TemplateID, &item.TemplateContentHash, &item.PageNo, &item.ProcessingStatus, &item.MatchStatus, &item.Confidence, &item.Method, &item.ProfileVersion, &source, &inverse, &item.FeatureCount, &item.MatchCount, &item.InlierCount, &item.InlierRatio, &item.ReprojectionError, &item.RegisteredFileAssetID, &item.RuntimeTaskID, &item.ErrorCode, &item.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return RegistrationRun{}, ErrNotFound
		}
		return RegistrationRun{}, err
	}
	_ = json.Unmarshal(source, &item.SourceToTemplate)
	_ = json.Unmarshal(inverse, &item.TemplateToSource)
	if item.SourceToTemplate == nil {
		item.SourceToTemplate = []any{}
	}
	if item.TemplateToSource == nil {
		item.TemplateToSource = []any{}
	}
	return item, nil
}
