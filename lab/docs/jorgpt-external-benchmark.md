# JorGPT External Real-Answer Benchmark

Status: Governed external evaluation benchmark; not Pilot Gold and not training-approved.

## Frozen Source

The source is Zenodo record `18981627`, DOI `10.5281/zenodo.18981627`, published 2026-03-12 by Jorge Cisneros-González and Javier Sánchez-Soriano. Zenodo identifies the record as open access under `CC BY 4.0`. The English file is frozen at 14,558,120 bytes, MD5 `196825ce723a632b1b117efc187b0147`, and SHA256 `1d5deb07247a415181f1999e66763c29a2a4df9130d0ad6e9bc813f64343653c`.

The Kaggle mirror reports a different license label and is not the source used here. Download, attribution, integrity, and field contracts are pinned in `config/dataset-source-locks/jorgpt-zenodo-18981627.json`.

## Scope

The dataset contains 3,041 authentic, anonymized student answers from 79 students to 50 university computer-science questions. The imported English version provides a validated translation, an instructor ideal answer, one corrected holistic teacher grade from 0 to 10, and teacher feedback.

It is out of the local Pilot domain, exposes no two independent teacher labels, and does not publish the original point-level teacher Rubric. The lab reconstructs a one-point holistic machine Rubric from each ideal answer solely to exercise the grading protocol. It cannot satisfy Pilot Gold, teacher agreement, in-domain model selection, calibration, fairness, or training gates.

## Data Minimization

Raw data stays in Git-ignored `.downloads` on D drive. Import removes the source participant ID, all DeepSeek/Qwen/Gemini/judge outputs, timestamps, and source length fields. A project hash of the source participant ID is used only in memory to audit participant overlap and is not persisted. Three structured example identity values were replaced with `[redacted]`; all 3,041 persisted records passed the lab privacy scanner and grading input schema.

## Governed Split

The deterministic question-plus-Rubric split contains 34 train question groups/2,133 records, 8 validation groups/456 records, and 8 test groups/452 records. Question/Rubric leakage is zero. All 79 participants occur in more than one question partition, so this split measures held-out-question behavior but not held-out-participant behavior.

The governed test SHA256 is `c98c75eb49e13d15da789890d8364794c2c3524e86734210c6301891aa80f441`. The tracked manifest points to ignored data files and records their individual hashes.

## Baselines

The full 452-record deterministic alias baseline completed without runtime failures but is unusable for semantic grading: MAE `4.383/10`, exact agreement `8.4%`, adjacent agreement `13.5%`, score bias `-4.383`, and high-score recall `0%`.

A three-record Qwen3 4B smoke run sampled three distinct questions and is explicitly marked `complete_dataset=false`. It completed 3/3 with MAE `1.333/10`, exact agreement `33.3%`, first-pass rate `66.7%`, evidence compliance `66.7%`, p50 latency `80.6s`, and p95 latency `189.9s`. At the measured average, a serial 452-record run would take about 14.4 hours.

Sample `jorgpt-en-0007` reproducibly scored 6 versus teacher 7 but failed because a cited excerpt was not in the answer. The first run required repair after invalid JSON; the diagnostic rerun required repair after timeout and took 203.3 seconds. It remains an open external regression and routes to human review.

## Commands

```powershell
npm.cmd run dataset:jorgpt:prepare
npm.cmd run dataset:jorgpt:import
npm.cmd run dataset:jorgpt:govern
npm.cmd run benchmark:jorgpt:rules
```

The full 4B benchmark is not scheduled on the current CPU-only machine until evidence compliance improves on a broader smoke set or stronger hardware is available.
