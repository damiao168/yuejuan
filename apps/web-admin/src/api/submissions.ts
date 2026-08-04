import { apiClient } from "./client";

export interface QualityIssue {
  code: string;
  message: string;
}

export interface SubmissionPage {
  id: string;
  tenant_id: string;
  submission_id: string;
  file_asset_id: string;
  page_no: number;
  status: string;
  quality_issues: QualityIssue[];
  created_at: string;
}

export interface Submission {
  id: string;
  tenant_id: string;
  exam_id: string;
  student_id?: string;
  candidate_no?: string;
  source_type: string;
  status: string;
  expected_page_count: number;
  actual_page_count: number;
  quality_status: string;
  quality_issues: QualityIssue[];
  collected_by: string;
  revision: number;
  created_at: string;
  pages?: SubmissionPage[];
  summary?: Record<string, unknown>;
}

export interface CreateSubmissionPayload {
  student_id?: string;
  candidate_no?: string;
  source_type: string;
  expected_page_count: number;
}

export interface OcrResult {
  id: string;
  tenant_id: string;
  ocr_task_id: string;
  submission_id: string;
  submission_page_id: string;
  text: string;
  bbox: number[];
  confidence: number;
  ocr_engine: string;
  ocr_version: string;
  source_image_file_id?: string;
  created_at: string;
}

export interface OcrTask {
  id: string;
  tenant_id: string;
  submission_id: string;
  status: string;
  engine: string;
  engine_version: string;
  min_confidence: number;
  result_count: number;
  requires_human_review: boolean;
  error_message?: string;
  requested_by: string;
  started_at?: string;
  completed_at?: string;
  created_at: string;
  results?: OcrResult[];
}

export interface AnswerSegment {
  id: string;
  tenant_id: string;
  submission_id: string;
  submission_page_id: string;
  question_id: string;
  question_no: string;
  bbox: number[];
  source: string;
  status: string;
  review_notes?: string;
  reviewed_by?: string;
  reviewed_at?: string;
  created_at: string;
  crop_file_asset_id?: string;
  crop_sha256?: string;
  processing_status?: string;
  confidence?: number;
}

export async function downloadAnswerSegmentImage(segmentId: string) {
  return apiClient.requestBlob(`/api/v1/answer-segments/${encodeURIComponent(segmentId)}/image`);
}

export interface SegmentIssue {
  code: string;
  message: string;
}

export interface SegmentResult {
  valid: boolean;
  issues: SegmentIssue[];
  segments: AnswerSegment[];
}

export async function createSubmission(examId: string, payload: CreateSubmissionPayload) {
  return apiClient.request<{ submission: Submission }>(`/api/v1/exams/${encodeURIComponent(examId)}/submissions`, {
    method: "POST",
    body: JSON.stringify(payload)
  });
}

export async function listSubmissions(examId: string, filter: { limit?: number; cursor?: string } = {}) {
  const params = new URLSearchParams();
  if (filter.limit) params.set("limit", String(filter.limit));
  if (filter.cursor) params.set("cursor", filter.cursor);
  const query = params.toString();
  return apiClient.request<{ submissions: Submission[]; next_cursor: string; has_more: boolean }>(`/api/v1/exams/${encodeURIComponent(examId)}/submissions${query ? `?${query}` : ""}`);
}

export async function getSubmission(submissionId: string) {
  return apiClient.request<{ submission: Submission }>(`/api/v1/submissions/${encodeURIComponent(submissionId)}`);
}

export async function addSubmissionPage(submissionId: string, fileAssetId: string, pageNo: number) {
  return apiClient.request<{ page: SubmissionPage }>(`/api/v1/submissions/${encodeURIComponent(submissionId)}/pages`, {
    method: "POST",
    body: JSON.stringify({ file_asset_id: fileAssetId, page_no: pageNo })
  });
}

export async function replaceSubmissionPage(submissionId: string, pageNo: number, fileAssetId: string) {
  return apiClient.request<{ page: SubmissionPage }>(`/api/v1/submissions/${encodeURIComponent(submissionId)}/pages/${pageNo}`, {
    method: "PUT",
    body: JSON.stringify({ file_asset_id: fileAssetId })
  });
}

export async function listSubmissionPages(submissionId: string) {
  return apiClient.request<{ pages: SubmissionPage[] }>(`/api/v1/submissions/${encodeURIComponent(submissionId)}/pages`);
}

export async function runQualityCheck(submissionId: string) {
  return apiClient.request<{ result: { valid: boolean; issues: QualityIssue[] } }>(`/api/v1/submissions/${encodeURIComponent(submissionId)}/quality-check`, {
    method: "POST"
  });
}

export async function updateSubmissionStatus(submissionId: string, status: string, expectedRevision: number) {
  return apiClient.request<{ submission: Submission }>(`/api/v1/submissions/${encodeURIComponent(submissionId)}/status`, {
    method: "POST",
    body: JSON.stringify({ status, expected_revision: expectedRevision })
  });
}

export async function createOcrTask(submissionId: string) {
  return apiClient.request<{ task: OcrTask }>(`/api/v1/submissions/${encodeURIComponent(submissionId)}/ocr-tasks`, {
    method: "POST",
    body: JSON.stringify({ engine: "external_ocr_worker", engine_version: "not_configured", min_confidence: 0.8 })
  });
}

export async function listOcrTasks(submissionId: string, filter: { limit?: number; cursor?: string } = {}) {
  const params = new URLSearchParams();
  if (filter.limit) params.set("limit", String(filter.limit));
  if (filter.cursor) params.set("cursor", filter.cursor);
  const query = params.toString();
  return apiClient.request<{ tasks: OcrTask[]; next_cursor: string; has_more: boolean }>(
    `/api/v1/submissions/${encodeURIComponent(submissionId)}/ocr-tasks${query ? `?${query}` : ""}`
  );
}

export async function getOcrTask(taskId: string) {
  return apiClient.request<{ task: OcrTask }>(`/api/v1/ocr-tasks/${encodeURIComponent(taskId)}`);
}

export async function generateAnswerSegments(submissionId: string) {
  return apiClient.request<{ result: SegmentResult }>(`/api/v1/submissions/${encodeURIComponent(submissionId)}/segment-answers`, {
    method: "POST"
  });
}

export async function listAnswerSegments(submissionId: string) {
  return apiClient.request<{ segments: AnswerSegment[] }>(`/api/v1/submissions/${encodeURIComponent(submissionId)}/answer-segments`);
}
