import { EduGradeApi, type ApiTransport, type StudentReviewAnnotation } from "@edugrade/sdk";

export interface AuthUser {
  id: string;
  tenant_id: string;
  tenant_code: string;
  username: string;
  display_name: string;
  roles: string[];
  permissions: string[];
  data_scope: Record<string, unknown>;
}

export interface PublishedExam {
  exam_id: string;
  name: string;
  subject: string;
  release_version: number;
  published_at: string;
}

export interface StudentQuestion {
  question_id: string;
  question_no: string;
  score: number;
  max_score: number;
  feedback?: string;
  rubric_summary?: string[];
}

export interface StudentResult {
  exam_id: string;
  // The release identifier is an immutable public-score version anchor. It is
  // sent back only when a student starts an appeal; the server still derives
  // the original score from that release rather than trusting the browser.
  release_id: string;
  release_version: number;
  total_score: number;
  max_score: number;
  questions?: StudentQuestion[];
  appeal_window: {
    open: boolean;
    closes_at?: string;
    allowed_reason_codes?: string[];
  };
}

export interface StudentQuestionAppeal {
  id: string;
  exam_id: string;
  source_release_id: string;
  source_release_version: number;
  question_id: string;
  question_no: string;
  source_score: number;
  source_max_score: number;
  reason_code: string;
  reason: string;
  status: string;
  decision?: string;
  public_response?: string;
  new_release_id?: string;
  created_at: string;
  updated_at: string;
}

export interface SelectedAppealRegion {
  coordinate_space: "canonical_image_normalized";
  x: number;
  y: number;
  width: number;
  height: number;
}

// Generated from the public OpenAPI contract. It deliberately has no
// reviewer, tenant, revision, visibility, or payload fields.
export type StudentQuestionAnnotation = StudentReviewAnnotation;

export class PortalApiError extends Error {
  constructor(readonly status: number, readonly code: string, message: string) {
    super(message);
    this.name = "PortalApiError";
  }
}

const baseUrl = (import.meta.env.VITE_API_BASE_URL ?? "").trim().replace(/\/+$/, "");

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const response = await fetch(`${baseUrl}${path}`, {
    ...init,
    credentials: "include",
    headers: { Accept: "application/json", ...init.headers }
  });
  if (!response.ok) {
    let payload: { error?: { code?: string; message?: string }; code?: string; message?: string } | undefined;
    try {
      payload = await response.json() as typeof payload;
    } catch {
      // The status is still useful when a proxy rejects a request without JSON.
    }
    throw new PortalApiError(
      response.status,
      payload?.error?.code ?? payload?.code ?? "request_failed",
      payload?.error?.message ?? payload?.message ?? response.statusText
    );
  }
  return response.json() as Promise<T>;
}

// Keep the portal's existing session/cookie and PortalApiError behavior while
// routing new contract-backed calls through the shared generated SDK.
const sdkTransport: ApiTransport = { request };
const sdk = new EduGradeApi(sdkTransport);

export function currentUser() {
  return request<{ user: AuthUser }>("/api/v1/auth/me");
}

export function login(input: { tenant_code: string; username: string; password: string }) {
  return request<{ user: AuthUser }>("/api/v1/auth/login", {
    method: "POST",
    headers: { "Content-Type": "application/json", "X-EduGrade-CSRF": "1" },
    body: JSON.stringify(input)
  });
}

export function logout() {
  return request<void>("/api/v1/auth/logout", { method: "POST", headers: { "X-EduGrade-CSRF": "1" } });
}

export function listPublishedExams() {
  return request<{ exams: PublishedExam[] }>("/api/v1/student/exams");
}

export function getResult(examID: string) {
  return request<{ result: StudentResult }>(`/api/v1/student/exams/${encodeURIComponent(examID)}/result`);
}

export function getQuestion(examID: string, questionID: string) {
  return request<{ question: StudentQuestion }>(`/api/v1/student/exams/${encodeURIComponent(examID)}/questions/${encodeURIComponent(questionID)}`);
}

// This is a student-scoped, per-question crop URL. The API authorizes it
// against the student's current published score release; no answer-segment ID
// or storage URL is exposed to the portal.
export function studentQuestionAnswerImageURL(examID: string, questionID: string) {
  return `${baseUrl}/api/v1/student/exams/${encodeURIComponent(examID)}/questions/${encodeURIComponent(questionID)}/answer-image`;
}

export function listQuestionAnnotations(examID: string, questionID: string) {
  return sdk.listStudentQuestionReviewAnnotations({ path: { examId: examID, questionId: questionID } });
}

export function createQuestionAppeal(examID: string, input: {
  source_release_id: string;
  question_id: string;
  reason_code: string;
  reason: string;
  selected_region?: SelectedAppealRegion;
}) {
  return request<{ appeal: StudentQuestionAppeal }>(`/api/v1/student/exams/${encodeURIComponent(examID)}/question-appeals`, {
    method: "POST",
    headers: { "Content-Type": "application/json", "X-EduGrade-CSRF": "1" },
    body: JSON.stringify(input)
  });
}

export function listQuestionAppeals(examID: string) {
  return request<{ appeals: StudentQuestionAppeal[] }>(`/api/v1/student/question-appeals?exam_id=${encodeURIComponent(examID)}`);
}
