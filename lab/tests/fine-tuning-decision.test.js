import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { collectFineTuningEvidence, evaluateFineTuningDecision } from "../src/training/decision.js";

const gate = JSON.parse(readFileSync("config/fine-tuning-gates.json", "utf8"));

test("current synthetic-only evidence forbids fine-tuning", () => {
  const evidence = collectFineTuningEvidence();
  const decision = evaluateFineTuningDecision(evidence, gate);
  assert.equal(decision.decision, "do_not_train");
  assert.equal(decision.eligible, false);
  assert.ok(decision.unmet_conditions.some((reason) => reason.includes("real_gold_training_records")));
  assert.ok(decision.unmet_conditions.some((reason) => reason.includes("teacher_qwk")));
  assert.ok(decision.unmet_conditions.some((reason) => reason.includes("real Gold baseline")));
});

test("complete governed evidence can become QLoRA eligible", () => {
  const evidence = {
    real_gold_training_records: 8000,
    real_gold_test_records: 1000,
    teacher_qwk: 0.86,
    prompt_plateau_iterations: 3,
    license_approved: true,
    privacy_approved: true,
    grouped_split_verified: true,
    baseline_is_real: true,
    frozen_test_set: true,
    latest_prompt_regression_passed: true,
    selected_base_model: "qwen3_4b"
  };
  assert.equal(evaluateFineTuningDecision(evidence, gate).decision, "qlora_eligible");
});

test("training remains forbidden when the test set is not frozen", () => {
  const evidence = {
    real_gold_training_records: 8000, real_gold_test_records: 1000, teacher_qwk: 0.9,
    prompt_plateau_iterations: 4, license_approved: true, privacy_approved: true,
    grouped_split_verified: true, baseline_is_real: true, frozen_test_set: false,
    latest_prompt_regression_passed: true
  };
  const decision = evaluateFineTuningDecision(evidence, gate);
  assert.equal(decision.eligible, false);
  assert.ok(decision.unmet_conditions.some((reason) => reason.includes("frozen real test set")));
});
