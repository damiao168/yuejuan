import type { Exam } from "../types";
import type { DesktopApiClient } from "./client";

export interface ExamListFilter {
  status?: string;
  school_id?: string;
}

/** Compact capture-batch shape used by the scan station to select a real
 * server batch. The native client never invents a batch UUID locally. */
export interface CaptureBatch {
  id: string;
  exam_id: string;
  name: string;
  source_type: string;
  status: "draft" | "uploading" | "processing" | "matching" | "completed" | "cancelled" | string;
  file_count: number;
  page_count: number;
  created_at: string;
}

export async function listExams(client: DesktopApiClient, filter: ExamListFilter = {}) {
  const params = new URLSearchParams();
  if (filter.status) {
    params.set("status", filter.status);
  }
  if (filter.school_id) {
    params.set("school_id", filter.school_id);
  }
  const query = params.toString();
  return client.request<{ exams: Exam[] }>(`/api/v1/exams${query ? `?${query}` : ""}`);
}

export async function listCaptureBatches(client: DesktopApiClient, examId: string) {
  const encodedExamID = encodeURIComponent(examId);
  return client.request<{ batches: CaptureBatch[]; next_cursor?: string; has_more?: boolean }>(
    `/api/v1/exams/${encodedExamID}/capture-batches?limit=100`
  );
}
