import test from "node:test";
import assert from "node:assert/strict";
import {
  capabilityRequiresHumanReview,
  loadCapabilityMatrix,
  resolveCapability,
  validateCapabilityMatrix
} from "../src/capabilities.js";

test("default capability matrix is valid and cannot publish final grades", () => {
  const matrix = loadCapabilityMatrix();
  assert.deepEqual(validateCapabilityMatrix(matrix), { valid: true, errors: [] });
  assert.equal(matrix.final_grade_publication_allowed, false);
});

test("math numeric routes to rule assistance for the launch grade level", () => {
  const route = resolveCapability({ subject: "math", question_type: "numeric", grade_level: "junior_middle" });
  assert.equal(route.mode, "rule_assisted");
  assert.equal(route.grader, "rule");
  assert.equal(route.reason, "in_scope");
  assert.equal(capabilityRequiresHumanReview(route), false);
});

test("chinese short answer requires teacher review", () => {
  const route = resolveCapability({ subject: "chinese", question_type: "short_answer", grade_level: "junior_middle" });
  assert.equal(route.mode, "llm_assisted");
  assert.equal(capabilityRequiresHumanReview(route), true);
});

test("essay wildcard stays in shadow mode", () => {
  const route = resolveCapability({ subject: "english", question_type: "essay", grade_level: "junior_middle" });
  assert.equal(route.mode, "shadow");
  assert.equal(route.delivery, "shadow_only");
  assert.equal(capabilityRequiresHumanReview(route), true);
});

test("out-of-scope combinations route to humans", () => {
  const route = resolveCapability({ subject: "history", question_type: "short_answer", grade_level: "junior_middle" });
  assert.equal(route.mode, "human_only");
  assert.equal(route.reason, "subject_question_type_out_of_scope");
  assert.equal(capabilityRequiresHumanReview(route), true);
});

test("out-of-scope grade levels route to humans", () => {
  const route = resolveCapability({ subject: "math", question_type: "numeric", grade_level: "senior_high" });
  assert.equal(route.mode, "human_only");
  assert.equal(route.reason, "grade_level_out_of_scope");
});

test("external higher-education computer science remains human-only", () => {
  const route = resolveCapability({ subject: "computer_science", question_type: "short_answer", grade_level: "higher_education" });
  assert.equal(route.mode, "human_only");
  assert.equal(route.reason, "grade_level_out_of_scope");
  assert.equal(capabilityRequiresHumanReview(route), true);
});

test("matrix rejects final grade publication and unsafe LLM review policy", () => {
  const matrix = loadCapabilityMatrix();
  matrix.final_grade_publication_allowed = true;
  matrix.capabilities[4].review_policy = "risk_based";
  const result = validateCapabilityMatrix(matrix);
  assert.equal(result.valid, false);
  assert.ok(result.errors.some((error) => error.includes("publication")));
  assert.ok(result.errors.some((error) => error.includes("LLM-backed")));
});
