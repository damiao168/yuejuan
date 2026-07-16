# Story G Product Capability Boundary

## Spec v1

Define the first product profile by subject, grade level, question type, grading method, delivery mode, and review requirement. The agent remains suggestion-only.

## Spec Review

The initial concept was too easy to interpret as broad nine-subject support. It also needed machine-enforced fallback behavior and a distinction between teacher-visible suggestions and shadow-only results.

## Spec v2

Limit the local pilot to junior-middle rule scoring for selected numeric questions, hybrid math calculation, Chinese short answer assistance, and shadow-only long-form grading. All unsupported combinations must fail closed to human grading. Final-grade publication must be structurally disabled.

## Implementation

- Added `docs/product-capability-boundary.md`.
- Added `config/capability-matrix.json`.
- Added validator and resolver in `src/capabilities.js`.
- Added safety and routing tests in `tests/capabilities.test.js`.

## Implementation Review

Review found that wildcard long-form routes could accidentally become teacher suggestions and that model-backed routes could be configured for risk-only review. Validation now rejects both states. Grade-level mismatch is resolved before subject matching.

## Implementation Fixes

Added cross-field route validation, explicit profile IDs and reason codes, shadow-only enforcement, LLM review enforcement, and fail-closed defaults.

## Acceptance

Accepted when all lab tests pass and the capability tests prove that no configuration can publish final grades or silently expand the pilot scope.
