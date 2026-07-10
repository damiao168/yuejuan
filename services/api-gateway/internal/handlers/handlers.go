package handlers

import (
	"net/http"
	"time"

	"edugrade-enterprise/services/api-gateway/internal/config"
	"edugrade-enterprise/services/api-gateway/internal/deps"
	"edugrade-enterprise/services/api-gateway/internal/httpx"
)

type Handlers struct {
	cfg       config.Config
	checkers  []deps.Checker
	startedAt time.Time
}

func New(cfg config.Config, checkers []deps.Checker) *Handlers {
	return &Handlers{cfg: cfg, checkers: checkers, startedAt: time.Now().UTC()}
}

func (h *Handlers) Health(w http.ResponseWriter, _ *http.Request) {
	httpx.JSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"health":  "healthy",
		"service": h.cfg.Service.Name,
	})
}

func (h *Handlers) Ready(w http.ResponseWriter, r *http.Request) {
	results, ok := deps.CheckAll(r.Context(), h.cfg.Service.ReadinessTimeout, h.checkers)
	status := http.StatusOK
	overall := "ready"
	if !ok {
		status = http.StatusServiceUnavailable
		overall = "not_ready"
	}
	httpx.JSON(w, status, map[string]any{
		"status":       overall,
		"dependencies": results,
	})
}

func (h *Handlers) SystemStatus(w http.ResponseWriter, r *http.Request) {
	results, ok := deps.CheckAll(r.Context(), h.cfg.Service.ReadinessTimeout, h.checkers)
	overall := "healthy"
	if !ok {
		overall = "degraded"
	}
	now := time.Now().UTC()
	httpx.JSON(w, http.StatusOK, map[string]any{
		"status":       overall,
		"service":      h.cfg.Service.Name,
		"environment":  h.cfg.Service.Environment,
		"version":      "0.1.0",
		"started_at":   h.startedAt.Format(time.RFC3339),
		"generated_at": now.Format(time.RFC3339),
		"uptime_sec":   int64(now.Sub(h.startedAt).Seconds()),
		"dependencies": results,
		"observability": map[string]any{
			"log_format":                "json",
			"system_log_stream":         "stdout",
			"audit_log_stream":          "audit_log API / database records",
			"request_id_header":         "X-Request-ID",
			"trace_id_header":           "X-Trace-ID",
			"slow_request_threshold_ms": h.cfg.Observability.SlowRequestThreshold.Milliseconds(),
			"slow_query_log":            "placeholder_not_implemented",
			"sensitive_log_policy":      "普通系统日志脱敏 answer、student、score、token、secret 等字段；审计日志记录业务动作但不替代答卷内容存储。",
		},
	})
}

func (h *Handlers) SystemInfo(w http.ResponseWriter, _ *http.Request) {
	httpx.JSON(w, http.StatusOK, map[string]any{
		"service":     h.cfg.Service.Name,
		"environment": h.cfg.Service.Environment,
		"version":     "0.1.0",
		"started_at":  h.startedAt.Format(time.RFC3339),
		"capabilities": []string{
			"health_check",
			"readiness_check",
			"structured_logging",
			"request_id",
			"trace_id",
			"system_status_api",
			"dependency_status_checks",
			"slow_request_log_placeholder",
			"session_auth",
			"rbac_middleware",
			"login_audit",
			"tenant_management",
			"organization_management",
			"student_csv_import",
			"exam_management",
			"paper_metadata",
			"question_config",
			"rubric_versioning",
			"paper_config_validation",
			"file_upload",
			"object_storage",
			"private_file_download",
			"file_hash_deduplication",
			"submission_collection",
			"submission_pages",
			"submission_quality_gate",
			"ocr_task_management",
			"ocr_engine_inference",
			"ocr_result_ingestion",
			"ocr_low_confidence_review_trigger",
			"answer_segmentation_metadata",
			"answer_segment_manual_review",
			"agent_orchestration_control_plane",
			"agent_task_management",
			"agent_task_retry",
			"agent_human_review_trigger",
			"agent_worker_runtime",
			"answer_segment_answer_capture",
			"rule_based_objective_grading",
			"ai_grade_recording",
			"grading_low_confidence_review_trigger",
			"subjective_ai_grading_interface",
			"mock_llm_grading_adapter",
			"subjective_ai_grade_failure_recording",
			"rule_based_evidence_verification",
			"evidence_agent_job_recording",
			"evidence_failure_review_trigger",
			"human_review_task_management",
			"human_grade_recording",
			"review_assignment_workflow",
			"double_mark_policy_config",
			"double_mark_review_sessions",
			"arbitration_task_management",
			"final_grade_recording",
			"submission_grade_aggregation",
			"grade_confirmation_workflow",
			"grade_publish_quality_gate",
			"published_student_grade_lookup",
			"grade_csv_export_with_watermark",
			"student_appeal_submission",
			"appeal_review_workflow",
			"score_adjustment_audit_trail",
			"appeal_statistics",
			"student_learning_report",
			"exam_report_overview",
			"class_learning_report",
			"question_item_analysis",
			"grading_quality_report",
			"report_csv_export",
		},
		"not_implemented": []string{
			"real_subjective_model_inference",
			"semantic_evidence_verification",
			"visual_evidence_verification",
			"ai_grading",
		},
	})
}

func (h *Handlers) NotFound(w http.ResponseWriter, r *http.Request) {
	httpx.Error(w, r, http.StatusNotFound, "not_found", "route not found")
}
