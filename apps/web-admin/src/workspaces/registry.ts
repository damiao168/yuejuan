import { hasAnyPermission, type SessionUser } from "../auth/session";
import type { ProductExperience } from "../router/experience";

export type Workspace =
  | "platform"
  | "school_admin"
  | "exam_owner"
  | "teacher"
  | "grader"
  | "arbitrator";

export interface WorkspaceDefinition {
  key: Workspace;
  label: string;
  purpose: string;
}

export const workspaceRegistry: Record<Workspace, WorkspaceDefinition> = {
  platform: { key: "platform", label: "平台控制台", purpose: "管理学校、平台服务、模型与审计" },
  school_admin: { key: "school_admin", label: "学校管理台", purpose: "组织学校考试并处理阻断事项" },
  exam_owner: { key: "exam_owner", label: "考试负责人", purpose: "配置和推进本人获授权的考试" },
  teacher: { key: "teacher", label: "教师工作台", purpose: "处理本人班级、考试和反馈任务" },
  grader: { key: "grader", label: "阅卷工作台", purpose: "处理分配给本人的匿名阅卷任务" },
  arbitrator: { key: "arbitrator", label: "仲裁工作台", purpose: "处理分配给本人的评分差异" }
};

export function workspacesForExperience(user: SessionUser, experience: ProductExperience): Workspace[] {
  if (experience === "admin") {
    if (user.roles.includes("platform_admin")) return ["platform"];
    if (user.roles.some((role) => ["tenant_admin", "school_admin"].includes(role))) return ["school_admin"];
    return ["exam_owner"];
  }
  const workspaces: Workspace[] = [];
  if (user.roles.includes("teacher")) workspaces.push("teacher");
  if (user.roles.includes("grader") || hasAnyPermission(user, ["review:work"])) workspaces.push("grader");
  if (user.roles.includes("arbitrator") || hasAnyPermission(user, ["arbitration:work"])) workspaces.push("arbitrator");
  return workspaces.length > 0 ? [...new Set(workspaces)] : ["teacher"];
}

export function workspaceForExperience(user: SessionUser, experience: ProductExperience): Workspace {
  return workspacesForExperience(user, experience)[0];
}

export function workspaceLabel(user: SessionUser, experience: ProductExperience) {
  return workspaceRegistry[workspaceForExperience(user, experience)].label;
}
