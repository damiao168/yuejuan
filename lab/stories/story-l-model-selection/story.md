# Story L Fixed-Set Model Selection

## Spec v1

Compare rules, local 4B, and local 8B models by quality and operational cost.

## Spec Review

The governed synthetic test split has only two records and is not a useful selection set. Comparisons must freeze one independent file, hide labels from prompts, pin model artifacts, include failures and retries, and keep all runtime settings equal.

## Spec v2

Create a four-sample fixed set covering Chinese short-answer and math calculation full/partial/wrong answers. Compare deterministic rules, Qwen3 4B, and Qwen3 8B with dataset hashes, quality metrics, evidence/schema validity, failures, first-pass rate, and latency.

## Implementation

Added candidate registry, verified download tooling, server candidate switching, fixed dataset, benchmark runner, tests, and selection protocol.

## Implementation Review

Alias rules completed 4/4 with fixture-aligned exact matching. Qwen3 4B completed 4/4 with MAE 0.5 and p95 106.8 seconds, but overgraded the wrong-result math sample by two points. Qwen3 8B completed only 1/4, with one evidence-link failure and two repeated 120-second timeouts. Review also found that completed-only MAE could make the failed 8B look perfect and that the fixed set needed an explicit Story J governance audit.

## Implementation Fixes

Added completion-rate eligibility before quality ranking, marked quality metrics as completed-sample-only, excluded mock rules from LLM candidacy, enforced dataset source/privacy validation in the runner, added original input SHA256 to governance manifests, and generated an explicit decision report.

Added evidence-scope, source, complete-dataset, and frozen-test-hash binding so external or partial reports cannot unlock Pilot. On the 452-record JorGPT external test set, alias rules produced MAE 4.383/10. A three-question Qwen3 4B smoke produced MAE 1.333/10 but only 66.7% evidence compliance and p95 latency 189.9 seconds; it is ineligible for selection by construction.

## Acceptance

Accepted as a preliminary synthetic decision. Qwen3 4B is selected; 8B is rejected on this hardware; real Gold re-selection remains mandatory before pilot.
