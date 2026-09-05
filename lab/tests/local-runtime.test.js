import test from "node:test";
import assert from "node:assert/strict";
import {
  LAB_ROOT,
  inspectLocalRuntime,
  loadLocalRuntimeManifest,
  resolveLocalRuntimePaths,
  validateLocalRuntimeManifest
} from "../src/runtime/localRuntime.js";

test("local runtime manifest pins a CPU-only 4B grading baseline", () => {
  const manifest = loadLocalRuntimeManifest();
  assert.deepEqual(validateLocalRuntimeManifest(manifest), { valid: true, errors: [] });
  assert.equal(manifest.model.file, "Qwen3-4B-Q4_K_M.gguf");
  assert.equal(manifest.model.quantization, "Q4_K_M");
  assert.match(manifest.runtime.source_url, /^https:\/\/github\.com\//);
  assert.match(manifest.runtime.expected_sha256, /^[a-f0-9]{64}$/);
  assert.equal(manifest.execution.gpu_layers, 0);
  assert.equal(manifest.execution.parallel_requests, 1);
  assert.equal(manifest.execution.context_tokens, 16384);
  assert.equal(manifest.execution.temperature, 0);
});

test("runtime and model paths remain under lab", () => {
  const paths = resolveLocalRuntimePaths();
  for (const path of Object.values(paths)) assert.ok(path.startsWith(LAB_ROOT));
});

test("runtime manifest rejects paths outside lab", () => {
  const manifest = loadLocalRuntimeManifest();
  manifest.model.model_path = "../outside.gguf";
  const result = validateLocalRuntimeManifest(manifest);
  assert.equal(result.valid, false);
  assert.ok(result.errors.some((error) => error.includes("model_path")));
  assert.throws(() => resolveLocalRuntimePaths(manifest), /Invalid local runtime manifest/);
  const inspection = inspectLocalRuntime(manifest);
  assert.equal(inspection.manifest_validation.valid, false);
  assert.equal(inspection.model.path, null);
  assert.equal(inspection.model.exists, false);
});
