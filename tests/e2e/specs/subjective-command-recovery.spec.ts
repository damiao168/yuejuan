import { expect, test } from "@playwright/test";
import { installApiMocks } from "../fixtures/apiMocks";

test("subjective batch preserves command and segments across double click and reload", async ({ page }) => {
  await installApiMocks(page, { initiallyAuthenticated: true });
  const requests: { key: string; body: string }[] = [];
  await page.route("**/api/v1/subjective-grading-batches", async route => {
    requests.push({ key: route.request().headers()["idempotency-key"], body: route.request().postData()! });
    await route.fulfill({ status: 503, json: { error: { code: "idempotency_persist_failed" } } });
  });
  await page.route("**/api/v1/subjective-grading-batch-commands/*", route => route.fulfill({ json: { command_id: route.request().url().split("/").at(-1), status: "not_accepted" } }));
  await page.goto("/#/grading/subjective-batches");
  await page.getByPlaceholder("输入答题片段 ID，使用换行或逗号分隔（最多 1000 个）").fill("segment-original");
  await page.getByRole("button", { name: "创建批次", exact: true }).evaluate(button => { (button as HTMLButtonElement).click(); (button as HTMLButtonElement).click(); });
  await expect.poll(() => requests.length).toBe(1);
  await page.reload();
  await page.getByPlaceholder("输入答题片段 ID，使用换行或逗号分隔（最多 1000 个）").fill("segment-changed");
  await page.getByRole("button", { name: "继续确认原批次" }).click();
  await expect.poll(() => requests.length).toBe(2);
  expect(requests[1]).toEqual(requests[0]);
  expect(JSON.parse(requests[0].body).idempotency_key).toBe(requests[0].key);
});
