import type { AnswerGroup, AnswerGroupMember } from "@edugrade/sdk";

export function sampleRoleLabels(member: AnswerGroupMember): string[] {
  const labels: string[] = [];
  if (member.representative) labels.push("代表样本");
  if (member.boundary) labels.push("边界样本");
  if (member.outlier) labels.push("异常样本");
  return labels;
}

export function confirmBlockReason(group?: AnswerGroup): string | null {
  if (!group) return "请先选择答案组";
  if (group.status === "confirmed") return "该组已确认，如需撤销请使用回滚";
  if (group.status === "rolled_back") return "该组已回滚，不能再次确认";
  if (!group.decision) return "请先保存组评分候选";
  if (group.reviewed_sample_count < group.minimum_sample) {
    return `抽检未完成：已检查 ${group.reviewed_sample_count} / 至少 ${group.minimum_sample}`;
  }
  if (group.members.some((member) => member.outlier && !member.sample_status)) return "异常样本尚未全部检查";
  if (group.members.some((member) => member.sample_status === "rejected")) return "存在被拒绝样本，请转单份人工处理";
  return group.can_confirm ? null : "当前组状态不允许确认";
}
