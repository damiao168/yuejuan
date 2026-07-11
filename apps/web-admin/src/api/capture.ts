import { apiClient } from "./client";

export interface CaptureBatch {
  id: string;
  tenant_id: string;
  exam_id: string;
  name: string;
  source_type: "web_upload" | "scanner_upload" | "folder_import" | "desktop_sync";
  status: "draft" | "uploading" | "matching" | "processing" | "needs_review" | "ready" | "completed" | "cancelled";
  revision: number;
  operator_id: string;
  scanner_device?: string;
  file_count: number;
  page_count: number;
  submission_count: number;
  normal_count: number;
  review_count: number;
  failed_count: number;
  started_at?: string;
  completed_at?: string;
  created_at: string;
}

export interface CaptureFile {
  id: string;
  capture_batch_id: string;
  file_asset_id: string;
  original_name: string;
  content_type: string;
  sha256: string;
  byte_size: number;
  page_count: number;
  status: string;
  error_code?: string;
  created_at: string;
}

export interface CapturePage {
  id: string;
  capture_batch_id: string;
  capture_file_id: string;
  source_index: number;
  submission_id?: string;
  submission_page_id?: string;
  assigned_page_no?: number;
  sequence_no: number;
  rotation_degrees: number;
  decoded_file_asset_id: string;
  status: string;
  revision: number;
  page_identity: Record<string, unknown>;
}

export interface CaptureBatchDetail {
  batch: CaptureBatch;
  files: CaptureFile[];
  pages: CapturePage[];
}

export interface RegistrationRun {
  id: string;
  capture_page_id: string;
  submission_page_id: string;
  template_id: string;
  page_no: number;
  processing_status: string;
  match_status?: string;
  confidence?: number;
  method?: string;
  registered_file_asset_id?: string;
  error_code?: string;
}

export async function createCaptureBatch(examId: string, payload: { name: string; source_type: CaptureBatch["source_type"]; scanner_device?: string; idempotency_key: string }) {
  return apiClient.request<{ batch: CaptureBatch }>(`/api/v1/exams/${encodeURIComponent(examId)}/capture-batches`, { method: "POST", body: JSON.stringify(payload) });
}

export async function listCaptureBatches(examId: string) {
  return apiClient.request<{ batches: CaptureBatch[] }>(`/api/v1/exams/${encodeURIComponent(examId)}/capture-batches`);
}

export async function getCaptureBatch(batchId: string) {
  return apiClient.request<CaptureBatchDetail>(`/api/v1/capture-batches/${encodeURIComponent(batchId)}`);
}

export async function registerCaptureFile(batchId: string, fileAssetId: string, idempotencyKey: string) {
  return apiClient.request<{ file: CaptureFile }>(`/api/v1/capture-batches/${encodeURIComponent(batchId)}/files`, { method: "POST", body: JSON.stringify({ file_asset_id: fileAssetId, idempotency_key: idempotencyKey }) });
}

export async function processCaptureBatch(batchId: string) {
  return apiClient.request<{ batch: CaptureBatch }>(`/api/v1/capture-batches/${encodeURIComponent(batchId)}/process`, { method: "POST" });
}

export async function updateCapturePage(pageId: string, payload: { revision: number; rotation_degrees?: number; sequence_no?: number }) {
  return apiClient.request<{ page: CapturePage }>(`/api/v1/capture-pages/${encodeURIComponent(pageId)}`, { method: "PATCH", body: JSON.stringify(payload) });
}

export async function processSubmissionPages(submissionId: string) {
  return apiClient.request<{ runs: RegistrationRun[] }>(`/api/v1/submissions/${encodeURIComponent(submissionId)}/process-pages`, { method: "POST" });
}
