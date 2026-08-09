import { describe, expect, it } from "vitest";
import { redactLocalLogEntry, redactSensitiveText } from "./localRuntime";

describe("desktop local log redaction", () => {
  it("redacts bearer tokens and named credential fields", () => {
    const redacted = redactSensitiveText(
      'Authorization: Bearer abc.def password=cleartext {"access_token":"token-value"}'
    );

    expect(redacted).not.toContain("abc.def");
    expect(redacted).not.toContain("cleartext");
    expect(redacted).not.toContain("token-value");
    expect(redacted.match(/\[REDACTED\]/g)?.length).toBeGreaterThanOrEqual(3);
  });

  it("redacts both message and context without mutating unrelated metadata", () => {
    const redacted = redactLocalLogEntry({
      id: "log-1",
      at: "2026-08-09T00:00:00Z",
      level: "error",
      message: "credential=machine-secret",
      context: "safe operation"
    });

    expect(redacted.id).toBe("log-1");
    expect(redacted.message).toBe("credential=[REDACTED]");
    expect(redacted.context).toBe("safe operation");
  });
});
