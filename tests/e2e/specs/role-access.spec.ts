import { expect, test } from "@playwright/test";
import { installApiMocks } from "../fixtures/apiMocks";

test("平台管理员不能从直达网址进入学校考试管理", async ({ page }) => {
  await installApiMocks(page, { role: "platform_admin", initiallyAuthenticated: true });
  await page.goto("/#/admin/exams");
  await expect(page.getByText("无权限", { exact: true })).toBeVisible();
  await expect(page.locator(".side-menu").getByText("考试列表", { exact: true })).toHaveCount(0);
});

test("阅卷老师只能进入自己的阅卷工作区", async ({ page }) => {
  await installApiMocks(page, { role: "grader", initiallyAuthenticated: true });
  await page.goto("/#/admin/exams");
  await expect(page.getByText("无权限", { exact: true })).toBeVisible();
  await expect(page.locator(".side-menu").getByText("我的阅卷", { exact: true })).toBeVisible();
});
