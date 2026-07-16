# STORY-058: Governed Grading Agent Service

Status: accepted after implementation review on 2026-07-15.

## Goal

Replace the Docker AI placeholder with an internal grading-agent service that can call the frozen local llama.cpp candidate while preserving the platform's human-review and final-grade boundaries.

## Plan Review

The service remains in root `ai-services` because that is the existing deployment boundary and port allocation. Creating a second overlapping AI directory would add operational ambiguity. Lab Node modules are not imported; prompts, invariants, and capability data are promoted into production-owned files.

The service uses the Python standard library to keep the offline private-deployment image deterministic. The HTTP boundary is intentionally small and internal. Contract validation, model output validation, evidence checks, and score recomputation are explicit application code and unit tested.

## Implemented Controls

- Bearer service authentication with constant-time comparison.
- Required request/body idempotency identity and bounded in-memory replay cache.
- Strict request field allowlist and forbidden identity/final-grade fields.
- Capability routing from the promoted `local-pilot-v1` matrix.
- One local llama.cpp inference at a time with bounded queue wait.
- OpenAI-compatible chat-completions call with strict dynamic JSON Schema.
- At most one repair attempt and versioned model/prompt configuration.
- Complete Rubric classification, point score bounds, exact evidence links, answer-local excerpt verification, and code-recomputed total.
- Local prompt-injection reinspection even when the gateway reports no risk.
- Forced zero confidence while calibration evidence is unavailable.
- Forced human review and shadow-only delivery for essay/discussion.
- Logs contain request id, status, attempt count, latency, and error code only.
- Non-root container user and fail-fast service-token configuration.
- Hash-locked prompt manifest bound to the configured prompt version.
- Concurrent identical idempotency keys share one inference; distinct requests remain subject to the bounded model queue.

## Non-Goals

- No automatic final-grade write or publication.
- No student-facing internal endpoint.
- No persistent model-output cache.
- No claim that the current model passed real-data quality gates.
- No model binary or GGUF artifact inside the container image.

## Verification

```powershell
$env:PYTHONPATH = "ai-services"
python -m unittest discover -s ai-services/tests -p "test_*.py"
docker build -f ai-services/Dockerfile -t edugrade/grading-agent:story058 .
```

## Implementation Review

The first implementation held the idempotency cache lock for the full inference duration. That preserved duplicate suppression but made unrelated requests wait outside the configured model queue. It was replaced with per-key in-flight coordination: identical requests share the result, conflicting payloads fail, and different keys reach the single-concurrency model semaphore with a bounded wait.

The first implementation also bound prompt version only through configuration. A content-hash manifest now makes a prompt edit fail startup unless the versioned manifest is intentionally updated.
