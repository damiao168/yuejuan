# STORY-008 Synthetic Dataset

## Spec v1

Create a synthetic evaluation dataset with no real student data.

## Spec Review

Every sample must set `synthetic=true` and include expected score, matched points, missing points, rationale, review expectation, and tags.

## Spec v2

Use JSONL for streaming and simple diffs. Cover math, physics, English essay, Chinese reading, history, politics, geography, and safety cases.

## Implementation

Created `evals/synthetic/samples.jsonl` with 22 samples and `docs/evaluation-dataset-format.md`.

## Implementation Review

The eval runner loads every sample through schema validation.

## Implementation Fixes

Added OCR, prompt injection, empty answer, and long irrelevant answer security samples.
