const storagePrefix = "edugrade.review-draft.v1";
const maxDraftsPerUser = 12;
const maxDraftBytes = 96 * 1024;
const maxDraftAgeMs = 24 * 60 * 60 * 1000;

interface DraftIndexEntry {
  taskId: string;
  updatedAt: number;
}

export interface ReviewDraftFallback<T> {
  updatedAt: number;
  snapshot: T;
}

function storageAvailable() {
  return typeof window !== "undefined" && typeof window.localStorage !== "undefined";
}

function userScope(userId: string) {
  return encodeURIComponent(userId.trim());
}

function entryKey(userId: string, taskId: string) {
  return `${storagePrefix}:draft:${userScope(userId)}:${encodeURIComponent(taskId.trim())}`;
}

function indexKey(userId: string) {
  return `${storagePrefix}:index:${userScope(userId)}`;
}

function readIndex(userId: string): DraftIndexEntry[] {
  if (!storageAvailable() || !userId.trim()) return [];
  try {
    const parsed = JSON.parse(window.localStorage.getItem(indexKey(userId)) ?? "[]") as unknown;
    if (!Array.isArray(parsed)) return [];
    return parsed.flatMap((entry) => {
      if (!entry || typeof entry !== "object") return [];
      const typed = entry as { taskId?: unknown; updatedAt?: unknown };
      if (typeof typed.taskId !== "string" || typeof typed.updatedAt !== "number" || !Number.isFinite(typed.updatedAt)) return [];
      return [{ taskId: typed.taskId, updatedAt: typed.updatedAt }];
    });
  } catch {
    return [];
  }
}

function writeIndex(userId: string, entries: DraftIndexEntry[]) {
  if (!storageAvailable()) return;
  try {
    window.localStorage.setItem(indexKey(userId), JSON.stringify(entries));
  } catch {
    // Browser storage may be unavailable or quota-limited. The server draft remains authoritative.
  }
}

function prune(userId: string, now = Date.now()) {
  const entries = readIndex(userId);
  const expired = entries.filter((entry) => now-entry.updatedAt > maxDraftAgeMs);
  for (const entry of expired) {
    try {
      window.localStorage.removeItem(entryKey(userId, entry.taskId));
    } catch {
      // Expiry cleanup is best effort.
    }
  }
  const live = entries
    .filter((entry) => now-entry.updatedAt <= maxDraftAgeMs)
    .sort((left, right) => right.updatedAt-left.updatedAt);
  const retained = live.slice(0, maxDraftsPerUser);
  for (const entry of live.slice(maxDraftsPerUser)) {
    try {
      window.localStorage.removeItem(entryKey(userId, entry.taskId));
    } catch {
      // A failed cleanup must not stop the active draft from being saved.
    }
  }
  writeIndex(userId, retained);
  return retained;
}

export function saveReviewDraftFallback<T>(userId: string, taskId: string, snapshot: T): boolean {
  if (!storageAvailable() || !userId.trim() || !taskId.trim()) return false;
  const now = Date.now();
  const value: ReviewDraftFallback<T> = { updatedAt: now, snapshot };
  let raw: string;
  try {
    raw = JSON.stringify(value);
  } catch {
    return false;
  }
  if (raw.length > maxDraftBytes) return false;
  try {
    const retained = prune(userId, now).filter((entry) => entry.taskId !== taskId);
    const next = [{ taskId, updatedAt: now }, ...retained].slice(0, maxDraftsPerUser);
    for (const entry of retained.slice(Math.max(0, maxDraftsPerUser-1))) {
      if (!next.some((candidate) => candidate.taskId === entry.taskId)) {
        window.localStorage.removeItem(entryKey(userId, entry.taskId));
      }
    }
    window.localStorage.setItem(entryKey(userId, taskId), raw);
    writeIndex(userId, next);
    return true;
  } catch {
    return false;
  }
}

export function loadReviewDraftFallback<T>(userId: string, taskId: string): ReviewDraftFallback<T> | null {
  if (!storageAvailable() || !userId.trim() || !taskId.trim()) return null;
  prune(userId);
  try {
    const parsed = JSON.parse(window.localStorage.getItem(entryKey(userId, taskId)) ?? "null") as unknown;
    if (!parsed || typeof parsed !== "object") return null;
    const typed = parsed as { updatedAt?: unknown; snapshot?: unknown };
    if (typeof typed.updatedAt !== "number" || !Number.isFinite(typed.updatedAt) || Date.now()-typed.updatedAt > maxDraftAgeMs || !("snapshot" in typed)) {
      removeReviewDraftFallback(userId, taskId);
      return null;
    }
    return { updatedAt: typed.updatedAt, snapshot: typed.snapshot as T };
  } catch {
    removeReviewDraftFallback(userId, taskId);
    return null;
  }
}

export function removeReviewDraftFallback(userId: string, taskId: string) {
  if (!storageAvailable() || !userId.trim() || !taskId.trim()) return;
  try {
    window.localStorage.removeItem(entryKey(userId, taskId));
    writeIndex(userId, readIndex(userId).filter((entry) => entry.taskId !== taskId));
  } catch {
    // Cleanup is best effort; the expiry guard will retry on the next use.
  }
}

export function clearReviewDraftFallbacks(userId: string) {
  if (!storageAvailable() || !userId.trim()) return;
  for (const entry of readIndex(userId)) {
    try {
      window.localStorage.removeItem(entryKey(userId, entry.taskId));
    } catch {
      // Continue cleaning the remaining task-scoped drafts.
    }
  }
  try {
    window.localStorage.removeItem(indexKey(userId));
  } catch {
    // No further action is possible when browser storage is unavailable.
  }
}
