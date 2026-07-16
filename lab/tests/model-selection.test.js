import test from "node:test";
import assert from "node:assert/strict";
import { compareModelSelectionReports, qualifyPilotModelSelection, selectDiverseBenchmarkSamples } from "../src/evaluators/modelSelection.js";

function report(candidateId, { mock = false, completed = 4, failed = 0, mae = 0.5, p95 = 100000 } = {}) {
  return {
    candidate_id: candidateId,
    synthetic_only: true,
    model_info: { mock },
    evidence_scope: "synthetic_development",
    dataset: { sha256: "same", total_samples: 4, source_id: "lab_synthetic", evidence_scope: "synthetic_development" },
    quality: { mae, exact_agreement: 0.75, schema_validity_rate: 1, evidence_validity_rate: 1 },
    operations: { completed, failed, latency_ms_p95: p95 }
  };
}

test("selection excludes failed model even when completed-sample MAE looks perfect", () => {
  const decision = compareModelSelectionReports([
    report("rules", { mock: true, mae: 0 }),
    report("4b"),
    report("8b", { completed: 1, failed: 3, mae: 0, p95: 108000 })
  ]);
  assert.equal(decision.selected_local_llm, "4b");
  assert.equal(decision.candidates.find((candidate) => candidate.candidate_id === "8b").eligible, false);
  assert.equal(decision.candidates.find((candidate) => candidate.candidate_id === "rules").role, "deterministic_reference");
  assert.equal(decision.evidence_scope, "synthetic_development");
});

test("external benchmark cannot qualify as Pilot real model selection", () => {
  const external = {
    synthetic_only: false,
    evidence_scope: "external_real_benchmark",
    dataset_source_id: "jorgpt",
    dataset_sha256: "test-hash",
    selected_local_llm: "qwen3_4b",
    decision_status: "external_benchmark_only"
  };
  const manifest = {
    source: { source_id: "jorgpt", evidence_scope: "external_real_benchmark" },
    files: { test: { sha256: "test-hash" } }
  };
  const result = qualifyPilotModelSelection(external, manifest);
  assert.equal(result.passed, false);
  assert.ok(result.reasons.some((reason) => reason.includes("pilot_in_domain_gold")));
});

test("Pilot model selection is bound to in-domain source and frozen test hash", () => {
  const decision = {
    synthetic_only: false,
    evidence_scope: "pilot_in_domain_gold",
    dataset_source_id: "school_gold_v1",
    dataset_sha256: "frozen-test",
    selected_local_llm: "qwen3_4b",
    decision_status: "pilot_real_gold_selection"
  };
  const manifest = {
    source: { source_id: "school_gold_v1", evidence_scope: "pilot_in_domain_gold" },
    files: { test: { sha256: "frozen-test" } }
  };
  assert.deepEqual(qualifyPilotModelSelection(decision, manifest), { passed: true, reasons: [] });
  decision.dataset_sha256 = "other";
  assert.equal(qualifyPilotModelSelection(decision, manifest).passed, false);
});

test("selection rejects reports from different datasets", () => {
  const second = report("8b");
  second.dataset.sha256 = "different";
  assert.throws(() => compareModelSelectionReports([report("4b"), second]), /same dataset hash/);
});

test("smoke subsets select distinct questions first and cannot win selection", () => {
  const samples = [
    { sample_id: "q2-a", question_id: "q2", rubric_version: "v1" },
    { sample_id: "q1-a", question_id: "q1", rubric_version: "v1" },
    { sample_id: "q1-b", question_id: "q1", rubric_version: "v1" },
    { sample_id: "q3-a", question_id: "q3", rubric_version: "v1" }
  ];
  assert.deepEqual(selectDiverseBenchmarkSamples(samples, 3).map((sample) => sample.sample_id), ["q1-a", "q2-a", "q3-a"]);
  const subset = report("4b");
  subset.dataset.complete_dataset = false;
  const full = report("8b", { mae: 1 });
  const decision = compareModelSelectionReports([subset, full]);
  assert.equal(decision.candidates.find((candidate) => candidate.candidate_id === "4b").eligible, false);
  assert.equal(decision.selected_local_llm, "8b");
});
