# Dataset Governance And Grouped Split

Status: Story J accepted after synthetic and external-real governed-dataset execution and leakage review.

## Policy

No dataset can enter development, evaluation, regression, or training unless its source is registered, its license status is approved for that use, direct identifiers are absent, and real records are explicitly anonymized. Public dataset names in the registry are research candidates, not permission to download or train on them.

## Atomic Split Unit

The split key is `question_id + rubric_version`. Real samples require an explicit `question_id`. Synthetic legacy samples may derive a stable question fingerprint from exact question text. All answers in one group remain together in train, validation, or test.

## Privacy Gate

The scanner rejects email, Chinese mobile number, Chinese ID number, labeled student number, labeled name, and labeled phone patterns across question, answer, rationale, and Rubric text. It is a minimum automatic gate, not a substitute for human privacy review.

## Manifest

Every governed dataset records source and intended use, record count, group count, deterministic split seed, split group counts, output file SHA256 values, privacy result, and leakage audit. Unknown or unapproved sources fail closed.

## Command

```powershell
npm.cmd run dataset:govern -- --input evals/synthetic/samples.jsonl --source lab_synthetic --out evals/governed/synthetic
```

## Acceptance

The current synthetic dataset must pass privacy and source gates, produce deterministic train/validation/test files, and report zero question/Rubric group leakage.

## Execution Result

The 22-record synthetic dataset produced 12 atomic question/Rubric groups: 8 train groups with 14 records, 2 validation groups with 6 records, and 2 test groups with 2 records. Privacy scanning passed, leakage audit found zero leaked groups, and all three files have recorded SHA256 values in `evals/governed/synthetic/manifest.json`.

This small split proves mechanics only. Two test records are not evidence of model quality or pilot readiness.

## External Real Benchmark

JorGPT Zenodo record `18981627` is approved only for external evaluation under the authoritative record's `CC BY 4.0` metadata. Its English file is integrity-locked, minimized, privacy-scanned, converted into the grading protocol, and split without question/Rubric leakage. The 3,041 records produce 34/8/8 question groups and 2,133/456/452 records. Raw and split records remain in ignored `.downloads`; the import audit and governed manifest retain hashes and attribution.

This source has evidence scope `external_real_benchmark`. It is English university computer science, uses one holistic teacher label, reconstructs the machine Rubric from an ideal answer, and has participant overlap across question partitions. It cannot satisfy `pilot_in_domain_gold` or training approval.
