import type { AuthUser, LoginResult } from "../types";
import type { DesktopApiClient } from "./client";

export interface LoginPayload {
  tenant_code: string;
  username: string;
  password: string;
}

export async function login(client: DesktopApiClient, payload: LoginPayload) {
  return client.request<LoginResult>("/api/v1/auth/login", {
    method: "POST",
    body: JSON.stringify(payload)
  });
}

export async function getCurrentUser(client: DesktopApiClient) {
  return client.request<{ user: AuthUser }>("/api/v1/auth/me");
}
