# Lab-to-main-project integration

The Lab is connected to the main repository in two layers:

1. The offline Lab quality boundary validates capabilities, prompts, evidence,
   model candidates, datasets, calibration and release gates.
2. The governed `grading-agent` service promotes the Lab contract and local
   llama.cpp adapter behind the Go API Gateway. It accepts identity-free requests
   and returns teacher suggestions only.

The connection is intentionally one-way for authority: Lab evaluation results can
block a verification run, but neither Lab code nor the grading-agent can publish
grades or bypass the Go API gateway's review and finalization workflow.

Run the integrated check from the repository root:

```powershell
npm.cmd run check:lab-integration
```

The check runs:

- Lab unit tests (`112` tests at the current baseline)
- the synthetic evaluation with the explicitly marked mock adapter
- the Lab development release gate
- Go objective-grading tests used by STORY-056
- boundary checks for the capability matrix, evidence verifier, score bounds,
  mock marking, and Pilot readiness

The gate must keep these properties true:

- `final_grade_publication_allowed=false`
- out-of-scope routes fail closed to human grading
- essay and discussion remain `shadow_only`
- mock output is explicitly marked and cannot be mistaken for a real model
- Pilot readiness remains `NOT_READY` until governed in-domain data, teacher
  agreement, calibration/fairness, and real model-selection evidence exist

The service-level contract, Python grading-agent, Go HTTP adapter, persistence
metadata, Compose wiring and opt-in real-model adapter E2E are implemented in
STORY-057 through STORY-059. A Docker Compose end-to-end run is still an
environment-dependent verification step and requires a running Docker daemon.
