import type { ReactNode } from "react";
import {
  Activity,
  BarChart3,
  BookOpenCheck,
  ClipboardCheck,
  FileText,
  Gauge,
  Gavel,
  Home,
  Inbox,
  Layers3,
  LockKeyhole,
  ScanLine,
  ScrollText,
  ServerCog,
  Settings,
  Sparkles,
  Users
} from "lucide-react";
import type { ViewKey } from "../types";
import { hasAnyPermission, hasEveryPermission, type SessionUser } from "../auth/session";

export interface AppRoute {
  key: ViewKey | "permissions" | "settings";
  path: string;
  title: string;
  group: string;
  icon: ReactNode;
  permissions: string[];
  anyPermissions?: string[];
  allowedRoles?: string[];
  navigation?: boolean;
  mock: boolean;
  productionReady: boolean;
  replacementPath?: string;
}

export const routes: AppRoute[] = [
  { key: "dashboard", path: "/dashboard", title: "工作台", group: "工作", icon: <Home size={18} />, permissions: [], mock: false, productionReady: true },
  { key: "exams", path: "/exams", title: "考试", group: "工作", icon: <ClipboardCheck size={18} />, permissions: ["exam:manage"], mock: false, productionReady: true },
  { key: "papers", path: "/papers", title: "试卷管理", group: "考试配置", icon: <FileText size={18} />, permissions: ["exam:manage", "file:manage"], navigation: false, mock: false, productionReady: true },
  {
    key: "capture",
    path: "/capture",
    title: "答卷采集",
    group: "业务",
    icon: <ScanLine size={18} />,
    permissions: ["submission:manage", "file:manage", "ocr:manage", "segment:manage"],
    mock: false,
    productionReady: true
  },
  {
    key: "grading",
    path: "/grading",
    title: "阅卷",
    group: "业务",
    icon: <Sparkles size={18} />,
    permissions: [],
    anyPermissions: ["review:manage", "review:work"],
    mock: false,
    productionReady: true
  },
  {
    key: "review",
    path: "/review",
    title: "人工复核",
    group: "阅卷",
    icon: <BookOpenCheck size={18} />,
    permissions: ["review:manage"],
    mock: true,
    productionReady: false,
    replacementPath: "/grading"
  },
  { key: "arbitration", path: "/arbitration", title: "质量", group: "业务", icon: <Gavel size={18} />, permissions: [], anyPermissions: ["arbitration:manage", "arbitration:work"], mock: false, productionReady: true },
  { key: "scores", path: "/scores", title: "发布", group: "业务", icon: <Gauge size={18} />, permissions: ["score:manage", "exam:manage", "submission:manage"], mock: false, productionReady: true },
  { key: "reports", path: "/reports", title: "报告", group: "业务", icon: <BarChart3 size={18} />, permissions: ["report:read"], mock: false, productionReady: true },
  { key: "quality", path: "/quality", title: "质量控制", group: "结果", icon: <Activity size={18} />, permissions: ["quality:read"], mock: true, productionReady: false },
  { key: "appeals", path: "/appeals", title: "申诉", group: "业务", icon: <Inbox size={18} />, permissions: ["appeal:read"], mock: false, productionReady: true },
  { key: "organization", path: "/organization/setup", title: "组织与用户", group: "管理", icon: <Users size={18} />, permissions: ["org:manage"], mock: false, productionReady: true },
  { key: "permissions", path: "/permissions", title: "用户权限", group: "治理", icon: <Users size={18} />, permissions: ["org:manage"], mock: true, productionReady: false },
  { key: "settings", path: "/settings", title: "系统设置", group: "治理", icon: <Settings size={18} />, permissions: ["system:manage"], mock: true, productionReady: false },
  { key: "systemStatus", path: "/system/status", title: "系统运维", group: "管理", icon: <ServerCog size={18} />, permissions: ["system:read"], allowedRoles: ["platform_admin"], mock: false, productionReady: true },
  { key: "audit", path: "/audit", title: "审计日志", group: "管理", icon: <ScrollText size={18} />, permissions: ["audit:read"], mock: false, productionReady: true }
];

export const examWorkspaceRoute: AppRoute = {
  key: "examWorkspace",
  path: "/exams/:examId/overview",
  title: "考试工作区",
  group: "工作",
  icon: <ClipboardCheck size={18} />,
  permissions: ["exam:manage"],
  navigation: false,
  mock: false,
  productionReady: true
};

export const forbiddenRoute: AppRoute = {
  key: "system",
  path: "/forbidden",
  title: "无权限",
  group: "系统",
  icon: <LockKeyhole size={18} />,
  permissions: [],
  mock: false,
  productionReady: true
};

export const notFoundRoute: AppRoute = {
  key: "system",
  path: "/not-found",
  title: "未找到",
  group: "系统",
  icon: <Layers3 size={18} />,
  permissions: [],
  mock: false,
  productionReady: true
};

export function mockRoutesEnabled(): boolean {
  return import.meta.env.DEV || import.meta.env.MODE === "demo" || import.meta.env.VITE_ENABLE_MOCK_ROUTES === "true";
}

export function isRouteVisible(route: AppRoute): boolean {
  return route.productionReady || mockRoutesEnabled();
}

export function visibleRoutes(): AppRoute[] {
  return routes.filter((route) => isRouteVisible(route) && route.navigation !== false);
}

export function hasRouteAccess(user: SessionUser | null, route: AppRoute): boolean {
  return hasEveryPermission(user, route.permissions)
    && (!route.anyPermissions || hasAnyPermission(user, route.anyPermissions))
    && (!route.allowedRoles || Boolean(user && route.allowedRoles.some((role) => user.roles.includes(role))));
}

export function routeFromPath(pathname: string): AppRoute {
  if (examWorkspaceFromPath(pathname)) {
    return examWorkspaceRoute;
  }
  return routes.filter(isRouteVisible).find((route) => route.path === pathname) ?? notFoundRoute;
}

export function examWorkspaceFromPath(pathname: string): { examId: string; section: string } | null {
  const match = pathname.match(/^\/exams\/([^/]+)\/([^/?]+)$/);
  if (!match) {
    return null;
  }
  return { examId: decodeURIComponent(match[1]), section: match[2] };
}

export function routeGroups() {
  return Array.from(new Set(visibleRoutes().map((route) => route.group)));
}

export function pathFromHash(): string {
  const hash = window.location.hash.replace(/^#/, "");
  return hash || "/dashboard";
}
