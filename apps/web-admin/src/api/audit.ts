import { apiClient } from "./client";

export interface AuditLog {
  id: string;
  tenant_id: string;
  actor_id?: string;
  action: string;
  target_type: string;
  target_id?: string;
  before_value?: Record<string, unknown>;
  after_value?: Record<string, unknown>;
  reason?: string;
  ip_address?: string;
  user_agent?: string;
  request_id?: string;
  created_at: string;
}

export interface AuditLogFilter {
  action?: string;
  actor_id?: string;
  target_type?: string;
  target_id?: string;
  exam_id?: string;
  ip_address?: string;
  created_from?: string;
  created_to?: string;
  limit?: number;
}

function queryString(filter: AuditLogFilter) {
  const params = new URLSearchParams();
  if (filter.action) {
    params.set("action", filter.action);
  }
  if (filter.actor_id) {
    params.set("actor_id", filter.actor_id);
  }
  if (filter.target_type) {
    params.set("target_type", filter.target_type);
  }
  if (filter.target_id) {
    params.set("target_id", filter.target_id);
  }
  if (filter.exam_id) {
    params.set("exam_id", filter.exam_id);
  }
  if (filter.ip_address) {
    params.set("ip_address", filter.ip_address);
  }
  if (filter.created_from) {
    params.set("created_from", filter.created_from);
  }
  if (filter.created_to) {
    params.set("created_to", filter.created_to);
  }
  if (filter.limit) {
    params.set("limit", String(filter.limit));
  }
  const query = params.toString();
  return query ? `?${query}` : "";
}

export async function listAuditLogs(filter: AuditLogFilter = {}) {
  return apiClient.request<{ audit_logs: AuditLog[] }>(`/api/v1/audit-logs${queryString(filter)}`);
}

export async function exportAuditLogs(filter: AuditLogFilter = {}) {
  return apiClient.requestBlob(`/api/v1/audit-logs/export${queryString(filter)}`, { method: "POST" });
}
