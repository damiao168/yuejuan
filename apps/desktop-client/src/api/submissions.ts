import type { AnswerSegment, OcrTask, SubmissionPage, SubmissionQualityResult } from "../types";
import type { DesktopApiClient } from "./client";

export interface AddSubmissionPagePayload {
  file_asset_id: string;
  page_no: number;
}

export async function addSubmissionPage(client: DesktopApiClient, submissionId: string, payload: AddSubmissionPagePayload) {
  return client.request<{ page: SubmissionPage }>(`/api/v1/submissions/${encodeURIComponent(submissionId)}/pages`, {
    method: "POST",
    body: JSON.stringify(payload)
  });
}

export async function runSubmissionQualityCheck(client: DesktopApiClient, submissionId: string) {
  return client.request<{ result: SubmissionQualityResult }>(`/api/v1/submissions/${encodeURIComponent(submissionId)}/quality-check`, {
    method: "POST"
  });
}

export async function listSubmissionPages(client: DesktopApiClient, submissionId: string) {
  return client.request<{ pages: SubmissionPage[] }>(`/api/v1/submissions/${encodeURIComponent(submissionId)}/pages`);
}

export async function listAnswerSegments(client: DesktopApiClient, submissionId: string) {
  return client.request<{ segments: AnswerSegment[] }>(`/api/v1/submissions/${encodeURIComponent(submissionId)}/answer-segments`);
}

export async function listOcrTasks(client: DesktopApiClient, submissionId: string) {
  return client.request<{ tasks: OcrTask[] }>(`/api/v1/submissions/${encodeURIComponent(submissionId)}/ocr-tasks`);
}

export async function getOcrTask(client: DesktopApiClient, taskId: string) {
  return client.request<{ task: OcrTask }>(`/api/v1/ocr-tasks/${encodeURIComponent(taskId)}`);
}
