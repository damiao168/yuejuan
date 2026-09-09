import { expect, it, vi } from "vitest";
import { submitSubjectiveBatchCommand } from "./subjectiveBatchCommand";
import type { SubjectiveGradingBatch } from "../../api/subjectiveGrading";

it("persists the original segments and command across response loss, reload and draft edits", async () => {
  const values = new Map<string, string>();
  const storage = { getItem: (k: string) => values.get(k) ?? null, setItem: (k: string, v: string) => { values.set(k, v); }, removeItem: (k: string) => { values.delete(k); } };
  const create = vi.fn(async () => { throw new Error("503"); });
  const recover = vi.fn(async (id: string) => ({ command_id: id, status: "not_accepted" as const }));
  await expect(submitSubjectiveBatchCommand(storage, "actor-1", { commandId: "original", segmentIds: ["segment-1"] }, { create, recover })).rejects.toThrow("503");
  await expect(submitSubjectiveBatchCommand(storage, "actor-1", { commandId: "changed", segmentIds: ["segment-2"] }, { create, recover })).rejects.toThrow("503");
  expect(create.mock.calls).toEqual([["original", ["segment-1"]], ["original", ["segment-1"]]]);
  const batch = { id: "batch-1" } as SubjectiveGradingBatch;
  const accepted = async (id: string) => ({ command_id: id, status: "succeeded" as const, batch });
  expect(await submitSubjectiveBatchCommand(storage, "actor-1", { commandId: "changed", segmentIds: [] }, { create, recover: accepted })).toEqual({ batch });
  expect(values.get("actor-1:batch")).toBe("batch-1");
  expect(values.has("actor-1")).toBe(false);
  expect(create).toHaveBeenCalledTimes(2);
});

it("does not submit when the stored command is corrupt", async () => {
  const create = vi.fn(async () => { throw new Error("should not send"); });
  const recover = vi.fn(async (id: string) => ({ command_id: id, status: "not_accepted" as const }));
  const storage = { getItem: () => "{}", setItem: vi.fn(), removeItem: vi.fn() };
  await expect(submitSubjectiveBatchCommand(storage, "actor", { commandId: "new", segmentIds: ["s"] }, { create, recover })).rejects.toThrow();
  expect(create).not.toHaveBeenCalled();
  expect(storage.setItem).not.toHaveBeenCalled();
});
