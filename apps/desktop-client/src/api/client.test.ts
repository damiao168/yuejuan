import { describe, expect, it } from "vitest";
import { DesktopApiClient, normalizeBaseUrl } from "./client";

describe("desktop API origin validation", () => {
  it("allows HTTPS servers and loopback HTTP development servers", () => {
    expect(normalizeBaseUrl("https://grading.example.edu/"))
      .toBe("https://grading.example.edu");
    expect(normalizeBaseUrl("http://127.0.0.1:8080/"))
      .toBe("http://127.0.0.1:8080");
    expect(normalizeBaseUrl("http://localhost:8080"))
      .toBe("http://localhost:8080");
  });

  it("rejects remote plaintext HTTP and embedded credentials", () => {
    expect(() => normalizeBaseUrl("http://grading.example.edu"))
      .toThrow("远程 API 必须使用 HTTPS");
    expect(() => normalizeBaseUrl("https://user:password@grading.example.edu"))
      .toThrow("不能包含用户名或密码");
  });

  it("rejects API paths that could replace the configured origin", () => {
    const client = new DesktopApiClient({ baseUrl: "https://grading.example.edu" });

    expect(() => client.url("//attacker.example/api"))
      .toThrow("same-server absolute path");
    expect(client.url("/api/v1/auth/me"))
      .toBe("https://grading.example.edu/api/v1/auth/me");
  });
});
