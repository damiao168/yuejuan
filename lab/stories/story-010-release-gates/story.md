# STORY-010 Release Gates

## Spec v1

Define strict dev, pilot, and production gates.

## Spec Review

Thresholds must live in config and production cannot pass with too few samples.

## Spec v2

Use `config/release-gates.json` and a gate checker that returns pass/fail reasons.

## Implementation

Implemented `src/evaluators/gateChecker.js`, `scripts/check-gate.js`, and `docs/release-gates.md`.

## Implementation Review

Tests prove dev can pass and production fails on insufficient samples.

## Implementation Fixes

Added explicit checks for no score above max, mock marking, and evidence verifier execution.
