import test from "node:test";
import assert from "node:assert/strict";
import { validateGradingInput, validateGradingOutput } from "../src/schemas/gradingSchema.js";
import { baseInput, baseOutput, baseRubric } from "./helpers.js";

test("valid input and output pass schema validation", () => {
  const input = baseInput();
  assert.equal(validateGradingInput(input).valid, true);
  assert.equal(validateGradingOutput(baseOutput(), input).valid, true);
});

test("output cannot exceed max score", () => {
  const validation = validateGradingOutput(baseOutput({ suggested_score: 3 }), baseInput());
  assert.equal(validation.valid, false);
  assert.match(validation.errors.join("\n"), /cannot exceed/);
});

test("output cannot use negative score", () => {
  const validation = validateGradingOutput(baseOutput({ suggested_score: -1 }), baseInput());
  assert.equal(validation.valid, false);
  assert.match(validation.errors.join("\n"), />= 0/);
});

test("confidence must be between 0 and 1", () => {
  const validation = validateGradingOutput(baseOutput({ confidence: 1.2 }), baseInput());
  assert.equal(validation.valid, false);
  assert.match(validation.errors.join("\n"), /confidence/);
});

test("model, prompt, and rubric versions are required", () => {
  const validation = validateGradingOutput(baseOutput({ model_version: "" }), baseInput());
  assert.equal(validation.valid, false);
  assert.match(validation.errors.join("\n"), /model_version/);
});

test("mock output must be marked with MOCK_OUTPUT", () => {
  const validation = validateGradingOutput(baseOutput({ risk_flags: [] }), baseInput());
  assert.equal(validation.valid, false);
  assert.match(validation.errors.join("\n"), /MOCK_OUTPUT/);
});

test("low OCR confidence requires human review", () => {
  const input = baseInput({ ocr_confidence: 0.5 });
  const validation = validateGradingOutput(baseOutput({ needs_human_review: false }), input);
  assert.equal(validation.valid, false);
  assert.match(validation.errors.join("\n"), /low OCR/);
});

test("essay output requires human review", () => {
  const rubric = baseRubric({
    question_type: "essay",
    max_score: 2,
    dimensions: ["content"],
    points: [
      { id: "p1", description: "topic", score: 2, required: true, aliases: ["environment"], evidence_required: true }
    ]
  });
  const input = baseInput({
    subject: "english",
    question_type: "essay",
    rubric,
    answer_text: "environment"
  });
  const output = baseOutput({
    max_score: 2,
    suggested_score: 2,
    matched_points: [{ rubric_point_id: "p1", score: 2, evidence_ids: ["ev-p1"] }],
    evidence: [{ evidence_id: "ev-p1", rubric_point_id: "p1", text_excerpt: "environment", location: "answer_text", confidence: 0.9 }],
    needs_human_review: false,
    rubric_version: rubric.rubric_version
  });
  const validation = validateGradingOutput(output, input);
  assert.equal(validation.valid, false);
  assert.match(validation.errors.join("\n"), /essay/);
});

test("matched required rubric point must have evidence", () => {
  const validation = validateGradingOutput(baseOutput({ evidence: [], matched_points: [{ rubric_point_id: "p1", score: 1, evidence_ids: [] }], suggested_score: 1 }), baseInput());
  assert.equal(validation.valid, false);
  assert.match(validation.errors.join("\n"), /lacks evidence/);
});

test("malformed output arrays return validation errors instead of throwing", () => {
  const validation = validateGradingOutput(baseOutput({ evidence: {}, matched_points: {} }), baseInput());
  assert.equal(validation.valid, false);
  assert.match(validation.errors.join("\n"), /must be an array/);
});

test("every rubric point must be classified exactly once", () => {
  const duplicate = validateGradingOutput(baseOutput({
    missing_points: [{ rubric_point_id: "p1", reason: "also missing" }]
  }), baseInput());
  assert.equal(duplicate.valid, false);
  assert.match(duplicate.errors.join("\n"), /classified more than once/);

  const incomplete = validateGradingOutput(baseOutput({
    suggested_score: 1,
    matched_points: [{ rubric_point_id: "p1", score: 1, evidence_ids: ["ev-p1"] }],
    evidence: [baseOutput().evidence[0]]
  }), baseInput());
  assert.equal(incomplete.valid, false);
  assert.match(incomplete.errors.join("\n"), /was not classified: p2/);
});

test("matched point score and evidence links must obey their rubric contract", () => {
  const validation = validateGradingOutput(baseOutput({
    suggested_score: 2,
    matched_points: [
      { rubric_point_id: "p1", score: 2, evidence_ids: ["ev-p1", "ev-p1"] },
      { rubric_point_id: "p2", score: 0, evidence_ids: ["ev-p2"] }
    ]
  }), baseInput());
  assert.equal(validation.valid, false);
  assert.match(validation.errors.join("\n"), /exceeds rubric allowance/);
  assert.match(validation.errors.join("\n"), /duplicate id/);
});

test("missing rubric points require a known id and a reason", () => {
  const validation = validateGradingOutput(baseOutput({
    suggested_score: 1,
    matched_points: [{ rubric_point_id: "p1", score: 1, evidence_ids: ["ev-p1"] }],
    missing_points: [{ rubric_point_id: "unknown", reason: "" }],
    evidence: [baseOutput().evidence[0]]
  }), baseInput());
  assert.equal(validation.valid, false);
  assert.match(validation.errors.join("\n"), /reason is required/);
  assert.match(validation.errors.join("\n"), /does not exist/);
});
