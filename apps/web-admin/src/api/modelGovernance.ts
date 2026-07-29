import { apiClient } from "./client";

export type ProviderKind = "local" | "external";
export type ProviderStatus = "unverified" | "active" | "degraded" | "rate_limited" | "disabled";
export type DeploymentStatus = "unverified" | "shadow_only" | "disabled";
export type HealthState = "unverified" | "available" | "degraded" | "rate_limited" | "unavailable" | "disabled";
export type PolicyMode = "local_only" | "shadow_compare" | "cloud_suggestion" | "hybrid_escalation" | "dual_provider_review";

export interface DataPolicy {
  training_allowed: boolean;
  retention_mode: "no_store" | "contractual";
}

export interface ModelProvider {
  id: string;
  tenant_id: string;
  provider_key: string;
  display_name: string;
  provider_kind: ProviderKind;
  adapter_type: string;
  credential_reference_set: boolean;
  credential_scheme?: string;
  region: string;
  data_policy: DataPolicy;
  status: ProviderStatus;
  created_at: string;
  updated_at: string;
}

export interface ModelDeployment {
  id: string;
  tenant_id: string;
  provider_id: string;
  provider_key: string;
  deployment_key: string;
  model_name: string;
  model_version: string;
  region: string;
  capability_profile: string;
  modalities: Array<"text" | "image">;
  capability_policy?: Record<string, unknown>;
  pricing_policy?: Record<string, unknown>;
  status: DeploymentStatus;
  health_state: HealthState;
  created_at: string;
  updated_at: string;
}

export interface TenantModelPolicy {
  id: string;
  tenant_id: string;
  policy_key: string;
  display_name: string;
  mode: PolicyMode;
  external_enabled: boolean;
  text_export_enabled: boolean;
  image_export_enabled: boolean;
  allowed_deployments: string[];
  max_cost_micros_per_question: number;
  max_cost_micros_per_exam: number;
  fallback_mode: "manual_only" | "approved_deployment_only";
  status: string;
  version: number;
  updated_at: string;
}

export interface CreateProviderInput {
  provider_key: string;
  display_name: string;
  provider_kind: ProviderKind;
  adapter_type: string;
  credential_ref?: string;
  region: string;
  data_policy: DataPolicy;
  status?: ProviderStatus;
}

export interface CreateDeploymentInput {
  provider_id: string;
  deployment_key: string;
  model_name: string;
  model_version: string;
  region: string;
  capability_profile: string;
  modalities: Array<"text" | "image">;
  capability_policy: Record<string, unknown>;
  pricing_policy: Record<string, unknown>;
  status?: DeploymentStatus;
  health_state?: HealthState;
}

export interface UpdatePolicyInput {
  display_name: string;
  mode: PolicyMode;
  external_enabled: boolean;
  text_export_enabled: boolean;
  image_export_enabled: boolean;
  allowed_deployments: string[];
  max_cost_micros_per_question: number;
  max_cost_micros_per_exam: number;
  fallback_mode: "manual_only" | "approved_deployment_only";
  expected_version: number;
  reason: string;
}

export interface SecretProbe {
  scheme: string;
  resolver_supported: boolean;
  configured: boolean;
  meets_minimum_strength: boolean;
}

export function listModelProviders() {
  return apiClient.request<{ providers: ModelProvider[] }>("/api/v1/model-providers");
}

export function createModelProvider(input: CreateProviderInput) {
  return apiClient.request<{ provider: ModelProvider }>("/api/v1/model-providers", {
    method: "POST",
    body: JSON.stringify(input)
  });
}

export function listModelDeployments() {
  return apiClient.request<{ deployments: ModelDeployment[] }>("/api/v1/model-deployments");
}

export function createModelDeployment(input: CreateDeploymentInput) {
  return apiClient.request<{ deployment: ModelDeployment }>("/api/v1/model-deployments", {
    method: "POST",
    body: JSON.stringify(input)
  });
}

export function getModelPolicy() {
  return apiClient.request<{ policy: TenantModelPolicy }>("/api/v1/model-policy");
}

export function updateModelPolicy(input: UpdatePolicyInput) {
  return apiClient.request<{ policy: TenantModelPolicy }>("/api/v1/model-policy", {
    method: "PUT",
    body: JSON.stringify(input)
  });
}

export function probeModelSecret(credentialRef: string) {
  return apiClient.request<{ probe: SecretProbe }>("/api/v1/model-secrets/probe", {
    method: "POST",
    body: JSON.stringify({ credential_ref: credentialRef })
  });
}
