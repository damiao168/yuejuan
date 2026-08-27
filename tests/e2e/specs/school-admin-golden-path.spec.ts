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
  await expect(page.getByRole("heading", { name: "需要你处理" })).toBeVisible();
  await expect(page.getByText("进行中的考试", { exact: true }).first()).toBeVisible();
  await expect(page.getByText("107 题待阅")).toBeVisible();
  await expect(page.getByText("主观题等待确认")).toBeVisible();

  await page.getByText("答题卡导入", { exact: true }).click();
  await expect(page).toHaveURL(/#\/admin\/capture/);
  await expect(page.getByRole("heading", { name: "答题卡导入" })).toBeVisible();
  await expect(page.getByRole("button", { name: "上传答题卡" })).toBeVisible();
  await expect(page.getByText("当前页面是兼容模式")).toHaveCount(0);

  await testInfo.attach("request-failures", {
    body: failedRequests.length ? failedRequests.join("\n") : "none",
    contentType: "text/plain"
  });
});
