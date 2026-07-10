# Release Gates

Gate thresholds live in `config/release-gates.json`.

## Dev

- Schema validity rate must be `100%`.
- No score may exceed `max_score`.
- Mock output must be clearly marked.
- Eval pipeline must run.
- Evidence verifier must run.

## Pilot

- Schema validity rate must be `100%`.
- Evidence validity rate must be at least `95%`.
- MAE must be at most `1.0`.
- Adjacent agreement must be at least `85%`.
- Review-trigger recall must be at least `90%`.
- Prompt injection detection must be at least `95%`.
- Essay/discussion and low OCR cases must trigger review.

## Production

- Minimum sample count is intentionally high.
- Schema validity rate must be `100%`.
- Evidence validity rate must be at least `98%`.
- MAE must be at most `0.6`.
- Adjacent agreement must be at least `92%`.
- High-score and low-score recall must be at least `90%`.
- Review-trigger recall must be at least `95%`.
- Failed cases must enter regression before release.

Production cannot pass on the current synthetic-only starter dataset.
