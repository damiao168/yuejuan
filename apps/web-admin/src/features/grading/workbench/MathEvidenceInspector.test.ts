import { describe, expect, it } from "vitest";
import type { MathUnderstandingResponse } from "../../../api/mathUnderstanding";
import { isFormulaEvidenceSubject } from "./MathEvidenceInspector";
import { selectEffectiveMathArtifact } from "./mathEffectiveEvidence";

describe("math evidence subject boundary", () => {
  it("enables formula evidence only for calibrated quantitative subjects", () => {
    expect(isFormulaEvidenceSubject("mathematics")).toBe(true);
    expect(isFormulaEvidenceSubject("physics")).toBe(true);
    expect(isFormulaEvidenceSubject("chemistry")).toBe(true);
  });

  it.each(["chinese", "history", "ethics_politics", "geography", "english", "biology"])(
    "does not call formula evidence for %s",
    (subject) => expect(isFormulaEvidenceSubject(subject)).toBe(false)
  );
});

describe("math evidence correction projection", () => {
  it("uses effective evidence while retaining the immutable artifact response", () => {
    const response = {
      artifact: { formulas: [{ id: "formula-1", canonical_latex: "x=2" }] },
      effective_artifact: { formulas: [{ id: "formula-1", canonical_latex: "x=3" }] },
      correction_revision: 1,
      corrected: true,
      corrections: []
    } as unknown as MathUnderstandingResponse;

    expect(selectEffectiveMathArtifact(response).formulas).toEqual([
      { id: "formula-1", canonical_latex: "x=3" }
    ]);
    expect(response.artifact.formulas).toEqual([{ id: "formula-1", canonical_latex: "x=2" }]);
  });

  it("keeps older server responses usable without interpreting correction history", () => {
    const response = {
      artifact: { formulas: [{ id: "formula-1", canonical_latex: "x=2" }] },
      corrections: [{ revision: 1, corrected_contract: { formulas: [{ canonical_latex: "x=9" }] } }]
    } as unknown as MathUnderstandingResponse;

    expect(selectEffectiveMathArtifact(response)).toBe(response.artifact);
  });
});
