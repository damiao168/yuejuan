import { apiClient } from "./client";

export interface ReviewTask {
  id: string;
  tenant_id: string;
  exam_id: string;
  question_id: string;
  question_no: string;
  answer_segment_id: string;
  submission_id: string;
  anonymous_code: string;
  source: string;
  status: string;
  priority: number;
  assigned_to?: string;
  return_reason?: string;
  grade_round: string;
  due_at?: string;
  created_by: string;
  created_at: string;
  updated_at: string;
}

export interface RubricSelection {
  point_id: string;
  score: number;
}

export interface SubmitHumanGradePayload {
  score: number;
  rubric_selections: RubricSelection[];
  comments: string;
  private_note: string;
  student_feedback: string;
  reason: string;
}

export interface HumanGrade {
  id: string;
  tenant_id: string;
  review_task_id: string;
  answer_segment_id: string;
  reviewer_id: string;
  score: number;
  max_score: number;
  rubric_selections: RubricSelection[];
  comments?: string;
  private_note?: string;
  student_feedback?: string;
  reason?: string;
  grade_round: string;
  created_at: string;
}

export interface FinalGrade {
  id: string;
  tenant_id?: string;
  exam_id?: string;
  question_id?: string;
  question_no?: string;
  answer_segment_id?: string;
  submission_id?: string;
  anonymous_code?: string;
  score: number;
  max_score: number;
  source: string;
  double_mark_session_id?: string;
  arbitration_task_id?: string;
  resolution_strategy?: string;
  locked: boolean;
  created_by?: string;
  created_at?: string;
  updated_at?: string;
}

export interface SubmitHumanGradeResult {
  task: ReviewTask;
  human_grade: HumanGrade;
  final_grade?: FinalGrade;
  double_mark_session?: unknown;
  arbitration_task?: unknown;
}

export interface SegmentAnswerPayload {
  answer_text: string;
  answer_payload: Record<string, unknown>;
  source: string;
  confidence?: number;
}

export interface SegmentAnswer {
  id: string;
  tenant_id: string;
  answer_segment_id: string;
  answer_text: string;
  answer_payload: Record<string, unknown>;
  source: string;
  confidence?: number;
  recorded_by: string;
  created_at: string;
}

export interface PointResult {
  code: string;
  label: string;
  score: number;
}

export interface GradeEvidence {
  type: string;
  answer_segment_id?: string;
  answer_text?: string;
  standard_answer?: string;
  rule?: string;
  bbox?: number[];
}

export interface AiGrade {
  id: string;
  tenant_id: string;
  answer_segment_id: string;
  question_id: string;
  question_no: string;
  question_type: string;
  answer_version?: string;
  grader_type: string;
  rule_version?: string;
  model_version?: string;
  prompt_version?: string;
  suggested_score: number;
  max_score: number;
  confidence: number;
  matched_points: PointResult[];
  missing_points: PointResult[];
  evidence: GradeEvidence[];
  risk_flags: string[];
  needs_human_review: boolean;
  auto_pass?: boolean;
  mock: boolean;
  status?: string;
  failure_reason?: string;
  student_feedback?: string;
  teacher_note?: string;
  raw_output?: Record<string, unknown>;
  created_by: string;
  created_at: string;
}

export interface EvidenceIssue {
  code: string;
  message: string;
}

export interface EvidenceResult {
  passed: boolean;
  failed: EvidenceIssue[];
  warnings: EvidenceIssue[];
  corrected_flags: string[];
  needs_human_review: boolean;
}

export interface EvidenceJob {
  id: string;
  tenant_id: string;
  job_type: string;
  target_type: string;
  target_id: string;
  status: string;
  result: EvidenceResult;
  needs_human_review: boolean;
  created_by: string;
  created_at: string;
}

export interface ReviewTaskFilter {
  status?: string;
  assigned_to?: string;
}

export interface ArbitrationContext {
  raw_answer?: string;
  ocr_text?: string;
  ai_suggestion?: Record<string, unknown>;
}

export interface ArbitrationTask {
  id: string;
  tenant_id: string;
  double_mark_session_id: string;
  exam_id: string;
  question_id: string;
  question_no: string;
  answer_segment_id: string;
  submission_id: string;
  anonymous_code: string;
  first_reviewer_id: string;
  second_reviewer_id: string;
  first_score: number;
  second_score: number;
  score_difference: number;
  difference_reason: string;
  status: string;
  assigned_to?: string;
  final_score?: number;
  reason?: string;
  student_feedback?: string;
  allow_same_arbitrator: boolean;
  context: ArbitrationContext;
  created_by: string;
  created_at: string;
  updated_at: string;
}

export interface ArbitrationTaskFilter {
  status?: string;
  assigned_to?: string;
}

export interface AssignArbitrationPayload {
  assigned_to: string;
}

export interface SubmitArbitrationPayload {
  final_score: number;
  reason: string;
  student_feedback: string;
}

export interface SubmitArbitrationResult {
  arbitration_task: ArbitrationTask;
  final_grade: FinalGrade;
}

function queryString(filter: { status?: string; assigned_to?: string }) {
  const params = new URLSearchParams();
  if (filter.status) {
    params.set("status", filter.status);
  }
  if (filter.assigned_to) {
    params.set("assigned_to", filter.assigned_to);
  }
  const query = params.toString();
  return query ? `?${query}` : "";
}

export async function listReviewTasks(filter: ReviewTaskFilter = {}) {
  return apiClient.request<{ tasks: ReviewTask[] }>(`/api/v1/review-tasks${queryString(filter)}`);
}

export async function getReviewTask(id: string) {
  return apiClient.request<{ task: ReviewTask }>(`/api/v1/review-tasks/${encodeURIComponent(id)}`);
}

export async function submitHumanGrade(taskId: string, payload: SubmitHumanGradePayload) {
  return apiClient.request<SubmitHumanGradeResult>(`/api/v1/review-tasks/${encodeURIComponent(taskId)}/submit`, {
    method: "POST",
    body: JSON.stringify(payload)
  });
}

export async function returnReviewTask(taskId: string, reason: string) {
  return apiClient.request<{ task: ReviewTask }>(`/api/v1/review-tasks/${encodeURIComponent(taskId)}/return`, {
    method: "POST",
    body: JSON.stringify({ reason })
  });
}

export async function listArbitrationTasks(filter: ArbitrationTaskFilter = {}) {
  return apiClient.request<{ arbitration_tasks: ArbitrationTask[] }>(`/api/v1/arbitration-tasks${queryString(filter)}`);
}

export async function getArbitrationTask(id: string) {
  return apiClient.request<{ arbitration_task: ArbitrationTask }>(`/api/v1/arbitration-tasks/${encodeURIComponent(id)}`);
}

export async function assignArbitrationTask(id: string, payload: AssignArbitrationPayload) {
  return apiClient.request<{ arbitration_task: ArbitrationTask }>(`/api/v1/arbitration-tasks/${encodeURIComponent(id)}/assign`, {
    method: "POST",
    body: JSON.stringify(payload)
  });
}

export async function submitArbitration(id: string, payload: SubmitArbitrationPayload) {
  return apiClient.request<SubmitArbitrationResult>(`/api/v1/arbitration-tasks/${encodeURIComponent(id)}/submit`, {
    method: "POST",
    body: JSON.stringify(payload)
  });
}

export async function recordSegmentAnswer(segmentId: string, payload: SegmentAnswerPayload) {
  return apiClient.request<{ answer: SegmentAnswer }>(`/api/v1/answer-segments/${encodeURIComponent(segmentId)}/answer`, {
    method: "PUT",
    body: JSON.stringify(payload)
  });
}

export async function createRuleGrade(segmentId: string) {
  return apiClient.request<{ grade: AiGrade }>(`/api/v1/answer-segments/${encodeURIComponent(segmentId)}/rule-grade`, {
    method: "POST"
  });
}

export async function createSubjectiveAiGrade(segmentId: string) {
  return apiClient.request<{ grade: AiGrade }>(`/api/v1/answer-segments/${encodeURIComponent(segmentId)}/subjective-ai-grade`, {
    method: "POST",
    body: JSON.stringify({
      model_policy: {
        model_version: "mock-llm-v1",
        prompt_version: "subjective-mock-prompt-v1",
        min_confidence: 0.8
      }
    })
  });
}

export async function listAiGrades(segmentId: string) {
  return apiClient.request<{ grades: AiGrade[] }>(`/api/v1/answer-segments/${encodeURIComponent(segmentId)}/ai-grades`);
}

export async function verifyEvidence(gradeId: string) {
  return apiClient.request<{ job: EvidenceJob }>(`/api/v1/ai-grades/${encodeURIComponent(gradeId)}/verify-evidence`, {
    method: "POST",
    body: JSON.stringify({})
  });
}
