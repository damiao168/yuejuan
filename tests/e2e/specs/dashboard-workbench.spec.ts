import { expect, test } from "@playwright/test";
import { installApiMocks } from "../fixtures/apiMocks";

async function login(page: import("@playwright/test").Page) {
  await page.goto("/");
  await page.getByLabel("学校代码").fill("demo-school");
  await page.getByLabel("账号").fill("school_admin");
  await page.getByRole("textbox", { name: /密码/ }).fill("password");
  await page.getByRole("button", { name: "登录" }).click();
  await expect(page.getByRole("heading", { name: "考试工作台" })).toBeVisible({ timeout: 30_000 });
}

test("学校工作台优先展示真实待办、四阶段考试和成员入口", async ({ page }) => {
  await installApiMocks(page);
  await login(page);

  await expect(page.getByRole("heading", { name: "需要你处理" })).toBeVisible();
  await expect(page.getByText("答卷处理失败")).toBeVisible();
  await expect(page.getByText("主观题等待确认")).toBeVisible();
  await expect(page.getByRole("heading", { name: "进行中的考试" })).toBeVisible();
  await expect(page.getByLabel("当前阶段：答卷导入")).toBeVisible();
  await expect(page.getByLabel("当前阶段：考试准备")).toBeVisible();
  await expect(page.getByLabel("当前阶段：阅卷")).toBeVisible();
  await expect(page.getByRole("heading", { name: "成员管理" })).toBeVisible();
  await expect(page.getByText("学生管理", { exact: true }).last()).toBeVisible();
});

test("桌面壳层取消空置顶栏，并固定品牌、菜单和账户区域", async ({ page }) => {
  await installApiMocks(page, { initiallyAuthenticated: true });
  await page.goto("/#/admin/dashboard", { waitUntil: "networkidle" });

  await expect(page.locator(".topbar")).toHaveCount(0);
  await expect(page.locator(".brand-copy")).toContainText("示范学校");
  await expect(page.getByRole("button", { name: "账户菜单：学校管理员" })).toBeVisible();

  const expandedWidth = await page.locator(".sidebar").evaluate((element) => element.getBoundingClientRect().width);
  expect(expandedWidth).toBeGreaterThanOrEqual(190);
  await page.getByRole("button", { name: "收起导航" }).click();
  await expect(page.getByRole("button", { name: "展开导航" })).toBeVisible();
  await expect.poll(() => page.locator(".sidebar").evaluate((element) => element.getBoundingClientRect().width)).toBeLessThanOrEqual(66);
  await expect(page.locator(".sidebar-account-copy")).toBeHidden();

  await page.getByRole("button", { name: "展开导航" }).click();
  await page.getByRole("button", { name: "账户菜单：学校管理员" }).click();
  await expect(page.getByText("账户安全", { exact: true })).toBeVisible();
  await expect(page.getByText("退出登录", { exact: true })).toBeVisible();
});

test("移动端只保留导航触发条，账户位于抽屉底部", async ({ page }) => {
  await page.setViewportSize({ width: 760, height: 800 });
  await installApiMocks(page, { initiallyAuthenticated: true });
  await page.goto("/#/admin/dashboard", { waitUntil: "networkidle" });

  await expect(page.locator(".topbar")).toHaveCount(0);
  const shellbar = page.locator(".mobile-shellbar");
  await expect(shellbar).toBeVisible();
  expect(await shellbar.evaluate((element) => element.getBoundingClientRect().height)).toBeLessThanOrEqual(48);
  await page.getByRole("button", { name: "打开主导航" }).click();
  await expect(page.locator(".mobile-navigation")).toBeVisible();
  await expect(page.getByRole("button", { name: "账户菜单：学校管理员" })).toBeVisible();
  await expect(page.locator(".mobile-navigation .sidebar-footer")).toBeVisible();
});

test("首页主动作完成分步骤表单新建考试并进入考试准备", async ({ page }) => {
  await installApiMocks(page);
  await login(page);

  await page.getByRole("button", { name: "新建考试" }).click();
  await expect(page).toHaveURL(/#\/admin\/exams\/new/);
  await expect(page.getByRole("heading", { name: "新建考试" })).toBeVisible();
  await expect(page.getByText("基本信息", { exact: true }).first()).toBeVisible();
  await page.getByRole("button", { name: /下一步/ }).click();
  await page.getByRole("button", { name: /高二（1）班/ }).click();
  await expect(page.getByText(/1 个班级.*2 名学生/)).toBeVisible();
  await page.getByRole("button", { name: /下一步/ }).click();
  await expect(page.getByText("阅卷完成并由管理员确认后发布")).toBeVisible();
  await page.getByRole("button", { name: /下一步/ }).click();
  await expect(page.getByRole("heading", { name: "确认创建" })).toBeVisible();
  await page.getByRole("button", { name: /创建考试/ }).click();
  await expect(page).toHaveURL(/#\/admin\/exams\/exam-created\/settings/);

  await page.goto("/#/admin/dashboard");
  await page.getByRole("button", { name: /主观题等待确认/ }).click();
  await expect(page).toHaveURL(/#\/admin\/grading\?status=pending/);

  await page.goto("/#/admin/dashboard");
  await page.getByRole("button", { name: /处理答卷/ }).click();
  await expect(page).toHaveURL(/#\/admin\/exams\/exam-1\/capture/);
});

test("没有待办时使用紧凑空状态", async ({ page }) => {
  await installApiMocks(page, { dashboardMode: "empty" });
  await login(page);

  await expect(page.getByText("当前没有需要你处理的事项")).toBeVisible();
  await expect(page.getByText("暂无进行中考试")).toBeVisible();
  const todoHeight = await page.locator(".todo-pane").evaluate((element) => element.getBoundingClientRect().height);
  expect(todoHeight).toBeLessThan(180);
});

test("成员管理入口进入正式班级和教师页面", async ({ page }) => {
  await installApiMocks(page);
  await login(page);

  await page.getByRole("button", { name: /年级与班级/ }).click();
  await expect(page).toHaveURL(/#\/admin\/members\/classes/);
  await expect(page.getByRole("heading", { name: "年级与班级" })).toBeVisible();
  await expect(page.getByText("高二（1）班")).toBeVisible();

  await page.goto("/#/admin/dashboard");
  await page.getByRole("button", { name: /教师与阅卷人员/ }).click();
  await expect(page).toHaveURL(/#\/admin\/members\/teachers/);
  await expect(page.getByRole("heading", { name: "教师与阅卷人员" })).toBeVisible();
  await expect(page.getByText("数学阅卷老师")).toBeVisible();
});
