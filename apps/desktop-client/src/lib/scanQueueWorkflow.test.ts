import { describe, expect, it } from "vitest";
import type { SyncQueueItem } from "../types";
import { transitionScanQueueItem } from "./scanQueueWorkflow";

const item = (status: SyncQueueItem["status"]): SyncQueueItem => ({
  id: "scan-1", title: "page.tif", kind: "scan_upload", status,
  progress: 0, detail: "queued", updatedAt: "2026-01-01T00:00:00.000Z"
});

describe("durable scan queue workflow", () => {
  it("allows resumable upload transitions and stamps one projection update", () => {
    const uploading = transitionScanQueueItem(item("pending"), { status: "uploading", progress: 30 }, new Date("2026-09-07T00:00:00Z"));
    const interrupted = transitionScanQueueItem(uploading, { status: "pending", confirmedOffset: 1024 });
    expect(uploading.updatedAt).toBe("2026-09-07T00:00:00.000Z");
    expect(interrupted).toMatchObject({ status: "pending", confirmedOffset: 1024 });
  });

  it("rejects reopening a server-confirmed upload", () => {
    expect(() => transitionScanQueueItem(item("succeeded"), { status: "uploading" })).toThrow("succeeded -> uploading");
  });
});
