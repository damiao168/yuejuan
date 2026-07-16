import test from "node:test";
import assert from "node:assert/strict";
import { PILOT_REQUIREMENTS, evaluatePilotReadiness } from "../src/release/pilot.js";

test("synthetic-only evidence remains not ready", () => {
  const evidence = Object.fromEntries(PILOT_REQUIREMENTS.map(([id]) => [id, true]));
  evidence.real_dataset_governed = false;
  evidence.real_model_selection_passed = false;
  const result = evaluatePilotReadiness(evidence);
  assert.equal(result.decision, "NOT_READY");
  assert.equal(result.blocker_count, 2);
});

test("all required evidence permits shadow pilot only", () => {
  const evidence = Object.fromEntries(PILOT_REQUIREMENTS.map(([id]) => [id, true]));
  const result = evaluatePilotReadiness(evidence);
  assert.equal(result.decision, "READY_FOR_SHADOW_PILOT");
  assert.equal(result.ready, true);
});

test("fine-tuning is not a mandatory pilot requirement", () => {
  assert.equal(PILOT_REQUIREMENTS.some(([id]) => id.includes("fine_tuning")), false);
});
