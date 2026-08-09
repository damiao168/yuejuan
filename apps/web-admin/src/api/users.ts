import { apiClient } from "./client";
import { buildQueryString } from "./query";

export interface ManagedUser {
  id: string;
  username: string;
  display_name: string;
  status: string;
  roles: string[];
  created_at?: string;
}

export interface AssignableRole {
  code: string;
  name: string;
  scope_type: string;
  description?: string;
}

export interface CreateManagedUserPayload {
  username: string;
  display_name: string;
  password: string;
  role_code: string;
}

export async function listManagedUsers(filter: { q?: string; role?: string; limit?: number; cursor?: string } = {}) {
  return apiClient.request<{ users: ManagedUser[]; next_cursor: string; has_more: boolean }>(`/api/v1/users${buildQueryString(filter)}`);
}

export async function listAssignableRoles() {
  return apiClient.request<{ roles: AssignableRole[] }>("/api/v1/roles");
}

export async function createManagedUser(payload: CreateManagedUserPayload) {
  return apiClient.request<{ user: ManagedUser }>("/api/v1/users", {
    method: "POST",
    body: JSON.stringify(payload)
  });
}
