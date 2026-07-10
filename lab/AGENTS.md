# Grading Agent Lab Rules

These rules apply only inside `lab/`.

- Do not use real student private data.
- All synthetic data must explicitly set `synthetic=true`.
- Mock adapter output must explicitly set `mock=true` and include `MOCK_OUTPUT`.
- Grading output must pass schema validation before it is treated as a valid suggestion.
- `suggested_score` must be `>= 0` and `<= max_score`.
- Essay and discussion outputs must set `needs_human_review=true`.
- OCR confidence below `0.85` must set `needs_human_review=true`.
- A matched rubric point without evidence must fail evidence verification.
- Prompt, model, and rubric outputs must carry version fields.
- Normal logs must not include student names, student numbers, complete answer text, or final scores.
- Changes to schemas, prompts, evaluators, rubric parsing, adapters, or guardrails must add or update tests.
- Do not build UI, EXE packaging, grade publishing, appeals, or main-platform workflows here.

Recommended commands:

```bash
cd lab
npm test
npm run eval
npm run gate:dev
```
