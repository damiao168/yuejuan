import type { OfflineDraftEnvelope } from "./lib/offlineStore";
import type { SyncQueueItem } from "./types";

export const queueStatusLabels: Record<SyncQueueItem["status"], string> = {
  pending: "等待上传",
  uploading: "上传中",
  succeeded: "已完成",
  failed: "上传失败",
  conflict: "需要处理",
  not_configured: "未配置/待接入"
};

export const offlineSyncStatusLabels: Record<OfflineDraftEnvelope["syncStatus"], string> = {
  draft: "本地草稿",
  syncing: "同步中",
  synced: "已同步",
  failed: "同步失败",
  conflict: "存在冲突"
};

export const genericStatusLabels: Record<string, string> = {
  draft: "草稿",
  planned: "待处理",
  pending: "等待处理",
  queued: "排队中",
  assigned: "已分配",
  in_progress: "处理中",
  processing: "处理中",
  uploading: "上传中",
  uploaded: "已上传",
  completed: "已完成",
  succeeded: "已完成",
  failed: "失败",
  cancelled: "已取消",
  collecting: "采集中",
  grading: "阅卷中",
  reviewing: "复核中",
  finalized: "待发布",
  published: "已发布",
  archived: "已归档"
};

export function genericStatusLabel(status: string): string {
  return genericStatusLabels[status] ?? "未知状态";
}

export const subjectLabels: Record<string, string> = {
  chinese: "语文",
  mathematics: "数学",
  math: "数学",
  english: "英语",
  physics: "物理",
  chemistry: "化学",
  biology: "生物",
  history: "历史",
  geography: "地理",
  politics: "政治",
  ethics_politics: "政治"
};
