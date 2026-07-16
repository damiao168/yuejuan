#!/usr/bin/env node
import { inspectLocalRuntime, loadLocalRuntimeManifest, resolveLocalRuntimePaths, sha256File } from "../src/runtime/localRuntime.js";

const fullHash = process.argv.includes("--full-hash");
const manifest = loadLocalRuntimeManifest();
const inspection = inspectLocalRuntime(manifest);
if (fullHash && inspection.model.exists) {
  inspection.model.sha256 = await sha256File(resolveLocalRuntimePaths(manifest).model_path);
  inspection.model.sha256_matches = inspection.model.sha256 === manifest.model.expected_sha256;
}
inspection.ready = inspection.manifest_validation.valid && inspection.server.exists && inspection.benchmark.exists &&
  inspection.model.size_matches && (!fullHash || inspection.model.sha256_matches);
console.log(JSON.stringify(inspection, null, 2));
if (!inspection.ready) process.exitCode = 1;
