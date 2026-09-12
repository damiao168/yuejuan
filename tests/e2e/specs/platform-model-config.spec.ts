import { expect, test } from "@playwright/test";
import { installApiMocks } from "../fixtures/apiMocks";

test("切换学校后忽略过期配置响应，并保留新学校的配置", async ({ page }) => {
  await installApiMocks(page, { role: "platform_admin", initiallyAuthenticated: true });
  await page.route("**/api/v1/tenants?*", route => route.fulfill({ json: {
    tenants: [{ id: "school-a", code: "A", name: "甲学校", status: "active" },
      { id: "school-b", code: "B", name: "乙学校", status: "active" }], has_more: false
  } }));
  let releaseA!: () => void;
  const pendingA = new Promise<void>(resolve => { releaseA = resolve; });
  let startedA = false;
  let finishedA = false;
  await page.route("**/api/v1/platform/model-api-configs?*", async route => {
    const tenant = new URL(route.request().url()).searchParams.get("tenant_id");
    if (tenant === "school-a") {
      startedA = true;
      await pendingA;
    }
    await route.fulfill({ json: { configs: [{ id: tenant, tenant_id: tenant,
      provider_key: "custom", display_name: tenant === "school-a" ? "甲校模型" : "乙校模型",
      model_name: "test-model", model_version: "v1", base_url: "https://example.test/v1",
      adapter_type: "openai_compatible", credential_configured: true, status: "active", is_default: true,
      last_test_status: "untested", region: "global" }] } });
    if (tenant === "school-a") finishedA = true;
  });
  await page.goto("/#/platform/model-config");
  await expect.poll(() => startedA).toBe(true);
  await page.getByText("甲学校 · A", { exact: true }).click();
  await page.getByText("乙学校 · B", { exact: true }).click();
  await expect(page.getByText("乙校模型", { exact: true })).toBeVisible();
  releaseA();
  await expect.poll(() => finishedA).toBe(true);
  await expect(page.getByText("甲校模型", { exact: true })).toHaveCount(0);
  await expect(page.getByText("乙校模型", { exact: true })).toBeVisible();
  await page.getByRole("button", { name: "添加模型" }).click();
  await expect(page.getByRole("combobox", { name: "配置学校" })).toBeDisabled();
  await page.getByRole("button", { name: "检测连接并保存", exact: true }).click();
  await expect(page.getByText("请选择供应商", { exact: true })).toBeVisible();
  await expect(page.getByText("请输入 API Key", { exact: true })).toBeVisible();
  await expect(page.getByRole("dialog")).toBeVisible();
});

test("只填供应商和 API Key 即可零 Token 获取模型并保存", async ({ page }) => {
  await installApiMocks(page, { role: "platform_admin", initiallyAuthenticated: true });
  await page.route("**/api/v1/tenants?*", route => route.fulfill({ json: {
    tenants: [{ id: "school-a", code: "A", name: "甲学校", status: "active" }], has_more: false
  } }));
  await page.route(/\/api\/v1\/platform\/model-api-configs(?:\?.*)?$/, route => route.fulfill({ json: { configs: [] } }));
  await page.route("**/api/v1/platform/model-api-configs/models", async route => {
    expect(route.request().postDataJSON()).toEqual({
      tenant_id: "school-a",
      api_key: "deepseek-secret-at-least-16",
      provider: "deepseek"
    });
    await route.fulfill({ json: {
      provider: { key: "deepseek", display_name: "DeepSeek" },
      models: ["deepseek-v4-flash", "deepseek-v4-pro"],
      latency_ms: 12
    } });
  });
  await page.route("**/api/v1/platform/model-api-configs/auto", async route => {
    const body = route.request().postDataJSON();
    expect(body).toEqual({
      tenant_id: "school-a",
      api_key: "deepseek-secret-at-least-16",
      model_name: "deepseek-v4-pro",
      provider: "deepseek"
    });
    await route.fulfill({ status: 201, json: {
      config: {
        id: "model-1", tenant_id: "school-a", provider_key: "deepseek", display_name: "DeepSeek",
        adapter_type: "openai_compatible", base_url: "https://api.deepseek.com", model_name: "deepseek-v4-pro",
        model_version: "deepseek-v4-pro", region: "global", credential_configured: true,
        credential_hint: "•••• st-16", status: "active", is_default: false, last_test_status: "success",
        last_probe_mode: "quick", last_capability_status: "untested",
        last_test_message: "连接检查成功，本次未发送模型生成请求", last_test_latency_ms: 18, last_tested_at: "2026-09-11T09:00:00Z",
        config_source: "auto", provider_registry_version: "2026-09-11",
        created_at: "2026-09-11T09:00:00Z", updated_at: "2026-09-11T09:00:00Z"
      },
      validation: {
        ok: true, probe_mode: "quick", generated_request: false, provider: "deepseek", model: "deepseek-v4-pro", status_code: 200, latency_ms: 18,
        message: "连接检查成功，本次未发送模型生成请求", credential_check: { ok: true }, model_check: { ok: true }, capability_check: { code: "not_run" },
        usage: { input_tokens: 0, cached_input_tokens: 0, output_tokens: 0, reasoning_tokens: 0, total_tokens: 0 }, diagnostic: {}
      }
    } });
  });

  await page.goto("/#/platform/model-config");
  await page.getByRole("button", { name: "添加模型" }).click();
  await page.getByLabel("供应商").click();
  await page.locator(".ant-select-dropdown:visible .ant-select-item-option-content", { hasText: "DeepSeek" }).click();
  await page.getByLabel("API Key").fill("deepseek-secret-at-least-16");
  await page.getByRole("button", { name: "获取可用模型" }).click();
  const modelDropdown = page.locator(".ant-select-dropdown:visible");
  await expect(modelDropdown.locator(".ant-select-item-option-content", { hasText: "deepseek-v4-flash" })).toBeVisible();
  await expect(page.getByRole("button", { name: "已找到 2 个，点击选择" })).toBeVisible();
  await modelDropdown.locator(".ant-select-item-option-content", { hasText: "deepseek-v4-pro" }).click();
  await page.getByRole("button", { name: "检测连接并保存", exact: true }).click();

  await expect(page.getByRole("dialog")).toHaveCount(0);
  await expect(page.getByText("DeepSeek", { exact: true })).toBeVisible();
  await expect(page.getByText("备用", { exact: true })).toBeVisible();
  await expect(page.getByText(/API Key 有效 · 模型可用 · 0 生成 Token · 18ms/)).toBeVisible();
});

test("普通连接检测使用零生成 Token 快速模式", async ({ page }) => {
  await installApiMocks(page, { role: "platform_admin", initiallyAuthenticated: true });
  await page.route("**/api/v1/tenants?*", route => route.fulfill({ json: {
    tenants: [{ id: "school-a", code: "A", name: "甲学校", status: "active" }], has_more: false
  } }));
  const config = {
    id: "model-1", tenant_id: "school-a", provider_key: "deepseek", display_name: "DeepSeek",
    adapter_type: "openai_compatible", base_url: "https://api.deepseek.com", model_name: "deepseek-v4-pro",
    model_version: "deepseek-v4-pro", region: "global", credential_configured: true,
    credential_hint: "•••• st-16", status: "active", is_default: true, last_test_status: "success",
    last_probe_mode: "capability", last_test_message: "配置验证成功", last_test_latency_ms: 18,
    last_tested_at: "2026-09-11T09:00:00Z", last_capability_status: "success",
    last_capability_message: "结构化输出正常", last_capability_tested_at: "2026-09-11T09:00:00Z",
    last_capability_probe_version: "structured-json-v3",
    last_capability_usage: { input_tokens: 9, cached_input_tokens: 0, output_tokens: 5, reasoning_tokens: 0, total_tokens: 14 },
    config_source: "auto", provider_registry_version: "2026-09-11",
    created_at: "2026-09-11T09:00:00Z", updated_at: "2026-09-11T09:00:00Z"
  };
  await page.route(/\/api\/v1\/platform\/model-api-configs(?:\?.*)?$/, route => route.fulfill({ json: { configs: [config] } }));
  let quickRequestSeen = false;
  await page.route("**/api/v1/platform/model-api-configs/model-1/probe?*", route => {
    const url = new URL(route.request().url());
    expect(url.searchParams.get("mode")).toBe("quick");
    expect(url.searchParams.has("force")).toBe(false);
    quickRequestSeen = true;
    return route.fulfill({ json: {
      result: {
        ok: true, probe_mode: "quick", generated_request: false, provider: "deepseek", model: "deepseek-v4-pro",
        latency_ms: 11, message: "连接检查成功，本次未发送模型生成请求",
        credential_check: { ok: true }, model_check: { ok: true }, capability_check: { ok: true, code: "reused" },
        usage: { input_tokens: 0, cached_input_tokens: 0, output_tokens: 0, reasoning_tokens: 0, total_tokens: 0 }
      },
      config: { ...config, last_probe_mode: "quick", last_test_latency_ms: 11 }
    } });
  });

  await page.goto("/#/platform/model-config");
  await page.getByRole("button", { name: "检测连接" }).click();
  await expect.poll(() => quickRequestSeen).toBe(true);
  await expect(page.getByText("连接正常", { exact: true })).toBeVisible();
  await expect(page.getByText(/结构化能力：已验证/)).toBeVisible();
});

test("完整能力检测失败时保留配置并展示具体原因", async ({ page }) => {
  await installApiMocks(page, { role: "platform_admin", initiallyAuthenticated: true });
  await page.route("**/api/v1/tenants?*", route => route.fulfill({ json: {
    tenants: [{ id: "school-a", code: "A", name: "甲学校", status: "active" }], has_more: false
  } }));
  const config = {
    id: "model-1", tenant_id: "school-a", provider_key: "deepseek", display_name: "DeepSeek",
    adapter_type: "openai_compatible", base_url: "https://api.deepseek.com", model_name: "deepseek-flash",
    model_version: "deepseek-flash", region: "global", credential_configured: true,
    credential_hint: "•••• st-16", status: "active", is_default: false, last_test_status: "success",
    last_probe_mode: "quick", last_test_message: "连接正常", last_test_latency_ms: 12,
    last_tested_at: "2026-09-11T09:00:00Z", last_capability_status: "untested",
    config_source: "auto", provider_registry_version: "2026-09-11",
    created_at: "2026-09-11T09:00:00Z", updated_at: "2026-09-11T09:00:00Z"
  };
  await page.route(/\/api\/v1\/platform\/model-api-configs(?:\?.*)?$/, route => route.fulfill({ json: { configs: [config] } }));
  await page.route("**/api/v1/platform/model-api-configs/model-1/probe?*", route => route.fulfill({ json: {
    result: {
      ok: false, probe_mode: "capability", generated_request: true, provider: "deepseek", model: "deepseek-flash",
      status_code: 200, latency_ms: 420, message: "模型返回内容不是合法 JSON", error_code: "invalid_json",
      credential_check: { ok: true }, model_check: { ok: true },
      capability_check: { ok: false, code: "invalid_json", message: "模型返回内容不是合法 JSON" },
      usage: { input_tokens: 28, cached_input_tokens: 0, output_tokens: 3, reasoning_tokens: 0, total_tokens: 31 },
      diagnostic: { finish_reason: "stop", response_format: "json_object", content_length: 8, content_preview: "not json" }
    },
    config: { ...config, last_capability_status: "failed", last_capability_message: "模型返回内容不是合法 JSON",
      last_capability_probe_version: "structured-json-v3", last_capability_diagnostic: {
        finish_reason: "stop", response_format: "json_object", content_length: 8
      } }
  } }));

  await page.goto("/#/platform/model-config");
  await page.getByRole("button", { name: "DeepSeek更多操作" }).click();
  await page.getByText("完整能力检测", { exact: true }).click();
  const diagnosticDialog = page.getByRole("dialog");
  await expect(diagnosticDialog.locator(".ant-modal-confirm-title")).toHaveText("DeepSeek：结构化输出检测未通过");
  await expect(diagnosticDialog.getByText("not json", { exact: true })).toBeVisible();
  await expect(page.getByText("验证失败", { exact: false })).toBeVisible();
  await expect(page.getByText("DeepSeek", { exact: true })).toBeVisible();
});

test("选择自定义供应商后才显示 API 地址", async ({ page }) => {
  await installApiMocks(page, { role: "platform_admin", initiallyAuthenticated: true });
  await page.route("**/api/v1/tenants?*", route => route.fulfill({ json: {
    tenants: [{ id: "school-a", code: "A", name: "甲学校", status: "active" }], has_more: false
  } }));
  await page.route(/\/api\/v1\/platform\/model-api-configs(?:\?.*)?$/, route => route.fulfill({ json: { configs: [] } }));
  await page.goto("/#/platform/model-config");
  await page.getByRole("button", { name: "添加模型" }).click();
  await expect(page.getByLabel("API 地址")).toHaveCount(0);
  await page.getByLabel("供应商").click();
  await page.locator(".ant-select-dropdown:visible .ant-select-item-option-content", { hasText: "其他兼容接口" }).click();
  await expect(page.getByLabel("API 地址")).toBeVisible();
  await expect(page.getByRole("dialog")).toBeVisible();
});
