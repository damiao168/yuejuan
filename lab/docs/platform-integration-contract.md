# Platform Integration Contract

This is a contract for a future platform integration. It does not change the main platform.

## Endpoints

- `POST /grading/grade`
- `POST /grading/verify-evidence`
- `POST /grading/evaluate`
- `GET /grading/models`
- `GET /grading/prompts`

## Request Rules

The platform sends `GradingInput` with `request_id` as idempotency key. The platform must pass `model_version`, `prompt_version`, and `rubric_version` where applicable.

## Response Rules

The service returns `GradingOutput` only after schema validation. `needs_human_review` and `risk_flags` must be persisted by the platform as review guidance, not as final grades.

## Error Codes

- `INVALID_INPUT`
- `INVALID_RUBRIC`
- `ADAPTER_NOT_CONFIGURED`
- `MODEL_TIMEOUT`
- `MODEL_PARSE_FAILED`
- `EVIDENCE_VERIFICATION_FAILED`
- `SCHEMA_VALIDATION_FAILED`

## Timeout And Retry

Use short request timeouts and retry only idempotent `request_id` calls. Never retry with modified answer text.

## Logging

Logs may include request id, versions, adapter name, status, and risk flags. Logs must not include full answer text, student names, student numbers, or final grades.

## Platform Persistence

Persist suggested score separately from teacher-confirmed score. If `needs_human_review=true`, create a review task and do not publish the AI score as final.
