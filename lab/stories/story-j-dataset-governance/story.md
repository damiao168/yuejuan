# Story J Dataset Governance And Grouped Split

## Spec v1

Register dataset provenance and split answers into train, validation, and test without question leakage.

## Spec Review

Dataset names alone do not prove license permission, privacy scanning must cover Rubric and rationale text, and answer-level random split would inflate metrics. Existing synthetic samples also lack explicit question IDs.

## Spec v2

Require approved source/use pairs, reject direct identifiers, require anonymized real data and explicit real question IDs, allow deterministic synthetic question fingerprints, split by question plus Rubric version, and hash every output artifact.

## Implementation

Added the source registry, governance library, grouped splitter, privacy detectors, manifest CLI, tests, and policy documentation.

## Implementation Review

The source registry initially contained one HTTP candidate homepage and correctly failed its own HTTPS policy. After correction, the 22 samples formed 12 groups and produced a zero-leakage 8/2/2 group split. Public candidates remain unusable because their licenses are not approved.

Remediation reviewed a published real-answer source rather than treating a dataset name as permission. Zenodo JorGPT record 18981627 provided authoritative `CC BY 4.0` metadata and fixed file hashes. Full import first failed on seven broad name-label matches; review identified five technical prose false positives and two structured identity examples requiring redaction. A later privacy review removed the project participant hash from persisted records while retaining aggregate overlap evidence.

## Implementation Fixes

Corrected the registry URL, ran the full governed split, and recorded output hashes, privacy status, group counts, and leakage evidence in the manifest.

Added a version-locked JorGPT downloader, RFC 4180 importer, field minimization, structured identity redaction, external evidence scope, portable governed manifest, and fail-closed source/schema checks. All 3,041 records passed; question/Rubric leakage is zero, while 79/79 participants crossing question partitions is disclosed rather than hidden.

## Acceptance

Accepted. Synthetic mechanics and the external-real benchmark are reproducible and hash-locked. JorGPT is evaluation-only and explicitly cannot satisfy Pilot Gold or training approval.
