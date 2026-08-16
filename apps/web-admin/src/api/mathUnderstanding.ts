import { EduGradeApi } from "@edugrade/sdk";
import type {
  CreateMathCorrectionRequest,
  MathCorrectionResponse,
  MathUnderstandingResponse
} from "@edugrade/sdk";
import { apiClient } from "./client";

const mathApi = new EduGradeApi(apiClient);

export type { CreateMathCorrectionRequest, MathCorrectionResponse, MathUnderstandingResponse };

export function getMathUnderstanding(segmentId: string, signal?: AbortSignal) {
  return mathApi.getMathUnderstanding({ path: { segmentId }, signal });
}

export function createMathUnderstandingCorrection(
  artifactId: string,
  body: CreateMathCorrectionRequest
) {
  return mathApi.createMathUnderstandingCorrection({ path: { artifactId }, body });
}
