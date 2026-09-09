import { createSubjectiveGradingBatch, recoverSubjectiveBatchCommand } from "../../api/subjectiveGrading";

export async function submitSubjectiveBatchCommand(storage: Pick<Storage, "getItem" | "setItem" | "removeItem">, key: string, draft: { commandId: string; segmentIds: string[] }, transport = { create: createSubjectiveGradingBatch, recover: recoverSubjectiveBatchCommand }) {
  const raw = storage.getItem(key);
  const command: typeof draft = raw ? JSON.parse(raw) : { commandId: draft.commandId, segmentIds: [...draft.segmentIds] };
  if (!command.commandId || !Array.isArray(command.segmentIds) || !command.segmentIds.length || command.segmentIds.some(id => typeof id !== "string")) throw new Error("批次命令或答题片段列表无效");
  if (!raw) storage.setItem(key, JSON.stringify(command));
  const recovered = raw ? await transport.recover(command.commandId) : undefined;
  if (recovered && recovered.command_id !== command.commandId) throw new Error("批次命令恢复结果不匹配");
  const result = recovered?.status === "succeeded" && recovered.batch ? { batch: recovered.batch } : await transport.create(command.commandId, command.segmentIds);
  // Save the result first so refresh can recover both the batch and its enqueue plan.
  storage.setItem(`${key}:batch`, result.batch.id);
  storage.removeItem(key);
  return result;
}
