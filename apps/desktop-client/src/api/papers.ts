import type { Question } from "../types";
import type { DesktopApiClient } from "./client";

export async function listQuestions(client: DesktopApiClient, examId: string) {
  return client.request<{ questions: Question[] }>(`/api/v1/exams/${encodeURIComponent(examId)}/questions`);
}
