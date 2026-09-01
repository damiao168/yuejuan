import type { StatusTone } from "../types";

export const examStatusLabels: Record<string, string> = {
  draft: "草稿",
  configured: "配置中",
  ready: "准备完成",
  collecting: "采集中",
  grading: "阅卷中",
  reviewing: "复核中",
  finalized: "待发布",
  published: "已发布",
  archived: "已归档"
};

export const examSubjectOptions = [
  { label: "语文", value: "chinese" },
  { label: "数学", value: "math" },
  { label: "英语", value: "english" },
  { label: "物理", value: "physics" },
  { label: "化学", value: "chemistry" },
  { label: "生物", value: "biology" },
  { label: "历史", value: "history" },
  { label: "地理", value: "geography" },
  { label: "政治", value: "politics" }
];

export function examSubjectLabel(subject: string): string {
  const canonicalLabels: Record<string, string> = {
    mathematics: "数学",
    ethics_politics: "政治"
  };
  return canonicalLabels[subject] ?? examSubjectOptions.find((item) => item.value === subject)?.label ?? "其他学科";
}

export function examStatusTone(status: string): StatusTone {
  if (status === "published" || status === "finalized") {
    return "success";
  }
  if (status === "archived") {
    return "neutral";
  }
  if (status === "draft" || status === "configured") {
    return "info";
  }
  if (status === "reviewing") {
    return "warning";
  }
  return "processing";
}
