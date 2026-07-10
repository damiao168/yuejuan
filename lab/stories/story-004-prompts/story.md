# STORY-004 Prompt Registry

## Spec v1

Prompts must be files, versioned, registered, and checksummed.

## Spec Review

Prompts must state that AI only suggests scores, must obey rubrics, must ignore student-in-answer instructions, and must output schema-shaped JSON.

## Spec v2

Implement a small registry with prompt id, version, question type, path, checksum, created date, and changelog.

## Implementation

Implemented `src/prompts/registry.js` and six prompt files.

## Implementation Review

Registry tests load every prompt, validate checksum stability, and reject unknown prompts.

## Implementation Fixes

Added evidence verification prompt as a separate registered prompt.
