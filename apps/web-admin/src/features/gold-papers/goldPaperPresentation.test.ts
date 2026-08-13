import { describe, expect, it } from "vitest";
import { buildRubricEvidence, goldCoverageGapLabel, rubricPointsFromSnapshot } from "./goldPaperPresentation";

describe("gold paper presentation", () => {
  it("keeps rubric-point evidence instead of reducing a Gold version to total score", () => {
    const points = rubricPointsFromSnapshot({ points: [{ id: "step", description: "关键步骤", score: 2 }] });
    expect(buildRubricEvidence({ step: 1.5, ignored: 9 }, points)).toEqual({ step: 1.5 });
  });

  it("labels a subject coverage pattern without hiding its code", () => {
    expect(goldCoverageGapLabel("missing_pattern:unit_error")).toBe("缺少典型模式：unit_error");
  });
});
