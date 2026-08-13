import { EduGradeApi } from "@edugrade/sdk";
import type {
  ExamWorkspaceCounts,
  ExamWorkspaceNextAction,
  ExamWorkspaceNotice,
  ExamWorkspaceProjection,
  ExamWorkspaceStage,
  ExamWorkspaceStageProgress
} from "@edugrade/sdk";
import { apiClient } from "./client";

export type {
  ExamWorkspaceCounts,
  ExamWorkspaceNextAction,
  ExamWorkspaceNotice,
  ExamWorkspaceProjection,
  ExamWorkspaceStage,
  ExamWorkspaceStageProgress
};

const workspaceApi = new EduGradeApi(apiClient);

export function getExamWorkspace(examId: string) {
  return workspaceApi.getExamWorkspace({ path: { examId } });
}
