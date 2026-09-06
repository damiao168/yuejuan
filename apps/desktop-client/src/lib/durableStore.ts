import { invoke } from "@tauri-apps/api/core";
import { sha256ForFile, type CaptureUploadSource } from "../api/captureUploads";
import type { OfflineDraftRecord, OfflineSyncStatus, ScanQualityCheck, SyncQueueItem } from "../types";
import { isTauriRuntime } from "./localRuntime";

/**
 * Native storage boundary for the Windows scan station.
 *
 * The Tauri implementation owns SQLite, encryption and filesystem paths. Browser
 * development is deliberately not a replacement for this store: callers must
 * choose an explicit development fallback where one is still supported.
 */
export interface DurableStoreStatus {
  ready: boolean;
  databasePath: string;
  spoolPath: string;
}

export interface SpoolAssetInput {
  file: File;
  examId?: string;
  captureBatchId?: string;
  submissionId?: string;
  pageNo?: number;
  qualityChecks?: ScanQualityCheck[];
}

export interface DurableSpoolFile {
  filename: string;
  mime: string;
  size: number;
  sha256: string;
  chunkSize: number;
}

interface SpoolAssetSession {
  localAssetId: string;
  chunkSize: number;
  confirmedOffset: number;
  item?: SyncQueueItem;
}

export function hasDurableDesktopStore() {
  return isTauriRuntime();
}

export async function spoolScanAsset(input: SpoolAssetInput): Promise<SyncQueueItem> {
  requireDurableRuntime();
  const sha256 = await sha256ForFile(input.file);
  const session = await invoke<SpoolAssetSession>("begin_spool_local_asset", {
    input: {
      filename: input.file.name,
      mime: input.file.type || inferContentType(input.file.name),
      size: input.file.size,
      sha256,
      examId: input.examId,
      captureBatchId: input.captureBatchId,
      submissionId: input.submissionId,
      pageNo: input.pageNo,
      qualityChecks: input.qualityChecks
    }
  });
  if (session.item) return session.item;
  let offset = session.confirmedOffset;
  while (offset < input.file.size) {
    const end = Math.min(offset + session.chunkSize, input.file.size);
    const bytes = Array.from(new Uint8Array(await input.file.slice(offset, end).arrayBuffer()));
    offset = await invoke<number>("write_spool_local_asset_chunk", {
      localAssetId: session.localAssetId,
      offset,
      bytes
    });
  }
  return invoke<SyncQueueItem>("complete_spool_local_asset", { localAssetId: session.localAssetId });
}

export async function listDurableScanQueue(): Promise<SyncQueueItem[]> {
  requireDurableRuntime();
  return invoke<SyncQueueItem[]>("list_durable_scan_queue");
}

export async function persistDurableScanQueueItem(item: SyncQueueItem): Promise<void> {
  requireDurableRuntime();
  await invoke("persist_durable_scan_queue_item", { item: withoutPreview(item) });
}

export async function archiveDurableScanQueueItems(ids: string[]): Promise<void> {
  requireDurableRuntime();
  await invoke("archive_durable_scan_queue_items", { ids });
}

export async function loadDurableSpoolFile(localAssetId: string): Promise<CaptureUploadSource> {
  requireDurableRuntime();
  const stored = await invoke<DurableSpoolFile>("read_durable_local_asset", { localAssetId });
  return {
    name: stored.filename,
    type: stored.mime,
    size: stored.size,
    sha256: stored.sha256,
    async slice(start: number, end: number) {
      if (start < 0 || end < start || end > stored.size) throw new Error("本地扫描原件读取范围无效");
      const parts: BlobPart[] = [];
      let offset = start;
      while (offset < end) {
        const length = Math.min(stored.chunkSize, end - offset);
        const bytes = await invoke<number[]>("read_durable_local_asset_chunk", { localAssetId, offset, length });
        if (bytes.length !== length) throw new Error("本地扫描原件分块不完整");
        parts.push(new Uint8Array(bytes));
        offset += bytes.length;
      }
      return new Blob(parts, { type: stored.mime });
    }
  };
}

export async function saveDurableDraft(record: OfflineDraftRecord): Promise<void> {
  requireDurableRuntime();
  await invoke("save_durable_draft", { record });
}

export async function listDurableDraftEnvelopes(): Promise<DurableDraftEnvelope[]> {
  requireDurableRuntime();
  return invoke<DurableDraftEnvelope[]>("list_durable_drafts");
}

export async function loadDurableDraft(taskId: string): Promise<OfflineDraftRecord | null> {
  requireDurableRuntime();
  return invoke<OfflineDraftRecord | null>("load_durable_draft", { taskId });
}

export async function updateDurableDraftStatus(taskId: string, patch: { syncStatus: OfflineSyncStatus; syncMessage?: string }): Promise<void> {
  requireDurableRuntime();
  await invoke("update_durable_draft_status", { taskId, syncStatus: patch.syncStatus, syncMessage: patch.syncMessage });
}

export async function purgeExpiredDurableDrafts(now = new Date()): Promise<number> {
  requireDurableRuntime();
  return invoke<number>("purge_expired_durable_drafts", { now: now.toISOString() });
}

export interface DurableDraftEnvelope {
  taskId: string;
  anonymousCode: string;
  savedAt: string;
  expiresAt: string;
  syncStatus: OfflineSyncStatus;
  syncMessage?: string;
}

function withoutPreview(item: SyncQueueItem): SyncQueueItem {
  const { previewUrl: _previewUrl, ...stored } = item;
  return stored;
}

function requireDurableRuntime() {
  if (!isTauriRuntime()) {
    throw new Error("耐久扫描存储仅在 Windows 桌面客户端可用；浏览器开发模式不能作为生产扫描队列。");
  }
}

function inferContentType(filename: string) {
  const extension = filename.slice(filename.lastIndexOf(".")).toLowerCase();
  if (extension === ".pdf") return "application/pdf";
  if (extension === ".png") return "image/png";
  if (extension === ".jpg" || extension === ".jpeg") return "image/jpeg";
  if (extension === ".tif" || extension === ".tiff") return "image/tiff";
  return "application/octet-stream";
}
