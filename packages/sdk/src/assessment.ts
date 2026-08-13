import { EduGradeApi } from "./generated/client";
import type {
  EducationStage,
  ExamQuestionAssessmentSnapshotResponse,
  PutQuestionAssessmentProfileRequest,
  QuestionArchetypeListResponse,
  QuestionAssessmentProfileResponse,
  SubjectCode,
  SubjectProfileListResponse
} from "./generated/types";
import type { ApiTransport } from "./runtime";

export function createAssessmentApi(transport: ApiTransport) {
  const api = new EduGradeApi(transport);
  return {
    listSubjectProfiles(stage?: EducationStage, subject?: SubjectCode): Promise<SubjectProfileListResponse> {
      return api.listAssessmentSubjectProfiles({ query: { stage, subject } });
    },
    listQuestionArchetypes(): Promise<QuestionArchetypeListResponse> {
      return api.listAssessmentQuestionArchetypes();
    },
    getQuestionAssessmentProfile(examId: string, questionId: string): Promise<QuestionAssessmentProfileResponse> {
      return api.getExamQuestionAssessmentProfile({ path: { examId, questionId } });
    },
    updateQuestionAssessmentProfile(
      examId: string,
      questionId: string,
      payload: PutQuestionAssessmentProfileRequest
    ): Promise<QuestionAssessmentProfileResponse> {
      return api.putExamQuestionAssessmentProfile({ path: { examId, questionId }, body: payload });
    },
    getQuestionAssessmentSnapshot(examId: string, questionId: string): Promise<ExamQuestionAssessmentSnapshotResponse> {
      return api.getExamQuestionAssessmentSnapshot({ path: { examId, questionId } });
    }
  };
}
