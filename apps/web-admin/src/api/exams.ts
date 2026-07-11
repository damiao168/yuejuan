import { apiClient } from "./client";

export type ExamStatus = "draft" | "configured" | "ready" | "collecting" | "grading" | "reviewing" | "finalized" | "published" | "archived";

export type GradingMode = "auto_objective_only" | "ai_assisted" | "human_review_required" | "double_mark" | "blind_double_mark";

export interface Exam {
  id: string;
  tenant_id: string;
  school_id: string;
  name: string;
  subject: string;
  exam_type: string;
  total_score: number;
  status: ExamStatus | string;
  grading_mode: GradingMode | string;
  appeal_enabled: boolean;
  publish_policy: string;
  created_by: string;
  class_ids: string[];
  created_at?: string;
}

export interface ExamListFilter {
  status?: string;
  school_id?: string;
}

export interface ExamPayload {
  school_id: string;
  name: string;
  subject: string;
  exam_type: string;
  total_score: number;
  grading_mode: string;
  appeal_enabled: boolean;
  publish_policy: string;
  class_ids: string[];
}

function queryString(filter: ExamListFilter) {
  const params = new URLSearchParams();
  if (filter.status) {
    params.set("status", filter.status);
  }
  if (filter.school_id) {
    params.set("school_id", filter.school_id);
  }
  const query = params.toString();
  return query ? `?${query}` : "";
}

export async function listExams(filter: ExamListFilter = {}) {
  return apiClient.request<{ exams: Exam[] }>(`/api/v1/exams${queryString(filter)}`);
}

export async function getExam(id: string) {
  return apiClient.request<{ exam: Exam }>(`/api/v1/exams/${encodeURIComponent(id)}`);
}

export async function createExam(payload: ExamPayload) {
  return apiClient.request<{ exam: Exam }>("/api/v1/exams", {
    method: "POST",
    body: JSON.stringify(payload)
  });
}

export async function updateExam(id: string, payload: Partial<ExamPayload>) {
  return apiClient.request<{ exam: Exam }>(`/api/v1/exams/${encodeURIComponent(id)}`, {
    method: "PATCH",
    body: JSON.stringify(payload)
  });
}

export async function updateExamStatus(id: string, status: string) {
  return apiClient.request<{ exam: Exam }>(`/api/v1/exams/${encodeURIComponent(id)}/status`, {
    method: "POST",
    body: JSON.stringify({ status })
  });
}

export async function archiveExam(id: string) {
  return apiClient.request<{ exam: Exam }>(`/api/v1/exams/${encodeURIComponent(id)}/archive`, {
    method: "POST"
  });
}
