export interface RememberedLogin {
  tenant_code: string;
  username: string;
  password: string;
}

interface EncryptedRememberedLogin {
  version: 1;
  iv: string;
  payload: string;
}

const STORAGE_KEY = "edugrade.remembered-login.v1";
const DATABASE_NAME = "edugrade-local-auth";
const DATABASE_VERSION = 1;
const KEY_STORE = "encryption-keys";
const KEY_ID = "remembered-login";

export const LOGIN_FIELD_LIMITS = {
  tenant_code: 128,
  username: 256,
  password: 72
} as const;

export function isWithinUtf8ByteLimit(value: string, limit: number): boolean {
  return new TextEncoder().encode(value).byteLength <= limit;
}

export async function loadRememberedLogin(): Promise<RememberedLogin | null> {
  try {
    const stored = window.localStorage.getItem(STORAGE_KEY);
    if (!stored) {
      return null;
    }
    if (stored.length > 16 * 1024) {
      throw new Error("Remembered login payload is too large.");
    }
    const encrypted = JSON.parse(stored) as EncryptedRememberedLogin;
    if (encrypted.version !== 1 || !encrypted.iv || !encrypted.payload) {
      throw new Error("Invalid remembered login payload.");
    }
    const key = await getEncryptionKey(false);
    if (!key) {
      throw new Error("Remembered login key is unavailable.");
    }
    const decrypted = await window.crypto.subtle.decrypt(
      { name: "AES-GCM", iv: fromBase64(encrypted.iv) },
      key,
      fromBase64(encrypted.payload)
    );
    const value = JSON.parse(new TextDecoder().decode(decrypted)) as Partial<RememberedLogin>;
    if (
      typeof value.tenant_code !== "string"
      || typeof value.username !== "string"
      || typeof value.password !== "string"
      || !value.tenant_code
      || !value.username
      || !value.password
      || !rememberedLoginWithinLimits(value as RememberedLogin)
    ) {
      throw new Error("Remembered login fields are incomplete.");
    }
    return {
      tenant_code: value.tenant_code,
      username: value.username,
      password: value.password
    };
  } catch {
    clearRememberedLogin();
    return null;
  }
}

export async function saveRememberedLogin(value: RememberedLogin): Promise<void> {
  if (
    !value.tenant_code
    || !value.username
    || !value.password
    || !rememberedLoginWithinLimits(value)
  ) {
    throw new Error("Login credentials exceed supported size limits.");
  }
  const key = await getEncryptionKey(true);
  if (!key) {
    throw new Error("This browser cannot securely remember login credentials.");
  }
  const iv = window.crypto.getRandomValues(new Uint8Array(12));
  const encrypted = await window.crypto.subtle.encrypt(
    { name: "AES-GCM", iv },
    key,
    new TextEncoder().encode(JSON.stringify(value))
  );
  const stored: EncryptedRememberedLogin = {
    version: 1,
    iv: toBase64(iv),
    payload: toBase64(new Uint8Array(encrypted))
  };
  window.localStorage.setItem(STORAGE_KEY, JSON.stringify(stored));
}

export function clearRememberedLogin(): void {
  try {
    window.localStorage.removeItem(STORAGE_KEY);
  } catch {
    // Storage can be disabled by browser policy. There is nothing else to clear.
  }
}

function getEncryptionKey(create: boolean): Promise<CryptoKey | null> {
  if (!window.isSecureContext || !window.crypto?.subtle || !window.indexedDB) {
    return Promise.resolve(null);
  }
  return openDatabase().then(async (database) => {
    try {
      const existing = await readKey(database);
      if (existing || !create) {
        return existing;
      }
      const key = await window.crypto.subtle.generateKey(
        { name: "AES-GCM", length: 256 },
        false,
        ["encrypt", "decrypt"]
      );
      await writeKey(database, key);
      return key;
    } finally {
      database.close();
    }
  });
}

function openDatabase(): Promise<IDBDatabase> {
  return new Promise((resolve, reject) => {
    const request = window.indexedDB.open(DATABASE_NAME, DATABASE_VERSION);
    request.onupgradeneeded = () => {
      if (!request.result.objectStoreNames.contains(KEY_STORE)) {
        request.result.createObjectStore(KEY_STORE);
      }
    };
    request.onsuccess = () => resolve(request.result);
    request.onerror = () => reject(request.error ?? new Error("Unable to open credential storage."));
  });
}

function readKey(database: IDBDatabase): Promise<CryptoKey | null> {
  return new Promise((resolve, reject) => {
    const request = database.transaction(KEY_STORE, "readonly").objectStore(KEY_STORE).get(KEY_ID);
    request.onsuccess = () => {
      const result = request.result as Partial<CryptoKey> | undefined;
      resolve(result && typeof result.type === "string" && Array.isArray(result.usages) ? result as CryptoKey : null);
    };
    request.onerror = () => reject(request.error ?? new Error("Unable to read credential key."));
  });
}

function writeKey(database: IDBDatabase, key: CryptoKey): Promise<void> {
  return new Promise((resolve, reject) => {
    const request = database.transaction(KEY_STORE, "readwrite").objectStore(KEY_STORE).put(key, KEY_ID);
    request.onsuccess = () => resolve();
    request.onerror = () => reject(request.error ?? new Error("Unable to save credential key."));
  });
}

function toBase64(value: Uint8Array): string {
  return window.btoa(String.fromCharCode(...value));
}

function fromBase64(value: string): ArrayBuffer {
  const bytes = Uint8Array.from(window.atob(value), (character) => character.charCodeAt(0));
  return bytes.buffer.slice(bytes.byteOffset, bytes.byteOffset + bytes.byteLength) as ArrayBuffer;
}

function rememberedLoginWithinLimits(value: RememberedLogin): boolean {
  return isWithinUtf8ByteLimit(value.tenant_code, LOGIN_FIELD_LIMITS.tenant_code)
    && isWithinUtf8ByteLimit(value.username, LOGIN_FIELD_LIMITS.username)
    && isWithinUtf8ByteLimit(value.password, LOGIN_FIELD_LIMITS.password);
}
