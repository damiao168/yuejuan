# Lab-to-main-project integration

The grading-agent Lab is now connected to the main repository as an offline
quality boundary. The connection is intentionally one-way: Lab evaluation results
can block a verification run, but Lab code cannot publish grades or bypass the Go
API gateway's review and finalization workflow.

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

This integration does not implement STORY-057, STORY-058, or STORY-059. Those
stories remain paused; no grading-agent service, browser-to-model call, or real
model rollout is enabled by this gate.
