# STORY-005 Adapter Layer

## Spec v1

Provide mock, OpenAI-compatible, and local model adapter boundaries.

## Spec Review

Mock must never pretend to be real AI. OpenAI and local adapters must fail clearly when not configured.

## Spec v2

Implement a deterministic mock adapter for tests, plus placeholders that protect secrets and avoid fake capability.

## Implementation

Implemented `src/adapters/mockAdapter.js` and `src/adapters/index.js`.

## Implementation Review

Mock output is schema validated, `mock=true`, and includes `MOCK_OUTPUT`.

## Implementation Fixes

Added deterministic alias matching and explicit model info warning.
