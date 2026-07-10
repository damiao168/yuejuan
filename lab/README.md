# Grading Agent Lab

This folder is an isolated lab for Grading Agent specifications, schemas, prompts, synthetic evals, adapters, guardrails, and release gates.

It intentionally does not modify the main EduGrade platform, database, UI, EXE packaging, permission flows, grade publishing, or appeal workflows.

## Commands

Run from `lab/`:

```bash
npm test
npm run eval
npm run gate:dev
```

## Story Loop

Every story follows this loop:

1. Write specification.
2. Review specification.
3. Revise specification.
4. Implement inside `lab` only.
5. Review implementation.
6. Revise implementation.
7. Move to the next story.

The current board is in [stories/story-board.md](stories/story-board.md).

## Safety Rules

- No real student private data.
- Every synthetic record must use `synthetic=true`.
- Mock adapter outputs must use `mock=true` and include `MOCK_OUTPUT`.
- AI scoring outputs must pass schema validation.
- Suggested scores must stay within `[0, max_score]`.
- Essay and discussion answers default to human review.
- OCR confidence below `0.85` forces human review.
- Matched rubric points require answer-local evidence.
- Prompt, model, and rubric versions are mandatory.
