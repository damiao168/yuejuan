# OCR Sample Set

## Purpose

This sample set defines the project-specific evidence required before an OCR engine can be used in an EduGrade pilot. Public OCR leaderboards are useful for candidate discovery, but acceptance must be based on authorized or desensitized exam materials that match this product's real scanning and grading workflow.

## Storage Rules

- Store source files outside the repository unless they are fully synthetic.
- Keep a manifest under `reports/ocr-evaluation/<run-id>/manifest.json`.
- Every sample must include `tenant_scope`, `exam_type`, `subject`, `page_count`, `source_hash`, `ground_truth_hash`, `license_or_authorization`, and `desensitization_status`.
- Do not include student names, candidate numbers, school identifiers, full answer text, or credentials in logs.
- If a sample cannot be fully desensitized, it may only be used in an authorized private environment.

## Required Categories

| Category | Minimum | Notes |
| --- | ---: | --- |
| Printed Chinese text | 20 pages | Common body text, question stems, option labels. |
| Handwritten Chinese short answers | 30 pages | Include different handwriting density and stroke quality. |
| Numbers and English | 20 pages | Student-entered numbers, units, mixed Chinese-English terms. |
| Math formulas | 20 regions | Fractions, superscripts, equations, geometry notation. |
| Tables | 15 pages | Ruled and unruled tables, merged cells, score tables. |
| Corrections and annotations | 10 pages | Cross-outs, inserted text, teacher marks excluded from ground truth when appropriate. |
| Low-quality scans | 20 pages | Blur, low contrast, compression artifacts, shadows. |
| Skewed or rotated pages | 15 pages | Mild and severe skew. |
| Double-sided or multi-page submissions | 10 submissions | Validate page order and per-page input hashing. |
| Blank or near-blank pages | 10 pages | Must not create normal OCR results silently. |

## Ground Truth Format

Use JSONL, one page or region per line:

```json
{"sample_id":"ocr_001_p1","page_no":1,"regions":[{"id":"r1","text":"F = ma","bbox":[120,340,680,410],"type":"formula"}]}
```

`bbox` uses `[x, y, width, height]` in image pixel coordinates. If a region is intentionally unreadable, mark it with `"expected_review": true` and explain the reason.

## Candidate Engines

The first production candidate is PaddleOCR through `services/ocr-worker`. Surya and docTR may be evaluated as comparison engines. Tesseract is a fallback candidate for simple printed text or low-resource deployments, not the default for handwritten Chinese and complex layout.

## Acceptance Workflow

1. Build or select an authorized sample bundle.
2. Run the OCR worker in an isolated environment using the same Docker image and environment variables intended for pilot use.
3. Save raw structured OCR output, metrics, runtime metadata, and failure codes under `reports/ocr-evaluation/<run-id>/`.
4. Review per-category failures with a human reviewer.
5. Promote only the categories that pass the configured gates; categories that fail remain routed to human review.
