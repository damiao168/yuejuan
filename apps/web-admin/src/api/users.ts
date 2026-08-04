import { apiClient } from "./client";

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
  const params = new URLSearchParams();
  if (filter.q) params.set("q", filter.q);
  if (filter.role) params.set("role", filter.role);
  if (filter.limit) params.set("limit", String(filter.limit));
  if (filter.cursor) params.set("cursor", filter.cursor);
  const query = params.toString();
  return apiClient.request<{ users: ManagedUser[]; next_cursor: string; has_more: boolean }>(`/api/v1/users${query ? `?${query}` : ""}`);
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
