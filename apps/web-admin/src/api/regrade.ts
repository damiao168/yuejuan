import { EduGradeApi } from "@edugrade/sdk";
import type {
  RecordRegradeCandidateRequest,
  RegradeGraderContext,
  RegradeWorkItem
} from "@edugrade/sdk";
import { apiClient } from "./client";

const generatedApi = new EduGradeApi(apiClient);

export type { RecordRegradeCandidateRequest, RegradeGraderContext, RegradeWorkItem };

export function listMyRegradeItems() {
  return generatedApi.listMyRegradeItems();
}

export function claimRegradeItem(itemId: string) {
  return generatedApi.claimRegradeItem({ path: { itemId } });
}

export function getRegradeItemContext(itemId: string) {
  return generatedApi.getRegradeItemContext({ path: { itemId } });
}

export function recordRegradeCandidate(itemId: string, payload: RecordRegradeCandidateRequest) {
  return generatedApi.recordRegradeCandidate({ path: { itemId }, body: payload });
}

export function downloadRegradeSegmentImage(itemId: string) {
  return apiClient.requestBlob(`/api/v1/regrade-items/${encodeURIComponent(itemId)}/segment-image`);
}
