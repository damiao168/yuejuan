import { expect, test } from "@playwright/test";
import { captureFailedRequests, installApiMocks } from "../fixtures/apiMocks";

test("学校管理员登录后从可信工作台进入正式答题卡导入", async ({ page }, testInfo) => {
  const failedRequests = captureFailedRequests(page);
  await installApiMocks(page);

  await page.goto("/");
  await page.getByLabel("学校代码").fill("demo-school");
  await page.getByLabel("账号").fill("school_admin");
  await page.getByRole("textbox", { name: /密码/ }).fill("password");
  await page.getByRole("button", { name: "登录" }).click();

  await expect(page.getByRole("heading", { name: "考试工作台" })).toBeVisible({ timeout: 30_000 });
  await expect(page.getByRole("heading", { name: "考试管理" })).toBeVisible();
  await expect(page.getByRole("heading", { name: "阅卷与成绩" })).toBeVisible();
  await expect(page.locator(".grading-metric-strip").getByText("107", { exact: true })).toBeVisible();
  await expect(page.getByText("主观题等待确认")).toBeVisible();

  await page.getByRole("button", { name: "处理答卷" }).click();
  await expect(page).toHaveURL(/#\/admin\/exams\/exam-1\/capture/);
  await expect(page.getByRole("heading", { name: "答卷导入" })).toBeVisible({ timeout: 30_000 });
  await expect(page.getByRole("button", { name: "新建批次" })).toBeVisible();
  await expect(page.getByText("当前页面是兼容模式")).toHaveCount(0);

  await testInfo.attach("request-failures", {
    body: failedRequests.length ? failedRequests.join("\n") : "none",
    contentType: "text/plain"
  });
});
