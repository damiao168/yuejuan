# Empirical Confidence Calibration And Operational Fairness

Status: Story N accepted for calibration mechanics; runtime confidence remains disabled.

## Confidence Meaning

Confidence is the empirical probability that a suggestion has valid schema/evidence and falls within 10% of max score from Gold. The model's self-reported confidence is not used. A deterministic reliability signal combines schema, evidence, first-pass completion, OCR confidence, capability scope, and rule consistency; isotonic regression maps that signal to observed agreement probability.

Calibration fitting and evaluation use separate partitions. Reports include ECE, Brier score, calibration bins, and coverage-versus-risk. Synthetic calibration artifacts are marked `promotable_to_runtime=false`, so the local adapter continues to return confidence `0`.

## Fairness Scope

Current slices are subject, question type, OCR band, and answer-length band. Each reports count, normalized MAE, acceptable agreement, overgrade/undergrade rate, and mean calibrated probability. Protected demographic attributes are not collected without a separately approved lawful and ethical protocol.

## Gates

Development requires 8 evaluation observations, ECE at most 0.35, Brier at most 0.35, slice NMAE gap at most 0.60, and at least 2 records per slice. Pilot requires 500 real evaluations, ECE at most 0.08, Brier at most 0.12, NMAE gap at most 0.03, and 50 records per slice.

## Acceptance

Synthetic mechanics must pass dev, fail pilot sample and slice requirements, prove monotonic calibration, and keep runtime promotion disabled.

## Execution Result

Eight calibration and eight held-out evaluation observations produced ECE `0.125` and Brier score `0.125`. At probability threshold 0.5, coverage was `0.625` and observed risk was `0.20`. Maximum normalized-MAE gaps were `0.50` for subject, question type, and OCR band, and `0.25` for answer length.

The development mechanics gate passed. Pilot fails because 8 is below 500 evaluations, Brier exceeds 0.12, NMAE gaps exceed 0.03, and every slice is below 50 records. The report is `promotable_to_runtime=false`; local model outputs remain confidence `0`.
