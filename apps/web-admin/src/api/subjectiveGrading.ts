import { apiClient } from "./client";

export interface SubjectiveGradingBatch {
  id: string;
  tenant_id: string;
  idempotency_key: string;
  status: "planned" | "processing" | "completed" | "failed" | "cancelled";
  segment_ids: string[];
  total_count: number;
  queued_count: number;
  processing_count: number;
  succeeded_count: number;
  failed_count: number;
  created_by: string;
  created_at: string;
  updated_at: string;
}

export interface SubjectiveGradingBatchEnqueueResult {
  requested_count: number;
  accepted_count: number;
  task_count: number;
  failed_count: number;
  partial_success: boolean;
  failures: Array<{
    segment_id: string;
    code: string;
  }>;
}

export async function createSubjectiveGradingBatch(idempotencyKey: string, segmentIds: string[]) {
  return apiClient.request<{ batch: SubjectiveGradingBatch }>("/api/v1/subjective-grading-batches", {
    method: "POST",
    body: JSON.stringify({ idempotency_key: idempotencyKey, segment_ids: segmentIds })
  });
}

export async function enqueueSubjectiveGradingBatch(batchId: string) {
  return apiClient.request<{
    batch: SubjectiveGradingBatch;
    tasks: Array<{ id: string; status: string }>;
    enqueue_result: SubjectiveGradingBatchEnqueueResult;
  }>(`/api/v1/subjective-grading-batches/${encodeURIComponent(batchId)}/enqueue`, { method: "POST" });
}

export async function getSubjectiveGradingBatch(batchId: string, signal?: AbortSignal) {
  return apiClient.request<{ batch: SubjectiveGradingBatch }>(`/api/v1/subjective-grading-batches/${encodeURIComponent(batchId)}`, { signal });
}
