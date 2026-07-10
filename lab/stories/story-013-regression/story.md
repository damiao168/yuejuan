# STORY-013 Failure Regression

## Spec v1

Create a failure regression set so discovered errors become repeatable tests.

## Spec Review

Regression cases must avoid private identifiers and must include failure type, severity, bad output, and expected output.

## Spec v2

Use JSONL plus small CLI helpers for adding and running regression cases.

## Implementation

Created `evals/regression/failed_cases.jsonl`, `scripts/add-failed-case.js`, and `scripts/run-regression.js`.

## Implementation Review

`node scripts/run-regression.js` ran one placeholder case with zero failures.

## Implementation Fixes

The placeholder case targets prompt injection receiving score without evidence.
