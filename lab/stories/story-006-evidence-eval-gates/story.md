# STORY-006 Evidence, Eval, And Gates

## Spec v1

Build evidence verification, synthetic data, metrics, and release gates.

## Spec Review

The first gate should prove the pipeline runs and enforce safety invariants. Pilot and production gates should remain strict and may fail until enough data exists.

## Spec v2

Create a dev gate that passes only with schema-valid mock-marked outputs and a running evidence verifier. Keep pilot/production thresholds in config.

## Implementation

Implemented `src/evidenceVerifier.js`, `src/evaluators`, `evals/synthetic/samples.jsonl`, and `config/release-gates.json`.

## Implementation Review

Tests cover prompt injection, OCR low confidence, empty answer, full eval, and gate behavior.

## Implementation Fixes

Added a synthetic security suite and a placeholder regression failed case.

After eval review, tightened the mock adapter so non-empty answers with zero rubric evidence are marked `INSUFFICIENT_EVIDENCE`, `AMBIGUOUS_ANSWER`, and `needs_human_review=true`.

After a second eval review, added an ambiguity heuristic for answers containing phrases such as `maybe`, `not sure`, `可能`, or `不确定`.
