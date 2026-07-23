import { apiClient } from "./client";

export interface DependencyStatus {
  name: string;
  status: "ok" | "error" | "not_configured";
  error?: string;
  detail?: string;
  duration_ms: number;
  checked_at: string;
}

export interface ObservabilityStatus {
  log_format: string;
  system_log_stream: string;
  audit_log_stream: string;
  request_id_header: string;
  trace_id_header: string;
  slow_request_threshold_ms: number;
  slow_query_log: string;
  sensitive_log_policy: string;
}

export interface WorkerServiceStatus {
  name: string;
  worker_service: string;
  queue_name: string;
  status: "ok" | "error";
  availability: "online" | "stale" | "unavailable";
  automation_available: boolean;
  fresh_instances: number;
  stale_instances: number;
  last_seen_at?: string;
  stale_after_sec: number;
  queued_tasks: number;
  in_flight_tasks: number;
  dead_letter_tasks: number;
  failed_last_hour: number;
  impact_code: string;
  impact: string;
  action: string;
}

export interface SystemStatus {
  status: "healthy" | "degraded";
  service: string;
  environment: string;
  version: string;
  started_at: string;
  generated_at: string;
  uptime_sec: number;
  dependencies: DependencyStatus[];
  worker_services?: WorkerServiceStatus[];
  observability: ObservabilityStatus;
}

export interface OcrAvailability {
  generated_at: string;
  worker: WorkerServiceStatus;
}

export function getSystemStatus() {
  return apiClient.request<SystemStatus>("/api/v1/system/status");
}

export function getOcrAvailability() {
  return apiClient.request<OcrAvailability>("/api/v1/ocr/availability");
}
