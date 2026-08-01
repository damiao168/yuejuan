import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const root = join(dirname(fileURLToPath(import.meta.url)), "..");
const read = (...parts) => readFileSync(join(root, ...parts), "utf8");

const server = read("services", "api-gateway", "internal", "server", "server.go");
const handlers = read("services", "api-gateway", "internal", "subjective", "handlers.go");
const types = read("services", "api-gateway", "internal", "subjective", "types.go");
const runMigration = read("services", "api-gateway", "migrations", "000063_story063_subjective_grading_runs.sql");
const batchMigration = read("services", "api-gateway", "migrations", "000064_story063_subjective_grading_batches.sql");
const workerAPI = read("services", "subjective-grading-worker", "subjective_grading_worker", "api.py");
const workerRunner = read("services", "subjective-grading-worker", "subjective_grading_worker", "runner.py");
const compose = read("infra", "docker-compose", "docker-compose.yml");
const envExample = read("infra", "docker-compose", ".env.example");
const webAPI = read("apps", "web-admin", "src", "api", "subjectiveGrading.ts");
const webPage = read("apps", "web-admin", "src", "pages", "SubjectiveGradingBatchPage.tsx");
const routes = read("apps", "web-admin", "src", "router", "routes.tsx");
const smoke = read("infra", "docker-compose", "scripts", "story063-subjective-smoke-test.ps1");

for (const endpoint of [
  "/api/v1/subjective-grading-batches",
  "/enqueue",
  "/api/v1/internal/subjective-grading/runs/{runId}/execute",
  "/result",
  "/failure"
]) {
  assert.ok(server.includes(endpoint), `server route missing ${endpoint}`);
}

for (const invariant of [
  "subjective_task_lease_mismatch",
  "task.LeaseToken != input.LeaseToken",
  "RefreshBatch",
  "RunQueued",
  "subjective_grading_run"
]) {
  assert.ok(handlers.includes(invariant) || types.includes(invariant), `subjective domain invariant missing ${invariant}`);
}

assert.ok(runMigration.includes("uq_subjective_grading_run_request"), "run request identity must be unique");
assert.ok(runMigration.includes("'queued', 'processing', 'succeeded', 'failed', 'conflict'"), "run lifecycle constraint is incomplete");
assert.ok(batchMigration.includes("chk_subjective_grading_batch_counts"), "batch count constraint is missing");
assert.ok(batchMigration.includes("idx_subjective_grading_run_batch"), "batch/run lookup index is missing");

for (const endpoint of ["/tasks/claim", "/heartbeat", "/execute", "/result", "/failure"]) {
  assert.ok(workerAPI.includes(endpoint), `worker client endpoint missing ${endpoint}`);
}
assert.ok(workerRunner.includes("with _Heartbeat"), "worker must maintain its lease while the grading agent runs");
assert.ok(workerRunner.includes("source_type") && workerRunner.includes("subjective_grading_run"), "worker must reject unrelated task sources");

assert.ok(compose.includes('profiles: ["subjective-grading"]'), "subjective worker compose profile is missing");
for (const variable of [
  "EDUGRADE_SUBJECTIVE_WORKER_TENANT_CODE",
  "EDUGRADE_SUBJECTIVE_WORKER_USERNAME",
  "EDUGRADE_SUBJECTIVE_WORKER_PASSWORD"
]) {
  assert.ok(compose.includes(variable), `compose is missing ${variable}`);
  assert.ok(envExample.includes(variable), `.env.example is missing ${variable}`);
}

for (const endpoint of ["/api/v1/subjective-grading-batches", "/enqueue"]) {
  assert.ok(webAPI.includes(endpoint), `web batch API missing ${endpoint}`);
}
assert.ok(routes.includes('/grading/subjective-batches'), "admin batch route is missing");
assert.ok(routes.includes('permissions: ["grading:manage"]'), "admin batch route must be permission gated");
assert.ok(webPage.includes("getSubjectiveGradingBatch"), "admin batch page must refresh persisted progress");

assert.ok(smoke.includes("Invoke-SubjectiveApi"), "real integration smoke script is incomplete");
assert.ok(smoke.includes('status -eq "completed"') && smoke.includes('status -eq "failed"'), "smoke script must wait for a terminal batch status");
assert.equal(smoke.includes("Write-Host $Password"), false, "smoke script must never print the password");
assert.equal(smoke.includes("Write-Host $AccessToken"), false, "smoke script must never print the access token");

console.log("STORY-063 subjective grading orchestration invariants passed");
