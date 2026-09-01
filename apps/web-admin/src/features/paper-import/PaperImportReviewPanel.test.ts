import { describe, expect, it } from "vitest";
import { displayPaperImportValue, rubricFromCandidate } from "./PaperImportReviewPanel";

describe("paper import review value display", () => {
  it("preserves scalar answers and serializes structured answers safely", () => {
    expect(displayPaperImportValue("A")).toBe("A");
    expect(displayPaperImportValue(42)).toBe("42");
    expect(displayPaperImportValue(false)).toBe("false");
    expect(displayPaperImportValue(["A", "C"])).toBe('[\n  "A",\n  "C"\n]');
    expect(displayPaperImportValue({ value: 3 })).toBe('{\n  "value": 3\n}');
  });

  it("keeps candidate identity outside the rubric and preserves all extracted rubric data", () => {
    const rubric = rubricFromCandidate({
      candidate_id: "rubric-1",
      points: [{ id: "p1", description: "步骤正确", score: null }, { id: "p2", description: "结论正确", score: 2, required: false }],
      deductions: [{ description: "单位错误", score: -1 }],
      examples: ["示例"],
      confidence: 0.8,
      source_refs: [],
      issues: []
    }, 6);
    expect(rubric).toEqual({
      status: "draft",
      max_score: 6,
      points: [
        { id: "p1", description: "步骤正确", score: 0, required: true, evidence_requirements: undefined },
        { id: "p2", description: "结论正确", score: 2, required: false, evidence_requirements: undefined }
      ],
      deductions: [{ description: "单位错误", score: -1 }],
      examples: ["示例"]
    });
  });
});
