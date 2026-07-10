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

export interface SystemStatus {
  status: "healthy" | "degraded";
  service: string;
  environment: string;
  version: string;
  started_at: string;
  generated_at: string;
  uptime_sec: number;
  dependencies: DependencyStatus[];
  observability: ObservabilityStatus;
}

export function getSystemStatus() {
  return apiClient.request<SystemStatus>("/api/v1/system/status");
}
