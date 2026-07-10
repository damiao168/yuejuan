# Reports API

STORY-022 implements backend learning reports and exam quality analytics.

Reports only use data already stored in EduGrade tables. The API does not generate new AI recommendations, render charts, or export PDF/XLSX files in this Story.

## Permissions

- `report:read`: read management reports.
- `report:export`: export report CSV.
- `student:report:read`: students read their own learning report.

Student report access is additionally scoped by `auth.User.DataScope.student_id` unless the user has `report:read`.

## Empty States

When there are no published locked grades, report endpoints return an explicit empty state instead of synthetic statistics:

```json
{
  "overview": {
    "exam_id": "exam-id",
    "empty": {
      "empty": true,
      "reason": "no_published_grades"
    }
  }
}
```

## Exam Overview

`GET /api/v1/exams/{examId}/reports/overview`

Returns published student count, score statistics, class comparisons, and question score rates.

## Class Reports

`GET /api/v1/exams/{examId}/reports/classes`

Returns class-level averages, highest/lowest score, median, pass rate, excellent rate, score distribution, frequent wrong questions, and weak knowledge points.

## Question Analysis

`GET /api/v1/exams/{examId}/reports/questions`

Returns average score, score rate, correct rate, difficulty, discrimination, knowledge points, option distribution when real answer payloads exist, and frequent error clues from stored AI/human evidence.

## Grading Quality

`GET /api/v1/exams/{examId}/reports/grading-quality`

Returns AI adoption rate, human modification rate, double-mark score differences, arbitration count, OCR failure rate, and low-confidence review counts. Metrics that lack source data return `available=false`.

## Student Report

`GET /api/v1/students/{studentId}/reports/{examId}`

Returns total score, question scores, knowledge mastery, teacher feedback, AI feedback, and error clues for the student's published locked grade.

## Export

`POST /api/v1/exams/{examId}/reports/export`

Returns a CSV export with an `X-EduGrade-Watermark` header and records a `report` row.

## Audit

The handler writes:

- `report.generated` for report generation/read actions.
- `report.exported` for CSV export.
