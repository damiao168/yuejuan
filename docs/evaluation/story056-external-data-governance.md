# STORY-056 External Data Governance

## Decision

The two local external datasets are approved only for isolated STORY-056 evaluation. They are not approved for model training, product demos, customer data seeding, CI fixtures, redistribution, or production threshold claims.

Raw files were confined to ignored `output/external-evaluation/` during evaluation and deleted after the final aggregate report was verified. Only aggregate metrics, provenance, integrity fingerprints, and limitations are committed.

## Inventory

| Dataset | Governed source | License | Local material | STORY-056 use |
| --- | --- | --- | --- | --- |
| SurveySet for OCR and OMR-Based Survey Digitization | [Zenodo 15610454](https://doi.org/10.5281/zenodo.15610454) | CC BY 4.0 | Evaluated, then deleted: `SurveySet.zip`; 906 valid JPEG crops plus 906 macOS `._*` metadata artifacts | Privacy/domain-gap review only; excluded from score metrics |
| Tamaulipas Multiple-Choice-Question Exam Image Dataset for Optical Mark Recognition Research, version 2 | [Mendeley Data](https://doi.org/10.17632/djmynjwjpy.2) | CC BY 4.0 | Evaluated, then deleted: 20 answer-sheet JPEGs and 20 human-observed label files from the `school001-grade10` path | External OMR safety and coverage evaluation |

### Integrity

- SurveySet upstream file: `SurveySet.zip`, 10,210,547 bytes, upstream MD5 `7de4dc1492de29acd2008d21c9fe8326`.
- SurveySet local ZIP SHA-256: `ce345f70a1d70aa3cdd7b97a20014b7648669521f2d2a2d3490d842c44c54d4d`.
- Tamaulipas local 40-file manifest SHA-256: `e2a2a66a8e499527d932e932cfbfd5d40b8df98e8b70ccc3f380121d2d8dab34`.
- The Tamaulipas manifest is reproducibly calculated by `tools/evaluate_external_omr.py` from sorted relative paths and each file's SHA-256.

## Provenance And Attribution

SurveySet was published on 2025-06-06 by Rubi Quinones and Cheekireddy Sreeja. Its Zenodo record describes real customer-experience survey crops containing handwriting, ticks, crosses, and partially filled bubbles, and requests citation of the SurveyNet paper.

Tamaulipas version 2 was published by Yahir Hernandez-Mier, Marco Aurelio Nuno-Maganda, Said Polanco-Martagon, Guadalupe Acosta-Villarreal, and Ruben Posada-Gomez. Its DataCite record states that the full dataset contains 5,721 anonymized high-school answer sheets from 42 schools in Tamaulipas, Mexico, administered in 2024, with 535,020 labeled items.

Any future publication using these results must retain the dataset titles, creators, DOI links, version, and CC BY 4.0 attribution. The evaluation report is not a substitute for the upstream citation instructions.

## Privacy Classification

### SurveySet: restricted sensitive demographic data

- Crops include or imply date/year of birth, gender, income, marital status, education, household information, and handwriting.
- These fields are sensitive or quasi-identifying in combination even when names are absent.
- The archive contains misplaced CSV labels and macOS metadata artifacts; file count alone is not a valid sample count.
- The dataset must not be copied into logs, screenshots, issue trackers, generated reports, or model prompts.

### Tamaulipas: restricted pseudonymous education data

- The publisher documents a geometric anonymization process, and the inspected name/date fields are blank.
- Answer-sheet IDs and the school/grade partition remain pseudonymous educational records. They are not anonymous enough for unrestricted product use.
- Reports must aggregate across sheets and must not include sheet IDs, labels by sheet, image excerpts, names, or original paths.

No attempt may be made to re-identify people, schools, or locations from either dataset.

## Purpose And Data Minimization

Permitted processing is limited to:

1. Validate that unsafe OMR outcomes route to review rather than silently becoming zero scores.
2. Measure automatic precision, automatic coverage, manual-routing rate, unsafe recall, and extraction throughput.
3. Assess whether a template-specific profile is eligible for approval.

The committed evaluator reads only the 20 governed Tamaulipas sheets, discards source IDs, and emits aggregate values. It uses labels only after extraction to calculate metrics. SurveySet is excluded from score metrics because its demographic crops do not provide the locked answer-sheet template and reliable item mapping required by STORY-056.

## Storage And Access Controls

- Raw files remained machine-local in the ignored `output/` tree during evaluation, were never staged or committed, and were deleted after the final report and fingerprints were verified.
- Evaluation runs offline after provenance retrieval. Raw content is not sent to external model APIs.
- CI and Docker images must not include `output/external-evaluation/`.
- Reproduction requires a fresh governed download, integrity verification, and the same isolated evaluator; raw files must be deleted again after use.
- A future expanded subset requires a new manifest fingerprint and a new review of source version, license, privacy, representativeness, and retention.

## Fitness And Approval Boundary

SurveySet is useful for domain-gap and privacy review but is not an exam-template acceptance set. Tamaulipas is relevant OMR evidence, but the local 20-sheet subset is only 0.350% of the upstream sheets and covers one school/grade path.

The external result therefore does not approve automatic confirmation. The evaluated template remains `manual_only`; production enablement still requires a locked blank reference, at least 100 governed calibration samples, per-option coverage, an explicit approval record, and working revocation. See [story056-external-omr-report.md](story056-external-omr-report.md).
