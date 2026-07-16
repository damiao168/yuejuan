# STORY-059: Platform Grading Agent Integration

Status: accepted after implementation review on 2026-07-15.

## Goal

Connect the Go subjective-grading boundary to the governed grading-agent service without changing the browser API or allowing model output to become a final grade.

## Plan Review

The browser must not call the grading-agent directly. The API gateway already owns authentication, tenant isolation, context loading, grade persistence, review, finalization, and audit, so the new HTTP adapter remains behind `LLMGradingAdapter`.

Exam subject already exists. Grade level is derived from the exam's active class assignments and their `grade.level_no`; only a single distinct 7-9 value maps to `junior_middle`. Missing, mixed, senior, or otherwise unsupported context fails before a model request.

The existing caller-supplied model policy is not authoritative in governed mode. The adapter implements `GovernedPolicyProvider`, and the handler replaces request versions with server configuration.

## Implementation

- Added an authenticated HTTP adapter with bounded body reads, a 250-second default timeout, one network retry, and a stable idempotency key across retries.
- Added strict internal request/response structs with unknown response fields rejected.
- Removed tenant, student identity, final-grade data, and image references from the model-service payload.
- Added Go-side response validation for versions, capability profile, delivery mode, zero uncalibrated confidence, empty deductions, risk flags, telemetry, complete Rubric classification, score recomputation, evidence links, and answer-local excerpts.
- Added answer-version traceability and production metadata to `ai_grade` through migration `000044_grading_agent_integration.sql`.
- Failed grades discard unvalidated evidence and retain only sanitized error metadata.
- Changed AI dependency diagnostics from service liveness to model readiness.
- Preserved the explicit mock adapter when `EDUGRADE_AI_SERVICE_URL` is empty.

## Stored Governance Fields

- `answer_version`
- `model_version`, `prompt_version`, `rubric_version`
- `delivery_mode`, `capability_profile`
- `adapter_request_id`, `adapter_name`
- `adapter_attempts`, `adapter_latency_ms`, `adapter_repair_attempted`

`adapter_request_id` is unique per tenant for active rows, preventing duplicate persistence of the same internal service request.

## Implementation Review

The first response mapping accepted any confidence in the schema range and trusted service telemetry. The final implementation requires confidence zero for the uncalibrated local candidate, rejects deductions, enforces essay/discussion `shadow_only`, validates canonical risks, and constrains telemetry to `local_llama_cpp` with one or two attempts.

No frontend change was required. Existing pages continue reading `ai_grade`; the additional governance fields are additive.

## Verification

```powershell
cd services/api-gateway
go test ./internal/subjective ./internal/config ./internal/server
```

Coverage includes service authentication, payload privacy, governed-version override, retry idempotency, sanitized errors, missing grade context, contract mapping, and full router wiring.
