import { apiClient } from "./client";

export interface AuthUser {
  id: string;
  tenant_id: string;
  tenant_code: string;
  username: string;
  display_name: string;
  status: string;
  roles: string[];
  permissions: string[];
  data_scope: Record<string, unknown>;
}

export interface LoginRequest {
  tenant_code: string;
  username: string;
  password: string;
  remember_device?: boolean;
  device_name?: string;
}

export interface LoginResponse {
  expires_at: string;
  user: AuthUser;
}

export interface DeviceSession {
  id: string;
  session_type: "standard" | "remembered_device" | "desktop_device" | "service";
  device_name: string;
  created_at: string;
  last_seen_at: string;
  expires_at: string;
  current: boolean;
}

export async function login(input: LoginRequest) {
  return apiClient.request<LoginResponse>("/api/v1/auth/login", {
    method: "POST",
    body: JSON.stringify(input)
  });
}

export async function getCurrentUser() {
  return apiClient.request<{ user: AuthUser }>("/api/v1/auth/me");
}

export async function logout() {
  return apiClient.request<{ status: string }>("/api/v1/auth/logout", {
    method: "POST"
  });
}

export function listSessions() {
  return apiClient.request<{ sessions: DeviceSession[] }>("/api/v1/auth/sessions");
}

export function revokeSession(id: string) {
  return apiClient.request<{ status: string }>(`/api/v1/auth/sessions/${encodeURIComponent(id)}`, {
    method: "DELETE"
  });
}

export function logoutAll() {
  return apiClient.request<{ status: string; revoked_count: number }>("/api/v1/auth/logout-all", {
    method: "POST"
  });
}

export function changePassword(input: { current_password: string; new_password: string }) {
  return apiClient.request<{ status: string; revoked_count: number }>("/api/v1/auth/password", {
    method: "POST",
    body: JSON.stringify(input)
  });
}
