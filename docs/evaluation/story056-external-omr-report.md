# STORY-056 External OMR Evaluation

## Decision

**Not approved for automatic confirmation. Keep this template in `manual_only` mode.**

The `0.98` server safety gate removed all observed false automatic decisions, but it automatically confirmed only 4 of 1800 items. Safety behavior passed; practical automatic coverage did not.

## Governed Dataset

- Source: Tamaulipas Multiple-Choice-Question Exam Image Dataset for Optical Mark Recognition Research
- DOI: https://doi.org/10.17632/djmynjwjpy.2
- License: CC BY 4.0
- Local subset: 20 answer sheets / 1800 labeled items (0.350% of upstream sheets)
- Local manifest SHA-256: `e2a2a66a8e499527d932e932cfbfd5d40b8df98e8b70ccc3f380121d2d8dab34`
- Privacy: direct name fields are blank; answer-sheet IDs remain pseudonymous. Raw files were confined to ignored local output during evaluation and deleted after this aggregate report was verified.

## Method

- Detected the printed answer frame, normalized it to `1960x3340`, and evaluated 90 four-choice items per sheet.
- Built a median blank reference in two folds; no evaluated sheet contributes to its own reference.
- Used production `extract_marks` with `opencv-template-difference-bubble-v1` and the server minimum confidence `0.98`.
- Human labels were used only after extraction for scoring metrics; `X` and `M` are unsafe cases that must route to review.

## Results

| Metric | Result |
| --- | ---: |
| Raw unique selections | 413 |
| Raw clear-label correct / wrong selections | 412 / 0 |
| Raw unsafe-label selections | 1 |
| Raw selected precision | 99.758% |
| Raw clear-answer selected coverage | 23.198% |
| Unsafe review recall before server gate | 95.833% |
| Server-gated automatic confirmations | 4 |
| Server-gated correct / wrong | 4 / 0 |
| Server-gated automatic precision | 100.000% |
| Server-gated clear-answer coverage | 0.225% |
| Manual routing rate | 99.778% |
| Unsafe review recall | 100.000% |
| Silent unsafe-to-zero | 0 |
| Average / P95 extraction | 8.21 ms / 11.726 ms |

## Interpretation

- The conservative gate behaved correctly on this subset: no observed unsafe or incorrect item was auto-confirmed.
- Coverage is not operationally acceptable, so this is evidence for manual routing, not evidence to loosen thresholds.
- The source subset is narrow and the registration step is evaluation-only. A production template still requires its own locked blank reference, at least 100 governed calibration samples, per-option coverage, approval, and revocation controls.
- SurveySet was governed separately and excluded from score metrics because its demographic survey crops do not provide the locked answer-sheet template/label mapping required by STORY-056.

## Limitations

- The local subset contains 20 of 5,721 upstream answer sheets from one school/grade path.
- The evaluation uses a harness-only frame registration step; production still requires a locked template and approved calibration evidence.
- X and M are treated as unsafe human-observed labels, not as automatically scorable zero answers.
- Aggregate results do not authorize use of raw images for training or redistribution.
