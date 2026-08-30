import { describe, expect, it } from "vitest";
import { createInitialDraft, fallbackSnapshot } from "./gradingWorkbench.model";
import type { WorkbenchContext } from "./gradingWorkbench.types";

function contextWith(points: Array<{ id: string; description: string; score: number }>, ocrText: string) {
  return {
    ocrText,
    ocrResults: [],
    question: { rubric: { points } }
  } as unknown as WorkbenchContext;
}

describe("grading workbench draft model", () => {
  it("creates a fresh draft for each task without leaking rubric selections", () => {
    const first = createInitialDraft(contextWith([
      { id: "step-1", description: "步骤一", score: 2 },
      { id: "step-2", description: "步骤二", score: 3 }
    ], "第一题答案"));

    first.score = 5;
    first.comments = "已评分";
    first.rubricSelections["step-1"] = 2;

    const second = createInitialDraft(contextWith([
      { id: "conclusion", description: "结论", score: 4 }
    ], "第二题答案"));

    expect(second).toEqual({
      score: null,
      comments: "",
      privateNote: "",
      studentFeedback: "",
      reason: "教师复核完成",
      disputeReason: "",
      rubricSelections: { conclusion: 0 },
      answerText: "第二题答案"
    });
    expect(second.rubricSelections).not.toBe(first.rubricSelections);
    expect(second.rubricSelections).not.toHaveProperty("step-1");
  });

  it("restores only valid fallback values and keeps task defaults for invalid data", () => {
    const initial = createInitialDraft(contextWith([
      { id: "criterion", description: "采分点", score: 2 }
    ], "识别文本"));

    const restored = fallbackSnapshot({
      draft: {
        score: Number.NaN,
        comments: "离线草稿",
        rubricSelections: { criterion: 2, invalid: "two" }
      },
      viewer: {
        mode: "unsupported",
        scale: 99,
        rotation: Number.NaN,
        offset: { x: 12, y: "bad" },
        fit: false
      }
    }, initial);

    expect(restored).toMatchObject({
      draft: {
        score: null,
        comments: "离线草稿",
        answerText: "识别文本",
        rubricSelections: { criterion: 2 }
      },
      viewer: {
        mode: "segment",
        scale: 4,
        rotation: 0,
        offset: { x: 12, y: 0 },
        fit: false
      }
    });
  });
});
