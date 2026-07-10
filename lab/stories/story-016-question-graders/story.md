# STORY-016 Question Type Graders

## Spec v1

Add question-type graders and a factory.

## Spec Review

All graders must validate input, call an adapter, apply evidence verification, validate output, and reject unsupported types.

## Spec v2

Implement a base grader plus type-specific classes for the six supported question types.

## Implementation

Implemented `src/graders.js`.

## Implementation Review

Tests cover factory routing, unsupported type rejection, and essay human review.

## Implementation Fixes

Kept the first implementation thin so future stories can specialize fill-blank and numeric rules without duplicating validation.
