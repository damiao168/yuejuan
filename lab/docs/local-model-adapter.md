# Local Model Adapter

Status: Story I accepted after real-server smoke and full regression review.

## Contract

The adapter calls a loopback-only `llama-server` OpenAI-compatible endpoint. The server uses a random API key stored only under ignored `lab/.runtime`, limits CORS to localhost, disables cross-origin credentials, and exposes no Web UI. Student answers are untrusted prompt data. The request disables thinking, uses temperature zero, applies a dynamic JSON Schema with rubric point enums, and allows one retry after invalid output.

The model must classify every rubric point exactly once and link every matched point to evidence. Adapter code rejects unknown or duplicate rubric points, point overscoring, incomplete classifications, and broken evidence links. It ignores the model total and recomputes `suggested_score` from validated point scores. Local-model suggestions always require teacher review.

Model self-confidence is discarded until Story N supplies empirical calibration. The adapter emits confidence `0`, `score_needs_review`, and a code-owned teacher note that cannot contradict mandatory review. Privacy-safe run metadata records attempts, repair use, error codes, and elapsed time without raw answer or response text.

## Failure Policy

Timeout, HTTP failure, malformed JSON, invalid schema, or exhausted retries throws a typed `LocalModelError`. Error messages do not include the answer or raw model response. No failed result can be treated as a valid suggestion.

## Runtime Commands

```powershell
npm.cmd run server:local:start
npm.cmd run smoke:local
npm.cmd run server:local:stop
```

## Acceptance

Unit tests must cover structured request settings, score recomputation, retry, rubric partition, and point bounds. Acceptance additionally requires a real 4B server call whose output passes schema and evidence verification.

## Measured Smoke Result

The synthetic Chinese short-answer smoke completed in one attempt in 85,055 ms. It produced a non-mock 2/2 suggestion, two valid rubric-point evidence excerpts, schema validity `true`, and evidence validity `1.0`. Code replaced uncalibrated confidence with `0` and forced both `SCORE_NEEDS_REVIEW` and `HUMAN_REVIEW_REQUIRED`.

The smoke also exposed a non-blocking quality failure: student feedback was English for a Chinese question despite the language instruction. The score and evidence contract passed, but language adherence must enter the Story O prompt regression set before pilot release.
