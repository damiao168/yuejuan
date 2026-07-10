# STORY-007 Evidence Verifier

## Spec v1

Verify that matched rubric points have answer-local evidence.

## Spec Review

Evidence checks need normalized matching, prompt injection flags, OCR-low review, empty-answer review, and overscore checks.

## Spec v2

Implement a verifier that returns pass/fail, evidence validity rate, invalid points, forced risk flags, and forced human review.

## Implementation

Implemented `src/evidenceVerifier.js`.

## Implementation Review

Tests cover valid evidence, missing evidence, unknown rubric point, overscore, prompt injection, empty answers, and low OCR.

## Implementation Fixes

Added `applyEvidenceVerification` so eval and graders can merge forced flags into outputs.
