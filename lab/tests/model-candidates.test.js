import test from "node:test";
import assert from "node:assert/strict";
import { getModelCandidate, loadModelCandidates, validateModelCandidates } from "../src/runtime/modelCandidates.js";

test("model candidate registry pins 4B and 8B artifacts", () => {
  const registry = loadModelCandidates();
  assert.deepEqual(validateModelCandidates(registry), { valid: true, errors: [] });
  assert.equal(getModelCandidate("qwen3_4b", registry).expected_bytes, 2497280256);
  assert.equal(getModelCandidate("qwen3_8b", registry).expected_sha256, "d98cdcbd03e17ce47681435b5150e34c1417f50b5c0019dd560e4882c5745785");
});

test("model candidate paths cannot escape lab", () => {
  const registry = loadModelCandidates();
  registry.candidates[0].model_path = "../outside.gguf";
  const result = validateModelCandidates(registry);
  assert.equal(result.valid, false);
  assert.ok(result.errors.some((error) => error.includes("model_path")));
});
