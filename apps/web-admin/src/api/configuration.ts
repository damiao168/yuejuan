import { apiClient } from "./client";

export interface LayoutRegion {
  id: string;
  question_id?: string;
  label?: string;
  x: number;
  y: number;
  width: number;
  height: number;
}

export interface TemplatePage {
  page_no: number;
  width: number;
  height: number;
  registration_marks: LayoutRegion[];
  identity_regions: LayoutRegion[];
  question_regions: LayoutRegion[];
}

export interface TemplateLayout {
  pages: TemplatePage[];
}

export interface AnswerSheetTemplate {
  id: string;
  tenant_id: string;
  exam_id: string;
  exam_paper_id: string;
  version_no: number;
  revision: number;
  name: string;
  status: "draft" | "locked" | "retired";
  page_count: number;
  layout: TemplateLayout;
  content_hash: string;
  created_by: string;
  locked_by?: string;
  locked_at?: string;
  created_at: string;
  updated_at: string;
}

export interface TemplatePayload {
  exam_paper_id: string;
  name: string;
  page_count: number;
  layout: TemplateLayout;
}

export interface ReadinessCheck {
  code: string;
  label: string;
  passed: boolean;
  severity: "blocker" | "warning";
  message: string;
  section: string;
}

export interface ExamReadiness {
  ready: boolean;
  confirmed: boolean;
  configuration_hash: string;
  checks: ReadinessCheck[];
  confirmed_at?: string;
  confirmed_by?: string;
}

export async function listAnswerSheetTemplates(examId: string) {
  return apiClient.request<{ templates: AnswerSheetTemplate[] }>(`/api/v1/exams/${encodeURIComponent(examId)}/answer-sheet-templates`);
}

export async function createAnswerSheetTemplate(examId: string, payload: TemplatePayload) {
  return apiClient.request<{ template: AnswerSheetTemplate }>(`/api/v1/exams/${encodeURIComponent(examId)}/answer-sheet-templates`, {
    method: "POST",
    body: JSON.stringify(payload)
  });
}

export async function updateAnswerSheetTemplate(templateId: string, payload: TemplatePayload, expectedRevision: number) {
  return apiClient.request<{ template: AnswerSheetTemplate }>(`/api/v1/answer-sheet-templates/${encodeURIComponent(templateId)}`, {
    method: "PATCH",
    body: JSON.stringify({ name: payload.name, page_count: payload.page_count, layout: payload.layout, expected_revision: expectedRevision })
  });
}

export async function lockAnswerSheetTemplate(templateId: string) {
  return apiClient.request<{ template: AnswerSheetTemplate }>(`/api/v1/answer-sheet-templates/${encodeURIComponent(templateId)}/lock`, { method: "POST" });
}

export async function cloneAnswerSheetTemplate(templateId: string) {
  return apiClient.request<{ template: AnswerSheetTemplate }>(`/api/v1/answer-sheet-templates/${encodeURIComponent(templateId)}/clone`, { method: "POST" });
}

export async function getExamReadiness(examId: string) {
  return apiClient.request<{ readiness: ExamReadiness }>(`/api/v1/exams/${encodeURIComponent(examId)}/readiness`);
}

export async function confirmExamReadiness(examId: string) {
  return apiClient.request<{ readiness: ExamReadiness }>(`/api/v1/exams/${encodeURIComponent(examId)}/readiness/confirm`, { method: "POST" });
}

export async function startExamCollection(examId: string) {
  return apiClient.request<{ readiness: ExamReadiness; status: string }>(`/api/v1/exams/${encodeURIComponent(examId)}/start-collection`, { method: "POST" });
}
