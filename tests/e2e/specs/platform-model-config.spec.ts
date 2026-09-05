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
  await page.getByRole("button", { name: "添加 API 配置" }).click();
  await expect(page.getByRole("combobox", { name: "配置学校" })).toBeDisabled();
  await page.getByRole("button", { name: "保存配置", exact: true }).click();
  await expect(page.getByText("请输入 API Key", { exact: true })).toBeVisible();
  await expect(page.getByRole("dialog")).toBeVisible();
});
