import { apiClient } from "./client";

export interface LayoutRegion {
  id: string;
  question_id?: string;
  label?: string;
  x: number;
  y: number;
  width: number;
  height: number;
  option_regions?: OptionRegion[];
  suggestion_confidence?: number;
  suggestion_source?: "pdf_text_anchor" | "ocr_layout";
}

export interface OptionRegion {
  id: string;
  label: string;
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

export interface TemplateOMRReference {
  source: "exam_paper";
  file_asset_id: string;
  hash_sha256: string;
  content_type: string;
}

export interface TemplateOMRProfile {
  mode: "manual_only" | "template_difference";
  version: "opencv-fill-v1" | "opencv-template-difference-bubble-v1";
  reference?: TemplateOMRReference;
}

export interface TemplateLayout {
  pages: TemplatePage[];
  omr_profile?: TemplateOMRProfile;
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

export interface ExamTemplateBinding {
  id: string;
  tenant_id: string;
  exam_id: string;
  template_id: string;
  template_content_hash: string;
  mode: "bound_auto" | "locked_with_guard";
  source: "automatic" | "manual";
  revision: number;
  bound_by: string;
  bound_at: string;
  updated_at: string;
}

export type OMRCalibrationStatus = "draft" | "approved" | "revoked" | "discarded";

export interface OMRCalibrationSummary {
  total_count: number;
  labeled_count: number;
  pending_count: number;
  match_count: number;
  mismatch_count: number;
  eligible_count: number;
  eligible_match_count: number;
  eligible_mismatch_count: number;
  option_coverage: Record<string, number>;
  question_coverage: Record<string, number>;
  stratum_coverage: Record<"selected_high" | "selected_low" | "blank" | "ambiguous", number>;
  ready_to_approve: boolean;
  blockers: string[];
}

export interface OMRCalibrationSession {
  id: string;
  tenant_id: string;
  template_id: string;
  template_content_hash: string;
  scope_type: "question" | "template";
  question_id?: string;
  question_ids: string[];
  question_type: "single_choice" | "true_false" | "template";
  profile_version: string;
  profile_hash: string;
  reference_file_asset_id: string;
  reference_sha256: string;
  option_labels: string[];
  sample_seed: string;
  sample_count: number;
  minimum_samples: number;
  minimum_samples_per_option: number;
  minimum_samples_per_stratum: number;
  minimum_confidence: number;
  inherited_from_session_id?: string;
  status: OMRCalibrationStatus;
  created_by: string;
  created_at: string;
  approved_by?: string;
  approved_at?: string;
  approval_note?: string;
  evidence_hash?: string;
  revoked_by?: string;
  revoked_at?: string;
  revoke_reason?: string;
  discarded_by?: string;
  discarded_at?: string;
  discard_reason?: string;
  updated_at: string;
  summary: OMRCalibrationSummary;
}

export interface OMRCalibrationCase {
  id: string;
  calibration_id: string;
  omr_run_id: string;
  answer_segment_id: string;
  question_id: string;
  question_no: string;
  question_type: "single_choice" | "multiple_choice" | "true_false";
  option_labels: string[];
  sample_stratum: "selected_high" | "selected_low" | "blank" | "ambiguous";
  crop_sha256: string;
  observed_decision: string;
  observed_options: string[];
  observed_confidence: number;
  measurements: Array<Record<string, unknown>>;
  expected_options?: string[];
  matches?: boolean;
  labeled_by?: string;
  labeled_at?: string;
  created_at: string;
  overlay_file_asset_id?: string;
  segment_image_url?: string;
}

export interface OMRCalibrationDetail {
  session: OMRCalibrationSession;
  cases: OMRCalibrationCase[];
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

export async function getExamTemplateBinding(examId: string) {
  return apiClient.request<{ binding: ExamTemplateBinding | null }>(`/api/v1/exams/${encodeURIComponent(examId)}/answer-sheet-template-binding`);
}

export async function bindExamTemplate(examId: string, templateId: string, expectedRevision: number, mode: "locked_with_guard" = "locked_with_guard") {
  return apiClient.request<{ binding: ExamTemplateBinding }>(`/api/v1/exams/${encodeURIComponent(examId)}/answer-sheet-template-binding`, {
    method: "PUT",
    body: JSON.stringify({ template_id: templateId, mode, expected_revision: expectedRevision })
  });
}

export async function unbindExamTemplate(examId: string, expectedRevision: number, reason: string) {
  return apiClient.request<{ binding: ExamTemplateBinding }>(`/api/v1/exams/${encodeURIComponent(examId)}/answer-sheet-template-binding`, {
    method: "DELETE",
    body: JSON.stringify({ expected_revision: expectedRevision, reason })
  });
}

export async function listOMRCalibrations(templateId: string) {
  return apiClient.request<{ calibrations: OMRCalibrationSession[] }>(`/api/v1/answer-sheet-templates/${encodeURIComponent(templateId)}/omr-calibrations`);
}

export async function createOMRCalibration(templateId: string) {
  return apiClient.request<{ calibration: OMRCalibrationDetail }>(`/api/v1/answer-sheet-templates/${encodeURIComponent(templateId)}/omr-calibrations`, {
    method: "POST",
    body: JSON.stringify({})
  });
}

export async function getOMRCalibration(calibrationId: string) {
  return apiClient.request<{ calibration: OMRCalibrationDetail }>(`/api/v1/omr-calibrations/${encodeURIComponent(calibrationId)}`);
}

export async function labelOMRCalibrationCase(calibrationId: string, caseId: string, expectedOptions: string[]) {
  return apiClient.request<{ calibration: OMRCalibrationDetail }>(`/api/v1/omr-calibrations/${encodeURIComponent(calibrationId)}/cases/${encodeURIComponent(caseId)}/label`, {
    method: "POST",
    body: JSON.stringify({ expected_options: expectedOptions })
  });
}

export async function approveOMRCalibration(calibrationId: string, approvalNote: string) {
  return apiClient.request<{ calibration: OMRCalibrationDetail }>(`/api/v1/omr-calibrations/${encodeURIComponent(calibrationId)}/approve`, {
    method: "POST",
    body: JSON.stringify({ approval_note: approvalNote })
  });
}

export async function revokeOMRCalibration(calibrationId: string, reason: string) {
  return apiClient.request<{ calibration: OMRCalibrationDetail }>(`/api/v1/omr-calibrations/${encodeURIComponent(calibrationId)}/revoke`, {
    method: "POST",
    body: JSON.stringify({ reason })
  });
}

export async function discardOMRCalibration(calibrationId: string, reason: string) {
  return apiClient.request<{ calibration: OMRCalibrationDetail }>(`/api/v1/omr-calibrations/${encodeURIComponent(calibrationId)}/discard`, {
    method: "POST",
    body: JSON.stringify({ reason })
  });
}

export async function downloadOMRCalibrationCaseImage(answerSegmentId: string) {
  return apiClient.requestBlob(`/api/v1/answer-segments/${encodeURIComponent(answerSegmentId)}/image`);
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
