import { apiClient } from "./client";
export { uploadFile } from "./files";
export type { FileAsset } from "./files";

export interface PaperFile {
  original_name: string;
  content_type: string;
  size_bytes: number;
  hash_sha256: string;
  storage_bucket?: string;
  storage_key?: string;
}

export interface PaperVersion {
  id: string;
  tenant_id: string;
  exam_id: string;
  file_asset_id: string;
  version_no: number;
  status: string;
  file: PaperFile;
}

export interface PaperImportDraftQuestion {
  question_no: string;
  question_type: string;
  score: number;
  stem: string;
  knowledge_points: string[];
  confidence: number;
  issues: string[];
  matched_question_id?: string;
  match_status?: "create" | "matched" | "matched_by_order" | "mismatch" | "extra" | "ambiguous";
}

export interface PaperImportJob {
  id: string;
  exam_id: string;
  exam_paper_id: string;
  paper_file_asset_id: string;
  answer_file_asset_id: string;
  status: "processing" | "review_required" | "failed" | "applied";
  subject: string;
  questions: PaperImportDraftQuestion[];
  issues: string[];
  error_code?: string;
  created_at: string;
  applied_at?: string;
}

export interface AnswerKeyInput {
  standard_answer: unknown;
  equivalent_answers: unknown[];
  tolerance: unknown;
}

export interface AnswerKey extends AnswerKeyInput {
  id: string;
  question_id: string;
  answer_version: string;
}

export interface RubricPoint {
  id: string;
  description: string;
  score: number;
  required: boolean;
  evidence_requirements?: RubricEvidenceRequirement[];
}

export interface RubricEvidenceRequirement {
  type: "all_of" | "any_of" | "at_least" | "valid_transformation" | "final_result" | "concept" | "unit" | "domain";
  target?: string;
  minimum?: number;
  children?: RubricEvidenceRequirement[];
}

export interface ScoringRule {
  id: string;
  question_id: string;
  version: number;
  rule_type: string;
  config: Record<string, unknown>;
  status: "draft" | "published" | "retired";
  revision: number;
  content_hash: string;
  published_at?: string;
}

export interface Rubric {
  id: string;
  question_id: string;
  version: string;
  status: string;
  max_score: number;
  points: RubricPoint[];
  deductions: unknown[];
  examples: unknown[];
}

export interface Question {
  id: string;
  tenant_id: string;
  exam_id: string;
  exam_paper_id?: string;
  question_no: string;
  question_type: string;
  score: number;
  stem?: string;
  knowledge_points: string[];
  answer_area?: Record<string, unknown>;
  sort_order: number;
  status: string;
  answer_key?: AnswerKey;
  rubric?: Rubric;
}

export interface QuestionPayload {
  exam_paper_id?: string;
  question_no: string;
  question_type: string;
  score: number;
  stem: string;
  knowledge_points: string[];
  answer_area: Record<string, unknown>;
  sort_order: number;
  answer_key: AnswerKeyInput;
}

export interface RubricPayload {
  status: string;
  max_score: number;
  points: RubricPoint[];
  deductions: unknown[];
  examples: unknown[];
}

export interface ValidationIssue {
  code: string;
  message: string;
}

export interface ValidationResult {
  valid: boolean;
  issues: ValidationIssue[];
}

export async function registerPaperFromFile(examId: string, fileAssetId: string) {
  return apiClient.request<{ paper: PaperVersion; note: string }>(`/api/v1/exams/${encodeURIComponent(examId)}/papers`, {
    method: "POST",
    body: JSON.stringify({ file_asset_id: fileAssetId })
  });
}

export async function listPapers(examId: string) {
  return apiClient.request<{ papers: PaperVersion[] }>(`/api/v1/exams/${encodeURIComponent(examId)}/papers`);
}

export async function listPaperImports(examId: string) {
  return apiClient.request<{ imports: PaperImportJob[] }>(`/api/v1/exams/${encodeURIComponent(examId)}/paper-imports`);
}

export async function createPaperImport(examId: string, payload: { exam_paper_id: string; paper_file_asset_id: string; answer_file_asset_id: string; subject: string }) {
  return apiClient.request<{ import: PaperImportJob }>(`/api/v1/exams/${encodeURIComponent(examId)}/paper-imports`, { method: "POST", body: JSON.stringify(payload) });
}

export async function applyPaperImport(importId: string) {
  return apiClient.request<{ import: PaperImportJob }>(`/api/v1/paper-imports/${encodeURIComponent(importId)}/apply`, { method: "POST" });
}

export async function listQuestions(examId: string) {
  return apiClient.request<{ questions: Question[] }>(`/api/v1/exams/${encodeURIComponent(examId)}/questions`);
}

export async function createQuestion(examId: string, payload: QuestionPayload) {
  return apiClient.request<{ question: Question }>(`/api/v1/exams/${encodeURIComponent(examId)}/questions`, {
    method: "POST",
    body: JSON.stringify(payload)
  });
}

export async function updateQuestion(questionId: string, payload: Partial<QuestionPayload>) {
  return apiClient.request<{ question: Question }>(`/api/v1/questions/${encodeURIComponent(questionId)}`, {
    method: "PATCH",
    body: JSON.stringify(payload)
  });
}

export async function deleteQuestion(questionId: string) {
  return apiClient.request<{ status: string }>(`/api/v1/questions/${encodeURIComponent(questionId)}`, {
    method: "DELETE"
  });
}

export async function createRubric(questionId: string, payload: RubricPayload) {
  return apiClient.request<{ rubric: Rubric }>(`/api/v1/questions/${encodeURIComponent(questionId)}/rubric`, {
    method: "POST",
    body: JSON.stringify(payload)
  });
}

export async function validatePaperConfig(examId: string) {
  return apiClient.request<{ result: ValidationResult }>(`/api/v1/exams/${encodeURIComponent(examId)}/validate-paper-config`, {
    method: "POST"
  });
}

export async function listScoringRules(questionId: string) {
  return apiClient.request<{ scoring_rules: ScoringRule[] }>(`/api/v1/questions/${encodeURIComponent(questionId)}/scoring-rules`);
}

export async function createScoringRule(questionId: string, ruleType: string, config: Record<string, unknown>) {
  return apiClient.request<{ scoring_rule: ScoringRule }>(`/api/v1/questions/${encodeURIComponent(questionId)}/scoring-rules`, {
    method: "POST",
    body: JSON.stringify({ rule_type: ruleType, config })
  });
}

export async function updateScoringRule(ruleId: string, config: Record<string, unknown>, expectedRevision: number) {
  return apiClient.request<{ scoring_rule: ScoringRule }>(`/api/v1/scoring-rules/${encodeURIComponent(ruleId)}`, {
    method: "PATCH",
    body: JSON.stringify({ config, expected_revision: expectedRevision })
  });
}

export async function publishScoringRule(ruleId: string) {
  return apiClient.request<{ scoring_rule: ScoringRule }>(`/api/v1/scoring-rules/${encodeURIComponent(ruleId)}/publish`, { method: "POST" });
}
