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
| STORY-025 | Rubric field extension proposal | Done | `docs/rubric-field-extension-proposal.md` |

Next story: STORY-026, recommended candidate: Prompt injection defense migration proposal.
