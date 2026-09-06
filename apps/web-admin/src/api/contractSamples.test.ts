import { readFileSync } from "node:fs";
import type { ErrorResponse, ProcessingSummaryResponse } from "@edugrade/sdk";
import { describe, expect, it, vi } from "vitest";
import { ApiClient, ApiClientError } from "./client";

function sample<T>(name: string): T {
  return JSON.parse(readFileSync(new URL(`../../../../contracts/samples/${name}`, import.meta.url), "utf8")) as T;
}

describe("shared API response samples", () => {
  it("matches the generated processing summary type used by the web consumer", () => {
    const response = sample<ProcessingSummaryResponse>("processing-summary.response.json");
    expect(response.summary.total_pages).toBe(500);
    expect(response.summary.by_stage[0]?.stage).toBe("READY");
  });

  it("preserves the Go error envelope diagnostics in the web transport", async () => {
    const response = sample<ErrorResponse>("error.response.json");
    const client = new ApiClient({ baseUrl: "https://grading.example.edu" });
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(JSON.stringify(response), {
      status: 409,
      headers: { "Content-Type": "application/json" }
    })));
    const error = await client.request("/api/v1/exams/1", { method: "PUT" }).catch((value) => value);
    expect(error).toBeInstanceOf(ApiClientError);
    expect(error).toMatchObject({ requestId: "req-contract-001", traceId: "trace-contract-001" });
  });
});
