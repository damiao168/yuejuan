import test from "node:test";
import assert from "node:assert/strict";
import { createQuestionGrader } from "../src/graders.js";
import { getSubjectStrategy } from "../src/subjects.js";
import { baseInput, baseRubric } from "./helpers.js";

test("english strategy requires essay human review", () => {
  const strategy = getSubjectStrategy("english");
  assert.equal(strategy.default_review_policy, "essay_and_discussion_always_review");
  assert.ok(strategy.required_risk_flags.includes("HUMAN_REVIEW_REQUIRED"));
});

test("math strategy requires step evidence constraints", () => {
  assert.ok(getSubjectStrategy("math").scoring_constraints.includes("do_not_award_for_final_answer_only_when_steps_required"));
});

test("physics strategy requires concept evidence", () => {
  assert.ok(getSubjectStrategy("physics").scoring_constraints.includes("require_concept_evidence"));
});

test("chinese strategy forbids rewarding length alone", () => {
  assert.ok(getSubjectStrategy("chinese").scoring_constraints.includes("do_not_reward_length_alone"));
});

test("grader factory routes supported question type", () => {
  const output = createQuestionGrader("short_answer").grade(baseInput());
  assert.equal(output.mock, true);
});

test("grader factory rejects unsupported question type", () => {
  assert.throws(() => createQuestionGrader("unsupported"), /Unsupported question type/);
});

test("essay grader output is marked for human review", () => {
  const rubric = baseRubric({
    question_type: "essay",
    max_score: 2,
    dimensions: ["content"],
    points: [{ id: "topic", description: "topic", score: 2, required: true, aliases: ["environment"], evidence_required: true }],
    rubric_version: "essay-rubric-v1"
  });
  const input = baseInput({
    subject: "english",
    question_type: "essay",
    rubric,
    max_score: 2,
    rubric_version: "essay-rubric-v1",
    answer_text: "environment"
  });
  const output = createQuestionGrader("essay").grade(input);
  assert.equal(output.needs_human_review, true);
});
