# STORY-001 Operating Model

## Spec v1

Create an isolated `lab` folder for Grading Agent work. The lab owns specs, prompts, schemas, evals, adapters, guardrails, and gates only.

## Spec Review

Risk: the pasted plan originally targets `ai-services/grading-agent-service`, but the current user explicitly requires a new `lab` folder and no other changes.

## Spec v2

All artifacts must live under `lab`. A local `AGENTS.md` records lab-only rules.

## Implementation

Created `README.md`, `AGENTS.md`, `docs/story-workflow.md`, and `stories/story-board.md`.

## Implementation Review

The rules avoid UI, EXE, database, grade publishing, and real student data.

## Implementation Fixes

Added command list and explicit story loop.
