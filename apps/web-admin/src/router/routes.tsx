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
  mock: boolean;
  productionReady: boolean;
  replacementPath?: string;
}

export const routes: AppRoute[] = [
  { key: "dashboard", path: "/dashboard", title: "首页", group: "运营", icon: <Home size={18} />, permissions: [], mock: false, productionReady: true },
  { key: "exams", path: "/exams", title: "考试管理", group: "教务", icon: <ClipboardCheck size={18} />, permissions: ["exam:manage"], mock: false, productionReady: true },
  { key: "papers", path: "/papers", title: "试卷管理", group: "教务", icon: <FileText size={18} />, permissions: ["exam:manage", "file:manage"], mock: false, productionReady: true },
  {
    key: "capture",
    path: "/capture",
    title: "答卷采集",
    group: "采集",
    icon: <ScanLine size={18} />,
    permissions: ["submission:manage", "file:manage", "ocr:manage", "segment:manage"],
    mock: false,
    productionReady: true
  },
  {
    key: "grading",
    path: "/grading",
    title: "智能阅卷",
    group: "阅卷",
    icon: <Sparkles size={18} />,
    permissions: ["review:manage", "grading:manage", "segment:manage", "ocr:manage", "file:manage", "exam:manage", "submission:manage", "evidence:manage"],
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
  { key: "arbitration", path: "/arbitration", title: "双评仲裁", group: "阅卷", icon: <Gavel size={18} />, permissions: [], anyPermissions: ["arbitration:manage", "arbitration:work"], mock: false, productionReady: true },
  { key: "scores", path: "/scores", title: "成绩管理", group: "结果", icon: <Gauge size={18} />, permissions: ["score:manage", "exam:manage", "submission:manage"], mock: false, productionReady: true },
  { key: "reports", path: "/reports", title: "学情报告", group: "结果", icon: <BarChart3 size={18} />, permissions: ["report:read"], mock: false, productionReady: true },
  { key: "quality", path: "/quality", title: "质量控制", group: "结果", icon: <Activity size={18} />, permissions: ["quality:read"], mock: true, productionReady: false },
  { key: "appeals", path: "/appeals", title: "申诉中心", group: "治理", icon: <Inbox size={18} />, permissions: ["appeal:read"], mock: false, productionReady: true },
  { key: "permissions", path: "/permissions", title: "用户权限", group: "治理", icon: <Users size={18} />, permissions: ["org:manage"], mock: true, productionReady: false },
  { key: "settings", path: "/settings", title: "系统设置", group: "治理", icon: <Settings size={18} />, permissions: ["system:manage"], mock: true, productionReady: false },
  { key: "systemStatus", path: "/system/status", title: "系统状态", group: "治理", icon: <ServerCog size={18} />, permissions: ["system:read"], mock: false, productionReady: true },
  { key: "audit", path: "/audit", title: "审计日志", group: "治理", icon: <ScrollText size={18} />, permissions: ["audit:read"], mock: false, productionReady: true }
];

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
  return routes.filter(isRouteVisible);
}

export function hasRouteAccess(user: SessionUser | null, route: AppRoute): boolean {
  return hasEveryPermission(user, route.permissions) && (!route.anyPermissions || hasAnyPermission(user, route.anyPermissions));
}

export function routeFromPath(pathname: string): AppRoute {
  return visibleRoutes().find((route) => route.path === pathname) ?? notFoundRoute;
}

export function routeGroups() {
  return Array.from(new Set(visibleRoutes().map((route) => route.group)));
}

export function pathFromHash(): string {
  const hash = window.location.hash.replace(/^#/, "");
  return hash || "/dashboard";
}
