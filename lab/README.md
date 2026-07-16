# Grading Agent Lab

This folder is an isolated lab for Grading Agent specifications, schemas, prompts, synthetic evals, adapters, guardrails, and release gates.

It intentionally does not modify the main EduGrade platform, database, UI, EXE packaging, permission flows, grade publishing, or appeal workflows.

## Commands

Run from `lab/`:

```bash
npm test
npm run eval
npm run gate:dev
npm run dataset:jorgpt:prepare
npm run dataset:jorgpt:import
npm run dataset:jorgpt:govern
npm run benchmark:jorgpt:rules
npm run annotations:workflow:demo
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

## Product Program Status

The finite Story G-R product program is complete. The automated graduation audit reports `LAB_IMPLEMENTATION_COMPLETE`. The Pilot freeze reports `NOT_READY` because in-domain real Gold data, in-domain real-data model selection, teacher agreement, and calibration/fairness evidence are still missing. The deterministic adversarial Pilot gate passes.

The selected local baseline is Qwen3 4B Q4_K_M through portable llama.cpp. Model and runtime artifacts stay in ignored directories under this D-drive lab. Qwen3 8B was rejected on this 14GB-RAM computer after completing only one of four fixed-set samples.

Zenodo JorGPT record 18981627 is available as a governed, evaluation-only external benchmark. Its 3,041 real anonymized answers are out of domain, single-teacher, and machine-Rubric-derived, so they do not satisfy Pilot Gold or training gates.

No further Story is created automatically. Main-platform shadow integration requires separate explicit authorization.

## Safety Rules

- No private or directly identifiable student data. Public real-answer sources require approved license/use, integrity lock, field minimization, privacy scan, and explicit evidence scope.
- Every synthetic record must use `synthetic=true`.
- Mock adapter outputs must use `mock=true` and include `MOCK_OUTPUT`.
- AI scoring outputs must pass schema validation.
- Suggested scores must stay within `[0, max_score]`.
- Essay and discussion answers default to human review.
- OCR confidence below `0.85` forces human review.
- Matched rubric points require answer-local evidence.
- Prompt, model, and rubric versions are mandatory.
