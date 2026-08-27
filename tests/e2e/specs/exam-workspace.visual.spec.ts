import { expect, test } from "@playwright/test";
import { installApiMocks } from "../fixtures/apiMocks";

const desktopViewports = [
  { name: "1366x768", width: 1366, height: 768 },
  { name: "1440x900", width: 1440, height: 900 },
  { name: "1920x1080", width: 1920, height: 1080 }
] as const;

test.use({
  colorScheme: "light",
  locale: "zh-CN",
  timezoneId: "Asia/Shanghai"
});

for (const viewport of desktopViewports) {
  test(`exam workspace showcase is stable at ${viewport.name}`, async ({ page }, testInfo) => {
    await page.setViewportSize(viewport);
    await installApiMocks(page, { role: "school_admin", initiallyAuthenticated: true });
    await page.goto("/#/admin/exams/exam-1/overview", { waitUntil: "networkidle" });

    const workspace = page.locator(".eg-workspace-layout");
    await expect(workspace).toBeVisible({ timeout: 30_000 });
    await expect(workspace.locator('[aria-label="考试流程"]')).toBeVisible();
    const stages = workspace.locator('[aria-label="考试流程"] .eg-workspace-stage');
    await expect(stages).toHaveCount(4);
    await expect(stages).toHaveText([/考试准备/, /答卷导入/, /阅卷/, /成绩/]);
    await page.evaluate(async () => document.fonts.ready);

    const dimensions = await page.evaluate(() => ({
      viewportWidth: window.innerWidth,
      documentWidth: document.documentElement.scrollWidth
    }));
    expect(dimensions.viewportWidth).toBe(viewport.width);
    expect(dimensions.documentWidth).toBeLessThanOrEqual(viewport.width);

    const image = await page.screenshot({ fullPage: false, animations: "disabled", scale: "css" });
    await testInfo.attach(`exam-workspace-${viewport.name}`, {
      body: image,
      contentType: "image/png"
    });
    // Pixel rendering differs between the checked-in Windows showcase host
    // and Linux CI (fonts and rasterization). CI still exercises every layout
    // assertion above and publishes the actual screenshot attachment.
    if (process.platform === "win32") {
      await expect(page).toHaveScreenshot(`exam-workspace-${viewport.name}.png`, {
        animations: "disabled",
        caret: "hide",
        scale: "css"
      });
    }
  });
}
