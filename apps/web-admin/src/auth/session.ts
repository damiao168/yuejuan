import type { AuthUser } from "../api/auth";

export interface SessionUser {
  id: string;
  username: string;
  displayName: string;
  name: string;
  role: string;
  roles: string[];
  tenant: string;
  school: string;
  currentExam: string;
  permissions: string[];
}

export function hasEveryPermission(user: SessionUser | null, permissions: string[]): boolean {
  if (!user) {
    return false;
  }
  return permissions.every((permission) => user.permissions.includes(permission));
}

export function hasAnyPermission(user: SessionUser | null, permissions: string[]): boolean {
  return Boolean(user && permissions.some((permission) => user.permissions.includes(permission)));
}

export function sessionFromAuthUser(user: AuthUser): SessionUser {
  const role = user.roles[0] ?? "user";
  const displayName = normalizeDisplayName(user.display_name);
  return {
    id: user.id,
    username: user.username,
    displayName,
    name: displayNameOrUsername(user.display_name, user.username),
    role,
    roles: user.roles,
    tenant: user.tenant_code,
    school: schoolLabelFromScope(user.data_scope, user.tenant_code),
    currentExam: "未选择考试",
    permissions: user.permissions
  };
}

export function displayNameOrUsername(displayName: unknown, username: string): string {
  return normalizeDisplayName(displayName) || username.trim() || "未命名用户";
}

function normalizeDisplayName(value: unknown): string {
  if (typeof value !== "string") {
    return "";
  }
  const trimmed = value.trim();
  return trimmed && !/^[?？]+$/.test(trimmed) && !trimmed.includes("\uFFFD") ? trimmed : "";
}

function schoolLabelFromScope(scope: Record<string, unknown>, tenantCode: string): string {
  const direct = scope["school_name"];
  if (typeof direct === "string" && direct.trim()) {
    return direct;
  }
  for (const value of Object.values(scope)) {
    if (!value || typeof value !== "object" || Array.isArray(value)) {
      continue;
    }
    const nested = value as Record<string, unknown>;
    const nestedSchool = nested["school_name"];
    if (typeof nestedSchool === "string" && nestedSchool.trim()) {
      return nestedSchool;
    }
  }
  return tenantCode || "当前机构";
}
