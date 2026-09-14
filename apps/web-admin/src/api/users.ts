import { apiClient } from "./client";
import { buildQueryString } from "./query";

export interface ManagedUser {
  id: string;
  username: string;
  display_name: string;
  phone_masked?: string;
  employee_no?: string;
  status: string;
  roles: string[];
  school_id?: string;
  created_at?: string;
  activated_at?: string;
  last_login_at?: string;
}

export interface AssignableRole {
  code: string;
  name: string;
  scope_type: string;
  description?: string;
}

export interface CreateManagedUserPayload {
  username?: string;
  phone?: string;
  employee_no?: string;
  display_name: string;
  password?: string;
  role_code: string;
  school_id?: string;
  class_ids?: string[];
}

export interface ManagedUserActivation {
  token: string;
  expires_at: string;
  path: string;
}

export interface ManagedUserRecovery extends ManagedUserActivation {
  display_name: string;
  phone_masked?: string;
}

export async function listManagedUsers(filter: { q?: string; role?: string; limit?: number; cursor?: string } = {}) {
  return apiClient.request<{ users: ManagedUser[]; next_cursor: string; has_more: boolean }>(`/api/v1/users${buildQueryString(filter)}`);
}

export async function listAssignableRoles() {
  return apiClient.request<{ roles: AssignableRole[] }>("/api/v1/roles");
}

export async function createManagedUser(payload: CreateManagedUserPayload) {
  return apiClient.request<{ user: ManagedUser; activation?: ManagedUserActivation }>("/api/v1/users", {
    method: "POST",
    body: JSON.stringify(payload)
  });
}

export async function updateManagedUserStatus(id: string, status: "active" | "disabled") {
  return apiClient.request<{ user: ManagedUser }>(`/api/v1/users/${encodeURIComponent(id)}/status`, {
    method: "PATCH",
    body: JSON.stringify({ status })
  });
}

export async function createCredentialRecovery(id: string) {
  return apiClient.request<{ recovery: ManagedUserRecovery }>(`/api/v1/users/${encodeURIComponent(id)}/credential-reset`, {
    method: "POST"
  });
}

export async function reissueManagedUserActivation(id: string) {
  return apiClient.request<{ activation: ManagedUserRecovery }>(`/api/v1/users/${encodeURIComponent(id)}/activation`, {
    method: "POST"
  });
}
