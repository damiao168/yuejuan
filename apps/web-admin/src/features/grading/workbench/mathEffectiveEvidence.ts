import type { MathUnderstandingResponse } from "../../../api/mathUnderstanding";

// The server owns correction projection. Legacy responses remain readable,
// but clients never derive effective evidence from a bounded audit history.
export function selectEffectiveMathArtifact(response: MathUnderstandingResponse) {
  return response.effective_artifact ?? response.artifact;
}
