import { expect, it, vi } from "vitest";
import { submitScoringCommand } from "./scoringCommand";
import type { startScoringRun } from "../../../../api/review";

const run = { id: "run-1" } as Awaited<ReturnType<typeof startScoringRun>>["scoring_run"];
function fixture() {
  const values = new Map<string, string>();
  const storage = { getItem: (key: string) => values.get(key) ?? null, setItem: (key: string, value: string) => { values.set(key, value); }, removeItem: (key: string) => { values.delete(key); } };
  return { values, options: { examId: "exam-1", storageKey: "tenant:actor:exam-1", storage, ready: vi.fn(async () => true), pending: vi.fn() } };
}

it("keeps the original command after an uncertain response and reload, without rerunning readiness", async () => {
  const { options, values } = fixture();
  const start = vi.fn(async (_exam: string, id: string) => {
    expect(values.get(options.storageKey)).toBe(id);
    throw new Error("503 after commit");
  });
  const recover = vi.fn(async (_exam: string, id: string) => ({ command_id: id, status: "not_accepted" as const }));
  await expect(submitScoringCommand(options, { start, recover })).rejects.toThrow("503");
  const id = values.get(options.storageKey);
  await expect(submitScoringCommand({ ...options }, { start, recover })).rejects.toThrow("503");
  expect(start.mock.calls.map((call) => call[1])).toEqual([id, id]);
  expect(options.ready).toHaveBeenCalledTimes(1);
});

it("recovers committed creation without creating again, then gives a new explicit operation a new ID", async () => {
  const { options, values } = fixture();
  values.set(options.storageKey, "old-command");
  const start = vi.fn(async () => ({ scoring_run: run }));
  const recover = vi.fn(async (_exam: string, id: string) => ({ command_id: id, status: "succeeded" as const, scoring_run: run }));
  expect(await submitScoringCommand(options, { start, recover })).toBe(true);
  expect(start).not.toHaveBeenCalled();
  expect(values.size).toBe(0);
  await submitScoringCommand(options, { start, recover });
  expect(start).toHaveBeenCalledTimes(1);
  expect(options.pending.mock.calls.some(([id]) => id !== null && id !== "old-command")).toBe(true);
});

it("does not send a command when persistence fails", async () => {
  const { options } = fixture();
  options.storage.setItem = () => { throw new Error("storage full"); };
  const start = vi.fn(async () => ({ scoring_run: run }));
  const recover = vi.fn(async (_exam: string, id: string) => ({ command_id: id, status: "not_accepted" as const }));
  await expect(submitScoringCommand(options, { start, recover })).rejects.toThrow("storage full");
  expect(start).not.toHaveBeenCalled();
});
