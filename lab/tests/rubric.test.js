import test from "node:test";
import assert from "node:assert/strict";
import { hashRubric, validateRubricDsl } from "../src/rubric.js";
import { baseRubric } from "./helpers.js";

test("rubric DSL validates a normal rubric", () => {
  assert.equal(validateRubricDsl(baseRubric()).valid, true);
});

test("rubric point total must equal max_score unless allow_partial_total is set", () => {
  const validation = validateRubricDsl(baseRubric({ max_score: 3 }));
  assert.equal(validation.valid, false);
  assert.match(validation.errors.join("\n"), /must equal/);
});

test("rubric hash changes when rubric changes", () => {
  const first = baseRubric();
  const second = baseRubric({ points: [...baseRubric().points, { id: "p3", description: "check", score: 1, required: false, aliases: ["check"], evidence_required: true }], max_score: 3 });
  assert.notEqual(hashRubric(first), hashRubric(second));
});

test("essay rubric must include dimensions", () => {
  const validation = validateRubricDsl(baseRubric({ question_type: "essay" }));
  assert.equal(validation.valid, false);
  assert.match(validation.errors.join("\n"), /dimensions/);
});

test("calculation rubric must include steps", () => {
  const validation = validateRubricDsl(baseRubric({ question_type: "calculation" }));
  assert.equal(validation.valid, false);
  assert.match(validation.errors.join("\n"), /steps/);
});

test("rubric point match policy is explicit and validated", () => {
  const rubric = baseRubric();
  rubric.points[0].match_policy = "guess";
  assert.equal(validateRubricDsl(rubric).valid, false);
});
