import type { Exam } from "../types";
import type { DesktopApiClient } from "./client";

export interface ExamListFilter {
  status?: string;
  school_id?: string;
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
