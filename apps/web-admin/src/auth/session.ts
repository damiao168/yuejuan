import type { AuthUser } from "../api/auth";

export interface SessionUser {
  id: string;
  name: string;
  role: string;
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
  return {
    id: user.id,
    name: user.display_name || user.username,
    role,
    tenant: user.tenant_code,
    school: schoolLabelFromScope(user.data_scope),
    currentExam: "未选择考试",
    permissions: user.permissions
  };
}

function schoolLabelFromScope(scope: Record<string, unknown>): string {
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
  return "当前租户";
}
