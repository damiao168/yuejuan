import { apiClient } from "./client";

export type ManagedAdapterType = "openai_compatible" | "dashscope_native";
export type ManagedAPIConfigStatus = "active" | "disabled";
export type ManagedAPIProbeStatus = "untested" | "success" | "failed";

export interface ManagedModelAPIConfig {
  id: string;
  tenant_id: string;
  provider_key: string;
  display_name: string;
  adapter_type: ManagedAdapterType;
  base_url: string;
  model_name: string;
  model_version: string;
  region: string;
  credential_configured: boolean;
  credential_hint?: string;
  status: ManagedAPIConfigStatus;
  is_default: boolean;
  last_test_status: ManagedAPIProbeStatus;
  last_test_message?: string;
  last_tested_at?: string;
  created_at: string;
  updated_at: string;
}

export interface ManagedModelAPIConfigInput {
  tenant_id: string;
  provider_key: string;
  display_name: string;
  adapter_type: ManagedAdapterType;
  base_url: string;
  model_name: string;
  model_version: string;
  region: string;
  api_key: string;
  status: ManagedAPIConfigStatus;
  is_default: boolean;
}

export type ManagedModelAPIConfigUpdateInput = Omit<ManagedModelAPIConfigInput, "tenant_id" | "provider_key">;

export interface ManagedAPIProbeResult {
  ok: boolean;
  status_code?: number;
  latency_ms: number;
  message: string;
}

function tenantQuery(tenantId: string) {
  return `?tenant_id=${encodeURIComponent(tenantId)}`;
}

export function listManagedModelAPIConfigs(tenantId: string) {
  return apiClient.request<{ configs: ManagedModelAPIConfig[] }>(`/api/v1/platform/model-api-configs${tenantQuery(tenantId)}`);
}

export function createManagedModelAPIConfig(input: ManagedModelAPIConfigInput) {
  return apiClient.request<{ config: ManagedModelAPIConfig }>("/api/v1/platform/model-api-configs", {
    method: "POST",
    body: JSON.stringify(input)
  });
}

export function updateManagedModelAPIConfig(id: string, tenantId: string, input: ManagedModelAPIConfigUpdateInput) {
  return apiClient.request<{ config: ManagedModelAPIConfig }>(`/api/v1/platform/model-api-configs/${encodeURIComponent(id)}${tenantQuery(tenantId)}`, {
    method: "PATCH",
    body: JSON.stringify(input)
  });
}

export function probeManagedModelAPIConfig(id: string, tenantId: string) {
  return apiClient.request<{ result: ManagedAPIProbeResult; config: ManagedModelAPIConfig }>(`/api/v1/platform/model-api-configs/${encodeURIComponent(id)}/probe${tenantQuery(tenantId)}`, {
    method: "POST"
  });
}
