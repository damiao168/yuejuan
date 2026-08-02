package handlers

import (
	"fmt"
	"net/http"
	"time"

	"edugrade-enterprise/services/api-gateway/internal/auth"
	"edugrade-enterprise/services/api-gateway/internal/config"
	"edugrade-enterprise/services/api-gateway/internal/deps"
	"edugrade-enterprise/services/api-gateway/internal/httpx"
	"edugrade-enterprise/services/api-gateway/internal/workerruntime"
)

type Handlers struct {
	cfg       config.Config
	checkers  []deps.Checker
	startedAt time.Time
	runtime   workerruntime.Store
}

func New(cfg config.Config, checkers []deps.Checker) *Handlers {
	return &Handlers{cfg: cfg, checkers: checkers, startedAt: time.Now().UTC()}
}

func (h *Handlers) WithWorkerRuntimeStore(store workerruntime.Store) *Handlers {
	h.runtime = store
	return h
}

type WorkerServiceStatus struct {
	Name                string `json:"name"`
	WorkerService       string `json:"worker_service"`
	QueueName           string `json:"queue_name"`
	Status              string `json:"status"`
	Availability        string `json:"availability"`
	AutomationAvailable bool   `json:"automation_available"`
	FreshInstances      int    `json:"fresh_instances"`
	StaleInstances      int    `json:"stale_instances"`
	LastSeenAt          string `json:"last_seen_at,omitempty"`
	StaleAfterSec       int64  `json:"stale_after_sec"`
	QueuedTasks         int    `json:"queued_tasks"`
	InFlightTasks       int    `json:"in_flight_tasks"`
	DeadLetterTasks     int    `json:"dead_letter_tasks"`
	FailedLastHour      int    `json:"failed_last_hour"`
	ImpactCode          string `json:"impact_code"`
	Impact              string `json:"impact"`
	Action              string `json:"action"`
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
	now := time.Now().UTC()
	workerServices := make([]WorkerServiceStatus, 0, 1)
	workerCheckStartedAt := time.Now()
	workerStatus, workerErr := h.ocrWorkerStatus(r, now)
	workerServices = append(workerServices, workerStatus)
	workerDependency := deps.CheckResult{
		Name:       workerStatus.Name,
		Status:     workerStatus.Status,
		Detail:     workerStatusDetail(workerStatus, workerErr),
		DurationMS: time.Since(workerCheckStartedAt).Milliseconds(),
		CheckedAt:  now.Format(time.RFC3339),
	}
	results = append(results, workerDependency)
	if workerStatus.Status != "ok" {
		ok = false
	}
	overall := "healthy"
	if !ok {
		overall = "degraded"
	}
	httpx.JSON(w, http.StatusOK, map[string]any{
		"status":          overall,
		"service":         h.cfg.Service.Name,
		"environment":     h.cfg.Service.Environment,
		"version":         "0.1.0",
		"started_at":      h.startedAt.Format(time.RFC3339),
		"generated_at":    now.Format(time.RFC3339),
		"uptime_sec":      int64(now.Sub(h.startedAt).Seconds()),
		"dependencies":    results,
		"worker_services": workerServices,
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

// OCRAvailability exposes only the tenant-scoped OCR automation state needed by
// capture and review workflows. Detailed dependency and observability data stay
// behind the system:read-protected SystemStatus endpoint.
func (h *Handlers) OCRAvailability(w http.ResponseWriter, r *http.Request) {
	now := time.Now().UTC()
	workerStatus, _ := h.ocrWorkerStatus(r, now)
	httpx.JSON(w, http.StatusOK, map[string]any{
		"generated_at": now.Format(time.RFC3339),
		"worker":       workerStatus,
	})
}

func (h *Handlers) ocrWorkerStatus(r *http.Request, now time.Time) (WorkerServiceStatus, error) {
	staleAfter := h.cfg.Observability.WorkerHeartbeatStaleAfter
	if staleAfter <= 0 {
		staleAfter = 30 * time.Second
	}
	if h.runtime == nil {
		return buildWorkerServiceStatus(workerruntime.Metrics{}, "ocr_worker", "ocr-worker", "ocr", now, staleAfter), fmt.Errorf("worker runtime store is unavailable")
	}
	user, _ := auth.UserFromContext(r.Context())
	metrics, err := h.runtime.MetricsAcrossTenants(r.Context(), auth.PlatformTenantID)
	if err != nil {
		return buildWorkerServiceStatus(workerruntime.Metrics{}, "ocr_worker", "ocr-worker", "ocr", now, staleAfter), err
	}
	if user.TenantID != auth.PlatformTenantID {
		scoped, scopedErr := h.runtime.Metrics(r.Context(), user.TenantID)
		if scopedErr != nil {
			return buildWorkerServiceStatus(workerruntime.Metrics{}, "ocr_worker", "ocr-worker", "ocr", now, staleAfter), scopedErr
		}
		scoped.Workers = metrics.Workers
		metrics = scoped
	}
	return buildWorkerServiceStatus(metrics, "ocr_worker", "ocr-worker", "ocr", now, staleAfter), nil
}

func buildWorkerServiceStatus(metrics workerruntime.Metrics, name string, workerService string, queueName string, now time.Time, staleAfter time.Duration) WorkerServiceStatus {
	status := WorkerServiceStatus{
		Name: name, WorkerService: workerService, QueueName: queueName,
		Status: "error", Availability: "unavailable", StaleAfterSec: int64(staleAfter.Seconds()),
		ImpactCode: "ocr_automation_unavailable",
		Impact:     "新 OCR 任务不会被自动处理；选择题 OMR 不受影响，依赖 OCR 的填空题自动批阅暂停，任务会保留在队列并可转人工处理。",
		Action:     "启动或恢复 OCR Worker，检查服务账号与 API 连通性；恢复后队列会自动继续处理。",
	}
	var latest time.Time
	for _, worker := range metrics.Workers {
		if worker.WorkerService != workerService || worker.QueueName != queueName {
			continue
		}
		if worker.LastSeenAt.After(latest) {
			latest = worker.LastSeenAt
		}
		if now.Sub(worker.LastSeenAt) <= staleAfter {
			status.FreshInstances++
		} else {
			status.StaleInstances++
		}
	}
	for _, queue := range metrics.Queues {
		if queue.QueueName != queueName {
			continue
		}
		status.QueuedTasks = queue.Queued
		status.InFlightTasks = queue.Leased + queue.Running
		status.DeadLetterTasks = queue.DeadLetter
		status.FailedLastHour = queue.FailedLastHour
	}
	if !latest.IsZero() {
		status.LastSeenAt = latest.UTC().Format(time.RFC3339)
	}
	if status.FreshInstances > 0 {
		status.Status = "ok"
		status.Availability = "online"
		status.AutomationAvailable = true
		status.ImpactCode = "ocr_automation_available"
		status.Impact = "OCR 图像识别可处理新任务；自动判分仍受置信度阈值与评分规则控制，低置信度结果继续进入人工复核。"
		status.Action = "无需处理。"
	} else if status.StaleInstances > 0 {
		status.Availability = "stale"
	}
	return status
}

func workerStatusDetail(status WorkerServiceStatus, statusErr error) string {
	detail := fmt.Sprintf("%s；影响：%s；处理：%s", status.Availability, status.Impact, status.Action)
	if statusErr != nil {
		return detail + "；状态读取错误：" + statusErr.Error()
	}
	return detail
}

func (h *Handlers) SystemInfo(w http.ResponseWriter, _ *http.Request) {
	modelCapability := "mock_llm_grading_adapter"
	notImplemented := []string{
		"real_subjective_model_inference",
		"semantic_evidence_verification",
		"visual_evidence_verification",
		"ai_grading",
	}
	if h.cfg.AIService.URL != "" {
		modelCapability = "governed_shadow_grading_agent"
		notImplemented = []string{
			"calibrated_subjective_model_confidence",
			"semantic_evidence_verification",
			"visual_evidence_verification",
			"automatic_subjective_grade_acceptance",
		}
	}
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
			modelCapability,
			"subjective_ai_grade_failure_recording",
			"model_governance_api",
			"model_secret_reference_probe",
			"local_model_baseline_registry",
			"offline_model_evaluation",
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
		"not_implemented": notImplemented,
	})
}

func (h *Handlers) NotFound(w http.ResponseWriter, r *http.Request) {
	httpx.Error(w, r, http.StatusNotFound, "not_found", "route not found")
}
