import test from "node:test";
import assert from "node:assert/strict";
import { createAdversarialAgents } from "../src/adversaries/index.js";
import { checkAdversarialGate, runAdversarialSuite } from "../src/evaluators/adversarial.js";

test("exactly three adversarial agents run offline", () => {
  const agents = createAdversarialAgents();
  assert.equal(agents.length, 3);
  assert.deepEqual(agents.map((agent) => agent.agent_id), [
    "prompt_injection_adversary", "rubric_exploit_adversary", "evidence_integrity_adversary"
  ]);
  for (const agent of agents) {
    const cases = agent.generate();
    assert.ok(cases.filter((item) => item.attack).length >= 25);
    assert.ok(cases.filter((item) => !item.attack).length >= 25);
    assert.ok(new Set(cases.filter((item) => item.attack).map((item) => item.family)).size >= 5);
    assert.ok(new Set(cases.filter((item) => !item.attack).map((item) => item.family)).size >= 5);
    assert.equal(new Set(cases.map((item) => item.case_id)).size, cases.length);
  }
});

test("development adversarial gate catches every attack without control false positives", () => {
  const reports = runAdversarialSuite();
  assert.ok(reports.every((report) => report.attack_recall === 1));
  assert.ok(reports.every((report) => report.control_false_positive_rate === 0));
  assert.deepEqual(checkAdversarialGate(reports, "dev"), { level: "dev", passed: true, reasons: [] });
});

test("pilot adversarial gate passes the expanded deterministic suite", () => {
  const gate = checkAdversarialGate(runAdversarialSuite(), "pilot");
  assert.deepEqual(gate, { level: "pilot", passed: true, reasons: [] });
});

test("adversarial gate fails closed when required agents are absent", () => {
  const emptyGate = checkAdversarialGate([], "pilot");
  assert.equal(emptyGate.passed, false);
  assert.equal(emptyGate.reasons.filter((reason) => reason.startsWith("missing adversarial agent")).length, 3);

  const reports = runAdversarialSuite();
  const missingGate = checkAdversarialGate(reports.slice(1), "pilot");
  assert.equal(missingGate.passed, false);
  assert.ok(missingGate.reasons.includes(`missing adversarial agent ${reports[0].agent_id}`));
});

test("adversarial gate rejects duplicate agents and tampered aggregates", () => {
  const reports = runAdversarialSuite();
  reports[0].attack_count += 100;
  reports.push(structuredClone(reports[1]));
  const gate = checkAdversarialGate(reports, "pilot");
  assert.equal(gate.passed, false);
  assert.ok(gate.reasons.includes(`${reports[0].agent_id}.attack_count does not match results`));
  assert.ok(gate.reasons.includes(`duplicate adversarial agent ${reports[1].agent_id}`));
});
