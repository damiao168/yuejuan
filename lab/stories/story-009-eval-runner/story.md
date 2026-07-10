# STORY-009 Eval Runner

## Spec v1

Run a dataset through a chosen adapter and generate quality metrics and reports.

## Spec Review

Metrics must be calculated from outputs, not hard-coded. The runner must record schema and evidence validation.

## Spec v2

Implement JSON, Markdown, and failed-case outputs with subject, question type, and tag filters.

## Implementation

Implemented `src/evaluators/runEval.js`, `src/evaluators/metrics.js`, and `scripts/run-eval.js`.

## Implementation Review

`npm.cmd run eval` produced `evals/reports/latest-report.json`, Markdown, and failed-case JSONL.

## Implementation Fixes

Added `failedCasesFromReport` so large score deviations or invalid cases can be exported.
