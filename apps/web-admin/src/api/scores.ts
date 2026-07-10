import { apiClient } from "./client";

export interface FinalGrade {
  id: string;
  tenant_id: string;
  exam_id: string;
  question_id: string;
  question_no: string;
  answer_segment_id: string;
  submission_id: string;
  anonymous_code: string;
  score: number;
  max_score: number;
  source: string;
  status: string;
  locked: boolean;
  created_by: string;
  created_at: string;
  updated_at: string;
}

export interface SubmissionGrade {
  id: string;
  tenant_id: string;
  exam_id: string;
  submission_id: string;
  student_id?: string;
  anonymous_code: string;
  total_score: number;
  max_score: number;
  status: string;
  locked: boolean;
  confirmed_by?: string;
  confirmed_at?: string;
  published_by?: string;
  published_at?: string;
  created_by: string;
  created_at: string;
  updated_at: string;
  items?: FinalGrade[];
}

export interface QualityIssue {
  code: string;
  message: string;
  blocking: boolean;
  count: number;
}

export interface QualityReport {
  passed: boolean;
  issues: QualityIssue[];
}

export interface FinalizeResult {
  status: string;
  created_finals: number;
  submission_grades: SubmissionGrade[];
  quality: QualityReport;
  available_statuses: string[];
}

export interface PublishResult {
  status: string;
  submission_grades: SubmissionGrade[];
  quality: QualityReport;
  published_at: string;
}

export interface QualityCheckResult {
  stage: string;
  can_publish: boolean;
  quality: QualityReport;
}

export interface ScoreExport {
  blob: Blob;
  contentType: string;
  filename?: string;
  watermark?: string;
}

export async function finalizeExamGrades(examId: string) {
  return apiClient.request<FinalizeResult>(`/api/v1/exams/${encodeURIComponent(examId)}/finalize`, {
    method: "POST"
  });
}

export async function listExamGrades(examId: string) {
  return apiClient.request<{ grades: SubmissionGrade[] }>(`/api/v1/exams/${encodeURIComponent(examId)}/grades`);
}

export async function checkExamGradeQuality(examId: string, stage: "confirmation" | "publish" = "publish") {
  const query = stage === "publish" ? "?stage=publish" : "";
  return apiClient.request<QualityCheckResult>(`/api/v1/exams/${encodeURIComponent(examId)}/grades/quality${query}`);
}

export async function confirmExamGrades(examId: string, reason: string) {
  return apiClient.request<{ grades: SubmissionGrade[] }>(`/api/v1/exams/${encodeURIComponent(examId)}/confirm-grades`, {
    method: "POST",
    body: JSON.stringify({ reason })
  });
}

export async function publishExamGrades(examId: string, reason: string) {
  return apiClient.request<PublishResult>(`/api/v1/exams/${encodeURIComponent(examId)}/publish`, {
    method: "POST",
    body: JSON.stringify({ reason })
  });
}

export async function exportExamGrades(examId: string): Promise<ScoreExport> {
  return apiClient.requestBlob(`/api/v1/exams/${encodeURIComponent(examId)}/grades/export`);
}
