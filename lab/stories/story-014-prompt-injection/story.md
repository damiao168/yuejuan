# STORY-014 Prompt Injection Suite

## Spec v1

Detect answer text that tries to override scoring instructions.

## Spec Review

The first implementation can be rule-based but must expose an extensible detector interface.

## Spec v2

Implement a `PromptInjectionDetector` class and feed it into adapter and verifier flows.

## Implementation

Implemented `src/guardrails/promptInjection.js` and security samples in `evals/synthetic/samples.jsonl`.

## Implementation Review

Tests confirm suspected injection adds `PROMPT_INJECTION_SUSPECTED` and forces human review.

## Implementation Fixes

Added Chinese and English patterns for role override, full-score requests, and schema bypass.
