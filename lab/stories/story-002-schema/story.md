# STORY-003 Structured Schema

## Spec v1

Define machine-checkable GradingInput, Rubric, Evidence, and GradingOutput validation.

## Spec Review

The schema must catch overscore, negative score, confidence bounds, missing versions, missing evidence, essay/discussion review, low OCR review, and mock marking.

## Spec v2

Use dependency-free JavaScript validators so the lab does not alter root dependencies.

## Implementation

Implemented `src/schemas/gradingSchema.js`.

## Implementation Review

Tests cover normal output and required failure paths.

## Implementation Fixes

Added `OCR_TEXT_EMPTY_REVIEW_REQUIRED` as an allowed risk flag because the evidence verifier can force it.
