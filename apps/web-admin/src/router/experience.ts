import type { SessionUser } from "../auth/session";

export type ProductExperience = "admin" | "teacher";

export const ADMIN_ROLES = ["platform_admin", "tenant_admin", "school_admin"] as const;
export const TEACHER_ROLES = ["teacher", "grader", "arbitrator"] as const;

const experiencePrefixes: Record<ProductExperience, string> = {
  admin: "/admin",
  teacher: "/teacher"
};

function hasRole(user: SessionUser, roles: readonly string[]) {
  return roles.some((role) => user.roles.includes(role));
}

export function availableExperiences(user: SessionUser): ProductExperience[] {
  const experiences: ProductExperience[] = [];
  if (hasRole(user, ADMIN_ROLES)) experiences.push("admin");
  if (hasRole(user, TEACHER_ROLES)) experiences.push("teacher");
  return experiences;
}

export function defaultExperience(user: SessionUser): ProductExperience {
  const experiences = availableExperiences(user);
  return experiences.includes("admin") ? "admin" : "teacher";
}

export function hasExperienceAccess(user: SessionUser, experience: ProductExperience): boolean {
  return availableExperiences(user).includes(experience);
}

export function experienceFromPath(pathname: string): ProductExperience | null {
  const normalized = normalizePath(pathname);
  if (normalized === "/admin" || normalized.startsWith("/admin/")) return "admin";
  if (normalized === "/teacher" || normalized.startsWith("/teacher/")) return "teacher";
  return null;
}

export function canonicalPathFromPath(pathname: string): string {
  const normalized = normalizePath(pathname);
  const experience = experienceFromPath(normalized);
  if (!experience) return normalized;
  const canonical = normalized.slice(experiencePrefixes[experience].length);
  return normalizePath(canonical || "/dashboard");
}

export function pathForExperience(pathname: string, experience: ProductExperience): string {
  const canonical = canonicalPathFromPath(pathname);
  return `${experiencePrefixes[experience]}${canonical}`;
}

export function experienceLabel(experience: ProductExperience): string {
  return experience === "admin" ? "管理端" : "教师端";
}

function normalizePath(pathname: string): string {
  if (!pathname) return "/dashboard";
  const withoutQuery = pathname.split(/[?#]/, 1)[0];
  const normalized = withoutQuery.startsWith("/") ? withoutQuery : `/${withoutQuery}`;
  return normalized.length > 1 ? normalized.replace(/\/+$/, "") : normalized;
}
