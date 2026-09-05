# Grading Agent Service

Internal, authenticated, shadow-only subjective grading service. Subjective grading supports the local llama.cpp runtime and DashScope's native generation API. Paper parsing additionally supports a school's platform-managed OpenAI-compatible API or DashScope native API; this does not change the governed subjective grading policy.

The service never publishes grades and never receives tenant or student identity. It validates every model response, verifies answer-local evidence, recomputes scores from Rubric points, forces teacher review, and emits privacy-safe request telemetry.

## Local Tests

```powershell
$env:PYTHONPATH = "ai-services"
python -m unittest discover -s ai-services/tests -p "test_*.py"
```

## Endpoints

- `GET /health`: service liveness.
- `GET /ready`: llama.cpp model-runtime readiness.
- `GET /grading/prompts/current`: authenticated snapshot of the prompt files actually loaded by the runtime.
- `POST /grading/grade`: authenticated internal suggestion endpoint. `Authorization: Bearer ...` and `Idempotency-Key` are required.
- `POST /paper/parse`: authenticated internal paper parsing. The gateway resolves the school's default API and passes a request-scoped `managed_model` configuration on the internal service connection. Credentials are removed before constructing document prompts and are never returned in parsing results. Without a school default, the configured local parser is used. External connections require public HTTPS addresses, preserve TLS hostname verification, and do not follow redirects.

Runtime settings are documented in `.env.example`. The service refuses to start without a service token of at least 32 characters.

For DashScope native mode, set the adapter to `dashscope_native`, the base URL to exactly `https://dashscope.aliyuncs.com/api/v1`, and provide the API key through the existing secret/environment injection. The prompt inspector is read-only by design: a prompt change must create a new manifest version and pass evaluation and approval before deployment.
