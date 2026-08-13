import {
  EduGradeApi,
  type ProcessingExceptionSeverity,
  type ProcessingExceptionStatus,
  type ProcessingStage
} from "@edugrade/sdk";
import { apiClient } from "./client";

export type {
  ProcessingException,
  ProcessingExceptionSeverity,
  ProcessingExceptionStatus,
  ProcessingStage,
  ProcessingSummary
} from "@edugrade/sdk";

export interface ProcessingExceptionFilter {
  examId: string;
  severity?: ProcessingExceptionSeverity;
  stage?: ProcessingStage;
  status?: ProcessingExceptionStatus;
  subject?: string;
  limit?: number;
  cursor?: string;
}

const processingApi = new EduGradeApi(apiClient);

export function getProcessingSummary(examId: string, signal?: AbortSignal) {
  return processingApi.getExamProcessingSummary({ path: { examId }, signal });
}

export function listProcessingExceptions(filter: ProcessingExceptionFilter, signal?: AbortSignal) {
  return processingApi.listProcessingExceptions({
    query: {
      exam_id: filter.examId,
      severity: filter.severity,
      stage: filter.stage,
      status: filter.status,
      subject: filter.subject,
      limit: filter.limit,
      cursor: filter.cursor
    },
    signal
  });
}

export function retryProcessingException(exceptionId: string) {
  return processingApi.retryProcessingException({ path: { id: exceptionId } });
}

export function assignProcessingException(exceptionId: string, assigneeId: string) {
  return processingApi.assignProcessingException({
    path: { id: exceptionId },
    body: { assignee_id: assigneeId }
  });
}

export function resolveProcessingException(exceptionId: string, resolution: string) {
  return processingApi.resolveProcessingException({
    path: { id: exceptionId },
    body: { resolution }
  });
}
