#!/usr/bin/env node
import { spawnSync } from "node:child_process";
import { mkdirSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { cpus, freemem, totalmem } from "node:os";
import { LAB_ROOT, loadLocalRuntimeManifest, resolveLocalRuntimePaths } from "../src/runtime/localRuntime.js";

const manifest = loadLocalRuntimeManifest();
const paths = resolveLocalRuntimePaths(manifest);
const args = [
  "-m", paths.model_path,
  "-p", "512",
  "-n", "128",
  "-t", String(manifest.execution.threads),
  "-ngl", String(manifest.execution.gpu_layers),
  "-r", "3",
  "-o", "json"
];
const startedAt = new Date();
const result = spawnSync(paths.benchmark_binary, args, { encoding: "utf8", maxBuffer: 16 * 1024 * 1024 });
if (result.error) throw result.error;
if (result.status !== 0) throw new Error(`llama-bench failed (${result.status}): ${result.stderr}`);

let runs;
try {
  runs = JSON.parse(result.stdout);
} catch (error) {
  throw new Error(`llama-bench did not return JSON: ${error.message}\n${result.stdout.slice(0, 1000)}`);
}
const report = {
  schema_version: "local-benchmark-v1",
  executed_at: startedAt.toISOString(),
  runtime: { name: manifest.runtime.name, release: manifest.runtime.release },
  model: {
    repository: manifest.model.repository,
    file: manifest.model.file,
    quantization: manifest.model.quantization,
    bytes: manifest.model.expected_bytes
  },
  execution: manifest.execution,
  machine: {
    cpu: cpus()[0]?.model ?? "unknown",
    logical_processors: cpus().length,
    total_memory_bytes: totalmem(),
    free_memory_before_bytes: freemem()
  },
  command: { binary: paths.benchmark_binary, args },
  runs
};
const output = join(LAB_ROOT, "evals", "reports", "local-runtime-benchmark.json");
mkdirSync(dirname(output), { recursive: true });
writeFileSync(output, `${JSON.stringify(report, null, 2)}\n`, "utf8");
console.log(JSON.stringify({ ok: true, report: output, runs }, null, 2));
