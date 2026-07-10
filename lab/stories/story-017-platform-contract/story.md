# STORY-017 Platform Integration Contract

## Spec v1

Define how the future main platform can call Grading Agent Lab.

## Spec Review

This must remain a contract only. It must not alter platform code, database schema, or UI.

## Spec v2

Write endpoint, request, response, timeout, retry, idempotency, logging, and review-task rules.

## Implementation

Created `docs/platform-integration-contract.md` and `openapi/grading-agent.openapi.json`.

## Implementation Review

The contract states that suggested score and teacher-confirmed score must remain separate.

## Implementation Fixes

Added explicit error codes and sensitive logging restrictions.
