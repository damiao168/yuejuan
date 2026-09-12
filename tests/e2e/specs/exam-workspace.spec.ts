import { expect, test } from "@playwright/test";
import { captureFailedRequests, installApiMocks } from "../fixtures/apiMocks";

test("mock API 支持动态考试 ID，并在测试层拦截未配置接口", async ({ page }) => {
  await installApiMocks(page, { role: "school_admin", initiallyAuthenticated: true });
  const failedRequests = captureFailedRequests(page);
  await page.goto("/#/admin/dashboard", { waitUntil: "domcontentloaded" });

  const result = await page.evaluate(async () => {
    const workspaceResponse = await fetch("/api/v1/exams/exam-created-math/workspace");
    const workspace = await workspaceResponse.json();
    const missingResponse = await fetch("/api/v1/e2e-unconfigured");
    const missing = await missingResponse.json();
    return {
      workspaceStatus: workspaceResponse.status,
      workspaceExamID: workspace.workspace.exam_id,
      missingStatus: missingResponse.status,
      missingCode: missing.error.code
    };
  });

  expect(result).toEqual({
    workspaceStatus: 200,
    workspaceExamID: "exam-created-math",
    missingStatus: 404,
    missingCode: "e2e_mock_missing"
  });
  expect(failedRequests).toEqual([]);
});

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
