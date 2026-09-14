const LEGACY_STORAGE_KEY = "edugrade.remembered-login.v1";
const LEGACY_DATABASE_NAME = "edugrade-local-auth";

export const LOGIN_FIELD_LIMITS = {
  tenant_code: 128,
  identifier: 256,
  password: 1024
} as const;

export const PUBLIC_COMPUTER_IDLE_LOCK_MS = 15 * 60 * 1000;

export function isPublicComputerIdle(lastActivityAt: number, now = Date.now(), timeout = PUBLIC_COMPUTER_IDLE_LOCK_MS): boolean {
  return now - lastActivityAt >= timeout;
}

export function isWithinUtf8ByteLimit(value: string, limit: number): boolean {
  return new TextEncoder().encode(value).byteLength <= limit;
}

// Match Go's rune count and UTF-8 byte ceiling rather than UTF-16 code units.
export function validateNewPassword(_: unknown, value?: string): Promise<void> {
  if (!value) return Promise.resolve();
  if (Array.from(value).length < 15) return Promise.reject(new Error("密码至少 15 个字符"));
  if (!isWithinUtf8ByteLimit(value, LOGIN_FIELD_LIMITS.password)) {
    return Promise.reject(new Error("密码过长，请缩短后重试"));
  }
  return Promise.resolve();
}

// One-way cleanup for versions that stored reversible login credentials.
// Failures are intentionally ignored so restrictive browser policies cannot
// prevent the application from reaching the login screen.
export function clearLegacyRememberedLogin(): void {
  try {
    window.localStorage.removeItem(LEGACY_STORAGE_KEY);
  } catch {
    // Storage may be disabled by policy.
  }
  try {
    window.indexedDB?.deleteDatabase(LEGACY_DATABASE_NAME);
  } catch {
    // IndexedDB may be unavailable or disabled by policy.
  }
}

export function clearPublicComputerData(tenantId: string, userId: string): void {
  if (typeof window === "undefined") return;
  const scope = `:${tenantId}:${userId}`;
  try {
    const keys: string[] = [];
    for (let index = 0; index < window.localStorage.length; index += 1) {
      const key = window.localStorage.key(index);
      if (key?.includes(scope)) keys.push(key);
    }
    for (const key of keys) window.localStorage.removeItem(key);
  } catch {
    // Browser policy can disable storage; in-memory cache is still cleared.
  }
}
