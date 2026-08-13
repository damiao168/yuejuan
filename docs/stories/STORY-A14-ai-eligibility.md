# STORY-A14 AI Eligibility Engine

`internal/aieligibility` is the mandatory server-side admission gate before an
external subjective-AI adapter can be called. It uses a frozen A01 assessment
snapshot and records one immutable decision per opaque run item.

The ordered checks are: risk/archetype hard prohibition, OCR/parser quality,
Rubric completeness, required material evidence, approved evaluation evidence,
calibration availability, then the policy's allowed scoring modes. Missing
evidence is conservative: the result is human primary and no external model is
called.

The narrow connection point is `aieligibility.Gate.Decide(ctx, tenantID,
DecisionInput)`. The subjective chain calls it after its run item exists but
before `adapter.Grade`; it must only call the adapter when
`decision.CanCallExternalAI()` is true. The caller supplies quality and the
approved evaluation/calibration facts. A15/A16 adapters provide those facts
without coupling their stores into the grading handler; A26 provides
segment-specific parser quality, and missing evidence remains conservative.

An allowed AI result is constrained to structured criterion/evidence output.
`allow_model_final_score` is always false. The final score authority is either
the server's frozen Rubric/deterministic rule or a human confirmation flow.
