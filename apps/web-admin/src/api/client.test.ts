import { describe, expect, it, vi } from "vitest";
import { ApiClient, ApiClientError } from "./client";

describe("web API diagnostics", () => {
  it("preserves request, trace and field context from the shared error envelope", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(JSON.stringify({
      error: { code: "invalid_exam", message: "internal detail" },
      request_id: "req-web-1",
      trace_id: "trace-web-1",
      field_errors: { name: ["required"] }
    }), { status: 400, headers: { "Content-Type": "application/json" } })));
    const client = new ApiClient({ baseUrl: "https://grading.example.edu" });

    const error = await client.request("/api/v1/exams", { method: "POST", body: "{}" }).catch((value) => value);

    expect(error).toBeInstanceOf(ApiClientError);
    expect(error).toMatchObject({
      status: 400,
      code: "invalid_exam",
      requestId: "req-web-1",
      traceId: "trace-web-1",
      fieldErrors: { name: ["required"] }
    });
  });
});
