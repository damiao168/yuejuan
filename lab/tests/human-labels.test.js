import test from "node:test";
import assert from "node:assert/strict";
import { validateHumanLabeledSample } from "../src/humanLabels.js";
import { baseRubric } from "./helpers.js";

function sample(overrides = {}) {
  return {
    sample_id: "human-1",
    anonymized_student_id: "anon-1",
    subject: "math",
    grade_level: "grade_7",
    question_type: "short_answer",
    question_text: "Explain why x=3 solves 2x=6.",
    max_score: 2,
    rubric: baseRubric(),
    answer_text: "2x=6 and x=3",
    ocr_confidence: 0.99,
    human_score: 2,
    human_matched_points: ["p1", "p2"],
    human_missing_points: [],
    human_deductions: [],
    human_rationale: "Both points are present.",
    labeler_id: "teacher-1",
    labeler_role: "teacher",
    label_timestamp: "2026-07-06T00:00:00.000Z",
    label_confidence: 0.95,
    privacy_status: "anonymized",
    synthetic: true,
    ...overrides
  };
}

test("valid human labeled sample passes", () => {
  assert.equal(validateHumanLabeledSample(sample()).valid, true);
});

test("human score range is validated", () => {
  const validation = validateHumanLabeledSample(sample({ human_score: 3 }));
  assert.equal(validation.valid, false);
  assert.match(validation.errors.join("\n"), /human_score/);
});

test("sensitive data in answer_text is rejected", () => {
  const validation = validateHumanLabeledSample(sample({ answer_text: "姓名: 张三 2x=6" }));
  assert.equal(validation.valid, false);
  assert.match(validation.errors.join("\n"), /sensitive/);
});
