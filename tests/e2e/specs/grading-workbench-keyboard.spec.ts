import { expect, test } from "@playwright/test";
import {
  createGradingWorkbenchMockState,
  installGradingWorkbenchMocks,
  subjectFixtures
} from "../fixtures/gradingWorkbenchMocks";

test("阅卷教师可用键盘连续完成 20 份跨学科夹具，并在刷新后恢复服务端草稿", async ({ page }) => {
  test.setTimeout(60_000);
  const state = createGradingWorkbenchMockState(20);
  await installGradingWorkbenchMocks(page, state);
  await page.goto("/#/teacher/grading", { waitUntil: "domcontentloaded" });

  const score = page.getByRole("spinbutton", { name: "最终得分" });
  const submit = page.getByRole("button", { name: "提交并下一份" });
  await expect(score).toBeVisible();
  await expect(page.locator(".answer-panel")).toContainText("fixture-01");

  await page.getByRole("button", { name: "领取任务" }).click();
  await expect.poll(() => state.claims).toEqual(["review-task-01"]);

  // A focused input must retain its own keyboard semantics: neither a digit
  // nor Enter may trigger the workbench's global score/submit shortcuts.
  await score.focus();
  await page.keyboard.press("1");
  await page.keyboard.press("Enter");
  await expect.poll(() => state.submissions).toHaveLength(0);
  await score.evaluate((element) => (element as HTMLInputElement).blur());
  await expect(page.locator(".draft-save-status")).toHaveText("草稿已保存", { timeout: 8_000 });
  await expect.poll(() => state.draftWrites).toEqual(["review-task-01"]);

  await page.getByRole("button", { name: "刷新" }).click();
  await expect(score).toHaveValue("1.0");

  for (let index = 0; index < state.tasks.length; index += 1) {
    const task = state.tasks[index];
    const expectedTool = subjectFixtures[index % subjectFixtures.length].tool;
    await expect(page.locator(".answer-panel")).toContainText(task.anonymous_code);
    await expect(page.locator(".subject-tool-panel h2")).toHaveText(expectedTool);

    // Refresh leaves focus outside form inputs. The score and submit are then
    // exclusively driven by the documented keyboard shortcuts.
    await page.keyboard.press(String((index % 8) + 1));
    await expect(submit).toBeEnabled();
    await page.keyboard.press("Enter");
    await expect.poll(() => state.submissions).toHaveLength(index + 1);

    if (index + 1 < state.tasks.length) {
      await expect(page.locator(".answer-panel")).toContainText(state.tasks[index + 1].anonymous_code);
    }
  }

  expect(state.contextsRead).toEqual(expect.arrayContaining(state.tasks.map((task) => task.id)));
  expect(new Set(state.tasks.map((task) => task.subject.code))).toEqual(new Set(["chinese", "mathematics", "physics", "history"]));
});

test("两个浏览器窗口保存同一草稿时，后写窗口收到 revision conflict", async ({ browser }) => {
  test.setTimeout(30_000);
  const state = createGradingWorkbenchMockState(1);
  const firstContext = await browser.newContext();
  const secondContext = await browser.newContext();
  const first = await firstContext.newPage();
  const second = await secondContext.newPage();
  await installGradingWorkbenchMocks(first, state);
  await installGradingWorkbenchMocks(second, state);

  try {
    await Promise.all([
      first.goto("/#/teacher/grading", { waitUntil: "domcontentloaded" }),
      second.goto("/#/teacher/grading", { waitUntil: "domcontentloaded" })
    ]);
    const firstScore = first.getByRole("spinbutton", { name: "最终得分" });
    const secondScore = second.getByRole("spinbutton", { name: "最终得分" });
    await Promise.all([expect(firstScore).toBeVisible(), expect(secondScore).toBeVisible()]);

    await firstScore.fill("5");
    await expect(first.locator(".draft-save-status")).toHaveText("草稿已保存", { timeout: 8_000 });
    await expect.poll(() => state.drafts.get("review-task-01")?.revision).toBe(1);

    await secondScore.fill("6");
    await expect(second.locator(".draft-save-status")).toHaveText("草稿冲突", { timeout: 8_000 });
    await expect(second.getByText("草稿已被其他会话更新", { exact: true })).toBeVisible();
    expect(state.drafts.get("review-task-01")?.score).toBe(5);
  } finally {
    await Promise.all([firstContext.close(), secondContext.close()]);
  }
});
