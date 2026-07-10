export interface ApiErrorPayload {
  error?: {
    code?: string;
    message?: string;
  };
  code?: string;
  message?: string;
}

export class ApiClientError extends Error {
  status: number;
  code: string;

  constructor(status: number, code: string, message: string) {
    super(message);
    this.name = "ApiClientError";
    this.status = status;
    this.code = code;
  }
}

export interface DesktopApiClientOptions {
  baseUrl: string;
  getToken?: () => string | null;
}

export interface SystemStatus {
  status: "healthy" | "degraded";
  service: string;
  environment: string;
  version: string;
  started_at: string;
  generated_at: string;
  uptime_sec: number;
  dependencies: {
    name: string;
    status: "ok" | "error" | "not_configured";
    error?: string;
    detail?: string;
    duration_ms: number;
    checked_at: string;
  }[];
  observability: {
    log_format: string;
    system_log_stream: string;
    audit_log_stream: string;
    request_id_header: string;
    trace_id_header: string;
    slow_request_threshold_ms: number;
    slow_query_log: string;
    sensitive_log_policy: string;
  };
}

export class DesktopApiClient {
  private readonly baseUrl: string;
  private readonly getToken?: () => string | null;

  constructor(options: DesktopApiClientOptions) {
    this.baseUrl = normalizeBaseUrl(options.baseUrl);
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
    const response = await fetch(this.url(path), { ...init, headers });
    if (!response.ok) {
      throw await toApiError(response);
    }
    if (response.status === 204) {
      return undefined as T;
    }
    return (await response.json()) as T;
  }

  async health() {
    return this.request<Record<string, unknown>>("/health");
  }

  async systemStatus() {
    return this.request<SystemStatus>("/api/v1/system/status");
  }

  async requestBlob(path: string, init: RequestInit = {}) {
    const headers = new Headers(init.headers);
    const authorization = this.authorizationHeader();
    if (authorization) {
      headers.set("Authorization", authorization);
    }
    const response = await fetch(this.url(path), { ...init, headers });
    if (!response.ok) {
      throw await toApiError(response);
    }
    return {
      blob: await response.blob(),
      contentType: response.headers.get("Content-Type") ?? "application/octet-stream",
      filename: filenameFromDisposition(response.headers.get("Content-Disposition"))
    };
  }
}

export function normalizeBaseUrl(value: string) {
  const trimmed = value.trim().replace(/\/+$/, "");
  return trimmed || "http://127.0.0.1:8080";
}

async function toApiError(response: Response): Promise<ApiClientError> {
  try {
    const payload = (await response.json()) as ApiErrorPayload;
    const code = payload.error?.code ?? payload.code ?? "request_failed";
    const message = payload.error?.message ?? payload.message ?? response.statusText;
    return new ApiClientError(response.status, code, message);
  } catch {
    return new ApiClientError(response.status, "request_failed", response.statusText);
  }
}

function filenameFromDisposition(disposition: string | null): string | undefined {
  if (!disposition) {
    return undefined;
  }
  const utf8Match = disposition.match(/filename\*=UTF-8''([^;]+)/i);
  if (utf8Match?.[1]) {
    return decodeURIComponent(utf8Match[1]);
  }
  const plainMatch = disposition.match(/filename="?([^";]+)"?/i);
  return plainMatch?.[1];
}
