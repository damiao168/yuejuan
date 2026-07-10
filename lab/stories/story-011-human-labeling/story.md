# STORY-011 Human Labeling

## Spec v1

Define a future teacher labeling format without importing real student data.

## Spec Review

The importer must reject bad scores, unknown rubric points, and sensitive identifiers in answer text.

## Spec v2

Implement a validator and importer script; keep privacy guidance in docs.

## Implementation

Implemented `src/humanLabels.js`, `scripts/import-human-labels.js`, and `docs/human-labeling-guideline.md`.

## Implementation Review

Tests cover valid labels, score range failure, and sensitive text rejection.

## Implementation Fixes

Added privacy status checks so non-synthetic samples must be anonymized.
