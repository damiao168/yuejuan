# Grading Agent Service

Internal, authenticated, shadow-only subjective grading service. It promotes the governed contract and local llama.cpp adapter from `lab` without importing the lab runtime.

The service never publishes grades and never receives tenant or student identity. It validates every model response, verifies answer-local evidence, recomputes scores from Rubric points, forces teacher review, and emits privacy-safe request telemetry.

## Local Tests

```powershell
$env:PYTHONPATH = "ai-services"
python -m unittest discover -s ai-services/tests -p "test_*.py"
```

## Endpoints

- `GET /health`: service liveness.
- `GET /ready`: llama.cpp model-runtime readiness.
- `POST /grading/grade`: authenticated internal suggestion endpoint. `Authorization: Bearer ...` and `Idempotency-Key` are required.

Runtime settings are documented in `.env.example`. The service refuses to start without a service token of at least 32 characters.
