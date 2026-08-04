const LEGACY_STORAGE_KEY = "edugrade.remembered-login.v1";
const LEGACY_DATABASE_NAME = "edugrade-local-auth";

export const LOGIN_FIELD_LIMITS = {
  tenant_code: 128,
  username: 256,
  password: 72
} as const;

export function isWithinUtf8ByteLimit(value: string, limit: number): boolean {
  return new TextEncoder().encode(value).byteLength <= limit;
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
