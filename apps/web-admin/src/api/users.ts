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

export async function listManagedUsers() {
  return apiClient.request<{ users: ManagedUser[] }>("/api/v1/users");
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
