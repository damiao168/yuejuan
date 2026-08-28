import { expect, test } from "@playwright/test";

test("学校管理员通过真实网关登录并查看已持久化考试", async ({ page }, testInfo) => {
  const serverErrors: string[] = [];
  page.on("response", (response) => {
    if (response.url().includes("/api/") && response.status() >= 500) {
      serverErrors.push(`${response.status()} ${response.request().method()} ${response.url()}`);
    }
  });

  await page.goto("/");
  await page.getByLabel("学校代码").fill("platform");
  await page.getByLabel("账号").fill("story060_school_admin");
  await page.getByRole("textbox", { name: /密码/ }).fill(process.env.EDUGRADE_E2E_SCHOOL_ADMIN_PASSWORD ?? "");
  await page.getByRole("button", { name: "登录" }).click();

  await expect(page.getByRole("heading", { name: "考试工作台" })).toBeVisible();
  await page.getByText("考试列表", { exact: true }).click();
  await expect(page).toHaveURL(/#\/admin\/exams/);
  await expect(page.getByText("STORY-060 Synthetic Chinese Exam", { exact: true })).toBeVisible();
  expect(serverErrors).toEqual([]);

  await testInfo.attach("system-boundary", {
    body: "No Playwright API routes were registered; requests traversed browser-gateway and api-gateway.",
    contentType: "text/plain"
  });
});
