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
}

export interface LoginResponse {
  token_type: "Bearer";
  access_token: string;
  expires_at: string;
  user: AuthUser;
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
