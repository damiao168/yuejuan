import { describe, expect, it } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";
import { createElement } from "react";
import type { MathRubricScoreResponse, MathUnderstandingResponse } from "../../../api/mathUnderstanding";
import type { AiGrade } from "../../../api/review";
import { MathRubricMatrix } from "./components/MathRubricMatrix";
import { mathDecisionSteps, mathScoreMatchesUnderstanding, mathSuggestionState, selectMathStep, type MathWorkbenchEvidence } from "./mathWorkbenchEvidence";

function fixture() {
  const understanding = { artifact: { id: "a1", answer_segment_id: "s1", exam_question_snapshot_id: "snapshot1", version: 3, stage: "verified", correction_revision: 2,
    blocks: [{ id: "b1", status: "active", bbox: { x: .1, y: .2, width: .4, height: .1 } }],
    formulas: [], solution_graph: { steps: [{ id: "step1", block_ids: ["b1"], formula_ids: ["f1"], normalized_text: "x = 2" }], edges: [] },
    verifications: [{ id: "v1", step_id: "step1", formula_id: "f1", status: "verified", engine: "sympy" }] }, correction_revision: 0 } as unknown as MathUnderstandingResponse;
  const score = { schema_version: "math-rubric-score-v1", scope: "teacher_suggestion_only", artifact_id: "a1", artifact_version: 3, correction_revision: 0, verified_correction_revision: 2,
    exam_question_snapshot_id: "snapshot1", rubric_version: "r1", max_score: 2, verified_score: 2, unresolved_score: 0, suggested_score: 2, score_range: { min: 2, max: 2 },
    criterion_decisions: [{ rubric_point_id: "p1", status: "supported", max_score: 2, awarded_score: 2, evidence_ids: ["e1"], verification_ids: ["v1"], decision_source: "symbolic", reason_code: "equivalent" }],
    rubric_evidence: [{ id: "e1", source_artifact_ids: ["f1", "b1"] }] } as unknown as MathRubricScoreResponse;
  const grade = { id: "g1", answer_segment_id: "s1", math_artifact_id: "a1", math_artifact_version: 3, math_correction_revision: 2, math_scoring_version: "math-rubric-score-v1", rubric_version: "r1",
    delivery_mode: "teacher_suggestion", needs_human_review: true, status: "succeeded", mock: false, suggested_score: 2, max_score: 2, created_at: "2026-09-14T00:00:00Z" } as AiGrade;
  const evidence: MathWorkbenchEvidence = { segmentId: "s1", phase: "ready", understanding, score, message: "" };
  return { understanding, score, grade, evidence };
}

describe("mathematical suggestion freshness", () => {
  it("accepts only the exact verified artifact, absorbed-plus-local correction and server scorer", () => {
    const { grade, evidence } = fixture();
    expect(mathSuggestionState(grade, evidence).current).toBe(true);
    expect(mathSuggestionState(grade, evidence, true).current).toBe(false);
    evidence.understanding!.correction_revision = 1;
    expect(mathSuggestionState(grade, evidence).label).toContain("已过期");
  });
  it.each(["loading", "pending", "unavailable", "conflict"] as const)("fails closed while %s", (phase) => {
    const { grade, evidence } = fixture();
    evidence.phase = phase;
    expect(mathSuggestionState(grade, evidence).current).toBe(false);
  });
  it.each([
    { math_artifact_id: "a2" }, { math_artifact_version: 4 }, { math_correction_revision: 0 }, { math_scoring_version: "future" },
    { rubric_version: "r2" }, { answer_segment_id: "other" }, { mock: true }, { status: "failed" }, { delivery_mode: "shadow_only" },
    { suggested_score: 999 }, { suggested_score: Number.NaN }, { needs_human_review: false }, { math_artifact_id: undefined }
  ])("rejects old, unbound or unsafe material: %j", (change) => {
    const { grade, evidence } = fixture();
    expect(mathSuggestionState({ ...grade, ...change } as AiGrade, evidence).current).toBe(false);
  });
  it("does not turn a nullable total or unresolved point into a current zero", () => {
    const { grade, evidence } = fixture();
    evidence.score!.suggested_score = null;
    evidence.score!.unresolved_score = 2;
    expect(mathSuggestionState({ ...grade, suggested_score: 0 }, evidence).current).toBe(false);
  });
  it("rejects raced score/projection and verified lineage reads", () => {
    const { understanding, score } = fixture();
    expect(mathScoreMatchesUnderstanding(score, understanding)).toBe(true);
    expect(mathScoreMatchesUnderstanding({ ...score, artifact_version: 4 }, understanding)).toBe(false);
    expect(mathScoreMatchesUnderstanding({ ...score, verified_correction_revision: 0 }, understanding)).toBe(false);
    expect(mathScoreMatchesUnderstanding({ ...score, exam_question_snapshot_id: "other" }, understanding)).toBe(false);
  });
});

describe("rubric × step provenance", () => {
  it("resolves bound formulas/blocks/verification to a single step and crop-relative box", () => {
    const { understanding, score } = fixture();
    expect(mathDecisionSteps(score.criterion_decisions[0], score, understanding).map((step) => step.id)).toEqual(["step1"]);
    expect(selectMathStep(understanding, "step1")?.bbox).toEqual({ x: .1, y: .2, width: .4, height: .10000000000000003 });
    expect(selectMathStep(understanding, "unknown")).toBeNull();
  });
  it("uses effective corrected coordinates, rejects crossed-out/invalid geometry", () => {
    const { understanding } = fixture();
    understanding.effective_artifact = { ...understanding.artifact, solution_graph: { ...understanding.artifact.solution_graph,
      steps: [{ ...understanding.artifact.solution_graph.steps[0], bbox: { x: .3, y: .4, width: .1, height: .2 } }] } } as unknown as NonNullable<MathUnderstandingResponse["effective_artifact"]>;
    expect(selectMathStep(understanding, "step1")?.bbox.x).toBe(.3);
    understanding.effective_artifact.solution_graph.steps[0].bbox!.x = 5;
    understanding.effective_artifact.blocks[0].status = "crossed_out";
    expect(selectMathStep(understanding, "step1")).toBeNull();
  });
  it("renders uncertain rows as pending, with a range and preserved stale history", () => {
    const { evidence, grade } = fixture();
    evidence.score!.suggested_score = null; evidence.score!.unresolved_score = 2; evidence.score!.verified_score = 0;
    evidence.score!.score_range = { min: 0, max: 2 };
    evidence.score!.criterion_decisions[0].awarded_score = null; evidence.score!.criterion_decisions[0].status = "uncertain";
    const html = renderToStaticMarkup(createElement(MathRubricMatrix, { evidence, grades: [{ ...grade, math_artifact_version: 2 }], points: [], dirty: false,
      requesting: false, canRequest: false, onRequest: () => {}, onSelectStep: () => {} }));
    expect(html).toContain("建议区间"); expect(html).toContain("0–2"); expect(html).toContain("待确认"); expect(html).toContain("已过期");
    expect(html).toContain("定位数学步骤 step1");
  });
});
