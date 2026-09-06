import { getApiErrorMessage } from "./userError";
export { getPaperImportUserMessage, getSafeUserText, getUserErrorMessage } from "./userError";

export interface ApiErrorPayload {
  error?: {
    code?: string;
    message?: string;
  };
  code?: string;
  message?: string;
  request_id?: string;
  trace_id?: string;
  field_errors?: Record<string, string[]>;
  conflict_revision?: number;
}

export class ApiClientError extends Error {
  status: number;
  code: string;
  requestId?: string;
  traceId?: string;
  fieldErrors?: Record<string, string[]>;
  conflictRevision?: number;

  constructor(status: number, code: string, message: string, context: Pick<ApiErrorPayload, "request_id" | "trace_id" | "field_errors" | "conflict_revision"> = {}) {
    super(message);
    this.name = "ApiClientError";
    this.status = status;
    this.code = code;
    this.requestId = context.request_id;
    this.traceId = context.trace_id;
    this.fieldErrors = context.field_errors;
    this.conflictRevision = context.conflict_revision;
  }
}

export interface ApiClientOptions {
  baseUrl?: string;
  getToken?: () => string | null;
}

export class ApiClient {
  private readonly baseUrl: string;
  private readonly getToken?: () => string | null;

  constructor(options: ApiClientOptions = {}) {
    this.baseUrl = normalizeBaseUrl(options.baseUrl ?? import.meta.env.VITE_API_BASE_URL ?? "");
    this.getToken = options.getToken;
  }

  url(path: string): string {
    return `${this.baseUrl}${path}`;
  }

  authorizationHeader(): string | null {
    const token = this.getToken?.();
    return token ? `Bearer ${token}` : null;
  }

  async request<T>(path: string, init: RequestInit = {}): Promise<T> {
    const headers = new Headers(init.headers);
    headers.set("Accept", "application/json");
    const isFormData = typeof FormData !== "undefined" && init.body instanceof FormData;
    if (init.body && !headers.has("Content-Type") && !isFormData) {
      headers.set("Content-Type", "application/json");
    }
    const authorization = this.authorizationHeader();
    if (authorization) {
      headers.set("Authorization", authorization);
    }
    addBrowserCSRFHeader(init.method, headers);
    addIdempotencyHeader(path, init.method, headers);
    const response = await fetch(this.url(path), { ...init, headers, credentials: init.credentials ?? "include" });
    if (!response.ok) {
      throw await this.toError(response);
    }
    if (response.status === 204) {
      return undefined as T;
    }
    return (await response.json()) as T;
  }

  async requestBlob(path: string, init: RequestInit = {}) {
    const headers = new Headers(init.headers);
    const authorization = this.authorizationHeader();
    if (authorization) {
      headers.set("Authorization", authorization);
    }
    addBrowserCSRFHeader(init.method, headers);
    addIdempotencyHeader(path, init.method, headers);
    const response = await fetch(this.url(path), { ...init, headers, credentials: init.credentials ?? "include" });
    if (!response.ok) {
      throw await this.toError(response);
    }
    return {
      blob: await response.blob(),
      contentType: response.headers.get("Content-Type") ?? "application/octet-stream",
      filename: filenameFromDisposition(response.headers.get("Content-Disposition")),
      watermark: response.headers.get("X-EduGrade-Watermark") ?? undefined
    };
  }

  private async toError(response: Response): Promise<ApiClientError> {
    try {
      const payload = (await response.json()) as ApiErrorPayload;
      const code = payload.error?.code ?? payload.code ?? "request_failed";
    return new ApiClientError(response.status, code, getApiErrorMessage(code, response.status), payload);
    } catch {
      return new ApiClientError(response.status, "request_failed", getApiErrorMessage("request_failed", response.status));
    }
  }
}

function addIdempotencyHeader(path: string, method: string | undefined, headers: Headers) {
  const normalizedMethod = (method ?? "GET").toUpperCase();
  if (!["POST", "PUT", "PATCH", "DELETE"].includes(normalizedMethod) || path.startsWith("/api/v1/auth/")) {
    return;
  }
  if (!headers.has("Idempotency-Key")) {
    headers.set("Idempotency-Key", crypto.randomUUID());
  }
}

function addBrowserCSRFHeader(method: string | undefined, headers: Headers) {
  const normalizedMethod = (method ?? "GET").toUpperCase();
  if (["POST", "PUT", "PATCH", "DELETE"].includes(normalizedMethod) && !headers.has("X-EduGrade-CSRF")) {
    headers.set("X-EduGrade-CSRF", "1");
  }
}

export const apiClient = new ApiClient();

export function normalizeBaseUrl(value: string) {
  const trimmed = value.trim().replace(/\/+$/, "");
  return trimmed;
}

function filenameFromDisposition(disposition: string | null): string | undefined {
  if (!disposition) {
    return undefined;
  }
  const utf8Match = disposition.match(/filename\*=UTF-8''([^;]+)/i);
  if (utf8Match?.[1]) {
    try {
      return decodeURIComponent(utf8Match[1]);
    } catch {
      return utf8Match[1];
    }
  }
  const plainMatch = disposition.match(/filename="?([^";]+)"?/i);
  return plainMatch?.[1];
}
