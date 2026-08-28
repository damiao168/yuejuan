import { expect, test } from "@playwright/test";
import { installApiMocks } from "../fixtures/apiMocks";

test("考试工作区按四阶段导航，并让阻断项跳到可处理环节", async ({ page }) => {
  await installApiMocks(page, { role: "school_admin", initiallyAuthenticated: true });
  await page.goto("/#/admin/exams/exam-1/overview", { waitUntil: "domcontentloaded" });

  await expect(page.getByRole("heading", { name: "2026 春季数学期中考试" })).toBeVisible({ timeout: 30_000 });
  const stageRail = page.getByRole("navigation", { name: "考试流程" });
  await expect(stageRail.getByRole("button")).toHaveCount(4);
  await expect(stageRail).toContainText("考试准备");
  await expect(stageRail).toContainText("答卷导入");
  await expect(stageRail).toContainText("阅卷");
  await expect(stageRail).toContainText("成绩");

  await page.getByRole("button", { name: "查看并重试", exact: true }).click();
  await expect(page).toHaveURL(/#\/admin\/exams\/exam-1\/capture/);

  await page.getByRole("button", { name: /阅卷/ }).click();
  await expect(page).toHaveURL(/#\/admin\/exams\/exam-1\/grading/);
  await page.getByRole("button", { name: /成绩/ }).click();
  await expect(page).toHaveURL(/#\/admin\/exams\/exam-1\/scores/);
});

test("阅卷员无法通过考试工作区深链访问学校管理员环节", async ({ page }) => {
  await installApiMocks(page, { role: "grader", initiallyAuthenticated: true });
  await page.goto("/#/admin/exams/exam-1/overview", { waitUntil: "domcontentloaded" });

  await expect(page.getByText("无权限", { exact: true })).toBeVisible();
  await expect(page.getByRole("navigation", { name: "考试流程" })).toHaveCount(0);
});
