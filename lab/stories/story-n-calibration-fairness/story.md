# Story N Calibration And Fairness

## Spec v1

Calibrate model confidence and measure fairness across meaningful product slices.

## Spec Review

Model confidence is not an accuracy probability. Calibration fit and evaluation must be separated, different score scales must be normalized, small slices must not disappear, and demographic attributes cannot be collected casually.

## Spec v2

Fit isotonic calibration on deterministic reliability signals, evaluate ECE/Brier and coverage-risk on a held-out partition, report normalized operational slices with minimum counts, and forbid runtime promotion of synthetic calibration.

## Implementation

Added reliability signals, isotonic calibration, ECE/Brier, coverage-risk, slice metrics, gates, synthetic observations, CLI, tests, and documentation.

## Implementation Review

The first run produced ECE and Brier 0.25, but review found that signals between observed isotonic blocks incorrectly received the final high-probability block. One out-of-scope failure was therefore assigned probability 1. The report also showed 0.50 NMAE gaps across subject, question type, and OCR slices.

## Implementation Fixes

Changed prediction to a conservative left-continuous step function, merged duplicate signal blocks, and added a between-block regression test. Recalculation reduced ECE to 0.125 and Brier to 0.125 while preserving explicit large slice gaps and runtime non-promotion.

## Acceptance

Accepted for mechanics. Five calibration tests pass, dev passes, pilot fails for real-data and slice requirements, and runtime confidence remains disabled.
