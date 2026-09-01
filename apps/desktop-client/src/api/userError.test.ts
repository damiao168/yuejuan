import { describe, expect, it, vi } from "vitest";
import zhCN from "antd/locale/zh_CN";
import { DesktopApiClient } from "./client";
import { getUserErrorMessage } from "./userError";

describe("desktop user-facing errors", () => {
  it("does not expose backend messages or status text", async () => {
    vi.stubGlobal("fetch", vi.fn<typeof fetch>().mockResolvedValue(new Response(
      JSON.stringify({ error: { code: "unknown_failure", message: "Backend exploded" } }),
      { status: 500, statusText: "Internal Server Error", headers: { "Content-Type": "application/json" } }
    )));
    const client = new DesktopApiClient({ baseUrl: "https://grading.example.edu" });
    await expect(client.request("/api/v1/test")).rejects.toMatchObject({
      message: "系统暂时无法完成操作，请稍后重试。"
    });
  });

  it("uses a Chinese fallback for non-JSON errors", async () => {
    vi.stubGlobal("fetch", vi.fn<typeof fetch>().mockResolvedValue(new Response("Bad Gateway", {
      status: 502,
      statusText: "Bad Gateway"
    })));
    const client = new DesktopApiClient({ baseUrl: "https://grading.example.edu" });
    await expect(client.request("/api/v1/test")).rejects.toMatchObject({
      message: "系统暂时无法完成操作，请稍后重试。"
    });
  });

  it("maps network errors and provides Chinese modal defaults", () => {
    expect(getUserErrorMessage(new TypeError("Failed to fetch"))).toBe("网络连接异常，请检查网络后重试。");
    expect(zhCN.Modal?.cancelText).toBe("取消");
  });
});
