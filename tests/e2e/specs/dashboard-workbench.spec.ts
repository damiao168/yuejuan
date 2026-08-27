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

test("首页主动作进入新建考试、阅卷和考试当前阶段", async ({ page }) => {
  await installApiMocks(page);
  await login(page);

  await page.getByRole("button", { name: "新建考试" }).click();
  await expect(page).toHaveURL(/#\/admin\/exams\?create=1/);
  await expect(page.getByRole("dialog", { name: "新建考试" })).toBeVisible();

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
