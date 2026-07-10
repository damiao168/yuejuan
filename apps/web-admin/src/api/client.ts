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

export interface ApiClientOptions {
  baseUrl?: string;
  getToken?: () => string | null;
}

export class ApiClient {
  private readonly baseUrl: string;
  private readonly getToken?: () => string | null;

  constructor(options: ApiClientOptions = {}) {
    this.baseUrl = options.baseUrl ?? import.meta.env.VITE_API_BASE_URL ?? "http://127.0.0.1:8080";
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
      const message = payload.error?.message ?? payload.message ?? response.statusText;
      return new ApiClientError(response.status, code, message);
    } catch {
      return new ApiClientError(response.status, "request_failed", response.statusText);
    }
  }
}

export const apiClient = new ApiClient();

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
