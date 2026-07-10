# STORY-020 Platform Current API Mapping

## Spec v1

Create a documentation-only mapping between the current Go grading chain and the lab Grading Agent contracts.

## Spec Review

The mapping must avoid runtime integration, must not modify the main platform, and must distinguish concept alignment from code-level compatibility.

## Spec v2

Use the actual repository path `lab/` while acknowledging that the pasted plan refers to `lab/grading-agent-lab/`. Cover GradingInput, GradingOutput, Rubric, Evidence, risk flags, and API paths.

## Implementation

Created `docs/platform-current-api-mapping.md`.

## Implementation Review

The document identifies direct mappings, required conversions, missing platform fields, lab-only fields, and the reasons direct integration is not recommended.

## Implementation Fixes

Self-review tightened the distinction between existing `/api/v1/...` platform APIs and future internal `/grading/...` service APIs.

## Acceptance

Accepted as Story A. It provides the input needed for Story B gap analysis.
