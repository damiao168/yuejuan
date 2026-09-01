import { describe, expect, it } from "vitest";
import { offlineSyncStatusLabels, queueStatusLabels } from "./statusLabels";

describe("desktop user-facing status labels", () => {
  it("localizes upload queue states", () => {
    expect(queueStatusLabels).toMatchObject({
      pending: "等待上传",
      uploading: "上传中",
      succeeded: "已完成",
      failed: "上传失败"
    });
  });

  it("localizes offline draft sync states", () => {
    expect(offlineSyncStatusLabels).toEqual({
      draft: "本地草稿",
      syncing: "同步中",
      synced: "已同步",
      failed: "同步失败",
      conflict: "存在冲突"
    });
  });
});
