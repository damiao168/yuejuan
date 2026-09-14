import { afterEach, describe, expect, it, vi } from "vitest";
import { mfaAPI } from "./mfa";

afterEach(() => { vi.unstubAllGlobals(); vi.useRealTimers(); });

describe("MFA requests are single-use, non-cached auth requests", () => {
  it("uses fixed routes, CSRF headers and cookies without generic idempotency keys or automatic retry", async () => {
    const fetch = vi.fn().mockImplementation(() => Promise.resolve(new Response("{}", { headers: { "Content-Type": "application/json" } })));
    vi.stubGlobal("fetch", fetch);
    const abort = new AbortController();
    await mfaAPI.status(abort.signal);
    await mfaAPI.enroll("synthetic-password", abort.signal);
    await mfaAPI.confirm("synthetic-enrollment", "012345", abort.signal);
    await mfaAPI.start("mfa.disable", "synthetic-password", abort.signal);
    await mfaAPI.verify("synthetic-challenge", "recovery_code", "synthetic-code", abort.signal);
    await mfaAPI.disable("synthetic-challenge", abort.signal);
    await mfaAPI.rotate("synthetic-challenge", abort.signal);
    expect(fetch).toHaveBeenCalledTimes(7);
    expect(fetch.mock.calls.map((call) => call[0])).toEqual([
      "/api/v1/auth/mfa", "/api/v1/auth/mfa/totp/enroll", "/api/v1/auth/mfa/totp/confirm", "/api/v1/auth/step-up/start", "/api/v1/auth/step-up/verify", "/api/v1/auth/mfa/totp/disable", "/api/v1/auth/mfa/recovery-codes/rotate"
    ]);
    for (const [, options] of fetch.mock.calls) {
      expect(options).toMatchObject({ cache: "no-store", credentials: "include" });
      expect(options.signal).toBeInstanceOf(AbortSignal);
      expect(options.headers.has("Idempotency-Key")).toBe(false);
      if (options.method === "POST") expect(options.headers.get("X-EduGrade-CSRF")).toBe("1");
    }
    expect(JSON.parse(fetch.mock.calls[2]![1].body)).toEqual({ enrollment_id: "synthetic-enrollment", code: "012345" });
    expect(JSON.parse(fetch.mock.calls[4]![1].body)).toEqual({ challenge_id: "synthetic-challenge", method: "recovery_code", code: "synthetic-code" });
  });
  it("does not retry an ambiguous mutation", async () => {
    const fetch = vi.fn().mockRejectedValue(new TypeError("network dropped")); vi.stubGlobal("fetch", fetch);
    await expect(mfaAPI.rotate("synthetic-challenge")).rejects.toThrow("network dropped");
    expect(fetch).toHaveBeenCalledTimes(1);
  });
  it("bounds a hung request to thirty seconds and does not retry it", async () => {
    vi.useFakeTimers();
    const fetch = vi.fn().mockImplementation((_path, options) => new Promise((_resolve, reject) => {
      options.signal.addEventListener("abort", () => reject(new DOMException("aborted", "AbortError")), { once: true });
    }));
    vi.stubGlobal("fetch", fetch);
    const pending = expect(mfaAPI.rotate("synthetic-challenge")).rejects.toMatchObject({ name: "AbortError" });
    await vi.advanceTimersByTimeAsync(30_000); await pending;
    expect(fetch).toHaveBeenCalledTimes(1); expect(vi.getTimerCount()).toBe(0);
  });
  it("forwards caller cancellation, including already aborted signals, and cleans timers", async () => {
    vi.useFakeTimers();
    const fetch = vi.fn().mockImplementation((_path, options) => new Promise((_resolve, reject) => {
      const abort = () => reject(new DOMException("aborted", "AbortError"));
      if (options.signal.aborted) abort(); else options.signal.addEventListener("abort", abort, { once: true });
    }));
    vi.stubGlobal("fetch", fetch);
    const controller = new AbortController();
    const pending = expect(mfaAPI.enroll("synthetic-password", controller.signal)).rejects.toMatchObject({ name: "AbortError" });
    controller.abort(); await pending;
    expect(fetch.mock.calls[0]![1].signal.aborted).toBe(true);
    await expect(mfaAPI.status(controller.signal)).rejects.toMatchObject({ name: "AbortError" });
    expect(vi.getTimerCount()).toBe(0);
  });
});
