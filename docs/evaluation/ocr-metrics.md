# OCR Metrics

## Required Report

Each OCR evaluation run must produce `reports/ocr-evaluation/<run-id>/summary.json` and a human-readable markdown summary. The report must include engine name, engine version, model version, config hash, input bundle hash, preprocessing profile, hardware profile, average duration, P95 duration, failure rate, and review-trigger rate.

## Core Metrics

| Metric | Definition | Gate Type |
| --- | --- | --- |
| CER | Character error rate against ground truth text. | Lower is better; set by subject/category. |
| WER | Word or token error rate for English, numbers, and mixed text. | Lower is better. |
| Region hit rate | Percentage of expected regions with a matching OCR block. | Higher is better. |
| BBox IoU | Intersection over Union for matched regions. | Higher is better. |
| Page success rate | Pages with usable OCR output or correct review/failure routing. | Higher is better. |
| Blank-page false positive rate | Blank pages that produce normal OCR results. | Must be near zero. |
| Low-confidence recall | Risky samples routed to human review. | Prefer high recall over silent errors. |
| Failure rate | Worker/API/download/engine failures per processed task. | Must be explainable and bounded. |
| Average duration | Mean processing time per page and per submission. | Capacity planning input. |
| P95 duration | 95th percentile processing time. | Pilot throughput gate. |

## First Pilot Gates

These are initial guardrail gates, not marketing claims:

- Blank or unreadable pages must be failed or routed to review, not completed as normal text.
- Any category with unreliable OCR must remain behind human review.
- Low-confidence recall is more important than low review volume during the first pilot.
- The worker must record `model_version`, `config_hash`, `input_hash`, `duration_ms`, `worker_id`, and `preprocess_profile` for every completed task.
- No evaluation output may contain student identity fields unless the environment is explicitly authorized for identifiable data.

## Summary Schema

```json
{
  "run_id": "ocr-2026-07-08-paddleocr-v1",
  "engine": "paddleocr",
  "engine_version": "pp-ocrv5",
  "model_version": "ppocr-v5-server",
  "config_hash": "sha256:...",
  "input_bundle_hash": "sha256:...",
  "preprocess_profile": "default",
  "hardware": {"device": "cpu", "cpu": "example", "gpu": null},
  "metrics": {
    "cer": 0.0,
    "wer": 0.0,
    "region_hit_rate": 0.0,
    "bbox_iou": 0.0,
    "page_success_rate": 0.0,
    "blank_false_positive_rate": 0.0,
    "low_confidence_recall": 0.0,
    "failure_rate": 0.0,
    "average_duration_ms": 0,
    "p95_duration_ms": 0
  },
  "category_results": [],
  "review_required_categories": []
}
```
