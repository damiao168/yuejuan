# Grading Agent Lab Story Board

| Story | Scope | Status | Verification |
| --- | --- | --- | --- |
| STORY-001 | Lab operating model and boundary rules | Done | `README.md`, `AGENTS.md`, `docs/story-workflow.md` |
| STORY-002 | Enterprise Grading Agent specification | Done | `docs/grading-agent-spec.md` |
| STORY-003 | Structured input/output schema | Done | `src/schemas/gradingSchema.js`, `tests/schema.test.js` |
| STORY-004 | Rubric DSL and version hash | Done | `src/rubric.js`, `examples/rubrics`, `tests/rubric.test.js` |
| STORY-005 | Prompt templates and registry | Done | `src/prompts`, `tests/prompt-registry.test.js` |
| STORY-006 | Model adapter layer | Done | `src/adapters`, `tests/adapter.test.js` |
| STORY-007 | Evidence verifier | Done | `src/evidenceVerifier.js`, `tests/evidence-verifier.test.js` |
| STORY-008 | Synthetic eval dataset | Done | `evals/synthetic/samples.jsonl`, `docs/evaluation-dataset-format.md` |
| STORY-009 | Eval runner and metrics | Done | `src/evaluators/runEval.js`, `src/evaluators/metrics.js`, `tests/eval-gate.test.js` |
| STORY-010 | Release gates | Done | `config/release-gates.json`, `src/evaluators/gateChecker.js`, `docs/release-gates.md` |
| STORY-011 | Human labeling format | Done | `src/humanLabels.js`, `docs/human-labeling-guideline.md`, `tests/human-labels.test.js` |
| STORY-012 | Model optimization roadmap | Done | `docs/model-optimization-roadmap.md` |
| STORY-013 | Failure regression set | Done | `evals/regression/failed_cases.jsonl`, `scripts/run-regression.js` |
| STORY-014 | Prompt injection suite | Done | `src/guardrails/promptInjection.js`, synthetic security samples |
| STORY-015 | Subject strategies | Done | `src/subjects.js`, `docs/subject-grading-strategies.md` |
| STORY-016 | Question type graders | Done | `src/graders.js`, `tests/subjects-graders.test.js` |
| STORY-017 | Platform integration contract | Done | `docs/platform-integration-contract.md`, `openapi/grading-agent.openapi.json` |
| STORY-018 | Enterprise pre-release audit | Done | `docs/pre-release-audit.md` |
| STORY-019 | High-risk implementation fixes | Done | `docs/high-risk-fixes.md`, `src/adapters/mockAdapter.js` |
| STORY-020 | Platform current API mapping | Done | `docs/platform-current-api-mapping.md` |
| STORY-021 | Current Go grading gap analysis | Done | `docs/current-grading-gap-analysis.md` |
| STORY-022 | Integration option selection | Done | `docs/grading-agent-integration-options.md` |
| STORY-023 | Platform mock output eval compatibility | Done | `docs/platform-mock-output-eval-compatibility.md` |
| STORY-024 | Risk flags naming standard | Done | `docs/risk-flags-naming-standard.md` |
| STORY-025 / Story F | Rubric field extension proposal | Done | `docs/rubric-field-extension-proposal.md` |

## Finite Product Program

This program supersedes the old open-ended `STORY-026` candidate. It ends at Story R and does not create another story automatically.

| Story | Scope | Status | Verification |
| --- | --- | --- | --- |
| Story G | Product capability boundary and executable capability matrix | Done | `docs/product-capability-boundary.md`, `config/capability-matrix.json`, `tests/capabilities.test.js` |
| Story H | Local model runtime and hardware baseline | Done | `docs/local-model-runtime-baseline.md`, `evals/reports/local-runtime-benchmark.json`, `tests/local-runtime.test.js` |
| Story I | Real local model adapter and structured output | Done | `docs/local-model-adapter.md`, `evals/reports/local-adapter-smoke.json`, `tests/local-adapter.test.js` |
| Story J | Dataset governance and grouped split | Done | `docs/dataset-governance.md`, `evals/governed/synthetic/manifest.json`, `tests/dataset-governance.test.js` |
| Story K | Gold annotation, blind assignment, adjudication, and agreement | Done | `docs/gold-annotation-protocol.md`, `docs/blind-annotation-workflow.md`, `evals/reports/annotation-workflow-demo.json`, `tests/annotation-workflow.test.js` |
| Story L | Fixed-set rule/4B/8B benchmark and model selection | Done | `docs/model-selection-protocol.md`, `evals/reports/model-selection-decision.json`, `tests/model-selection.test.js` |
| Story M | Three offline adversarial agents | Done | `docs/adversarial-agent-suite.md`, `evals/reports/adversarial-suite.json`, `tests/adversarial.test.js` |
| Story N | Calibration, fairness, and slice evaluation | Done | `docs/calibration-and-fairness.md`, `evals/reports/calibration-fairness.json`, `tests/calibration-fairness.test.js` |
| Story O | Failure-driven prompt and rubric optimization | Done | `docs/prompt-rubric-optimization-v2.md`, `evals/reports/local-regression-v2.json`, `tests/local-adapter.test.js` |
| Story P | Fine-tuning decision gate and optional QLoRA | Done: do not train | `docs/qlora-training-protocol.md`, `evals/reports/fine-tuning-decision.json`, `tests/fine-tuning-decision.test.js` |
| Story Q | Pilot freeze package | Done: NOT_READY | `docs/pilot-freeze-package.md`, `evals/pilot/pilot-candidate-v1/manifest.json`, `tests/pilot-readiness.test.js` |
| Story R | Lab graduation audit and platform handoff | Done | `docs/lab-graduation-audit.md`, `docs/platform-handoff-package.md`, `evals/reports/graduation-audit.json` |

## Program End

Story G-R is complete. No Story S is created. Final state: `LAB_IMPLEMENTATION_COMPLETE`, `PILOT_NOT_READY`. Future work is driven only by the four real-evidence Pilot blockers and requires explicit authorization for any main-project integration.
