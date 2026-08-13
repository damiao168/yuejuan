import type { SessionUser } from "../auth/session";
import {
  canonicalPathFromPath,
  defaultExperience,
  experienceFromPath,
  hasExperienceAccess,
  pathForExperience
} from "./experience";
import {
  examWorkspaceFromPath,
  hasExamWorkspaceSectionAccess,
  hasRouteAccess,
  notFoundRoute,
  routeFromPath
} from "./routes";

export function normalizeEntryPath(path: string): string {
  return path === "/" || path === "" ? "/dashboard" : path;
}

export function legacyRedirectForPath(path: string, user: SessionUser): string | null {
  const normalized = normalizeEntryPath(path);
  if (experienceFromPath(normalized)) {
    return null;
  }
  return pathForExperience(normalized, defaultExperience(user));
}

export function hasPathAccess(user: SessionUser, path: string): boolean {
  const normalized = normalizeEntryPath(path);
  const experience = experienceFromPath(normalized) ?? defaultExperience(user);
  const canonicalPath = canonicalPathFromPath(normalized);
  const route = routeFromPath(canonicalPath);
  const workspace = examWorkspaceFromPath(canonicalPath);

  return route !== notFoundRoute
    && hasExperienceAccess(user, experience)
    && hasRouteAccess(user, route, experience)
    && (!workspace || hasExamWorkspaceSectionAccess(experience, workspace.section));
}
