# Appeals API

STORY-021 implements the backend appeal workflow for published and locked grades.

This module does not implement a Web appeal center, notifications, multi-level approval, or image-level answer rendering. Evidence fields expose only data already stored by the grading pipeline.

## Permissions

- `appeal:create`: create a student appeal.
- `appeal:read`: list and view appeals.
- `appeal:manage`: review, adjust score, close appeals, and view statistics.

Students must have `auth.User.DataScope.student_id`; the backend enforces that non-manage users can only create and read appeals for that student id. Teachers, arbitrators, and admins may manage appeals according to RBAC. Auditors receive read-only appeal access in the default migration.

## Statuses

```text
submitted
under_review
need_more_info
accepted
rejected
score_adjusted
closed
```

Review transitions can move active appeals from `submitted`, `under_review`, or `need_more_info` to a review result. Terminal review results cannot be reopened by another review call; use the close endpoint to close the appeal.

## Create Appeal

`POST /api/v1/appeals`

```json
{
  "exam_id": "uuid",
  "student_id": "uuid",
  "target_type": "question",
  "final_grade_id": "uuid",
  "deduction_point_id": "",
  "reason": "Q3 deduction point was applied incorrectly",
  "attachment": {
    "file_id": "uuid"
  }
}
```

`target_type` supports `exam`, `question`, and `deduction_point`.

- `question` and `deduction_point` require `final_grade_id`.
- `deduction_point` requires `deduction_point_id`.
- The target grade must have `submission_grade.status=published` and `submission_grade.locked=true`.

Response:

```json
{
  "appeal": {
    "id": "uuid",
    "status": "submitted"
  }
}
```

## List Appeals

`GET /api/v1/appeals?exam_id={id}&student_id={id}&status={status}`

Students are always scoped to their own `student_id`. Manage users may filter by exam, student, and status.

## Get Appeal

`GET /api/v1/appeals/{id}`

The detail response includes current appeal data, optional score adjustments, and an evidence snapshot:

```json
{
  "appeal": {
    "id": "uuid",
    "status": "score_adjusted",
    "evidence": {
      "raw_answer": "stored answer text",
      "ocr_text": "stored OCR text",
      "ai_grades": [],
      "human_grades": [],
      "rubric": {},
      "final_grade": {}
    },
    "adjustments": []
  }
}
```

## Review Appeal

`POST /api/v1/appeals/{id}/review`

```json
{
  "status": "score_adjusted",
  "reason": "Rubric evidence supports full credit",
  "assigned_to": "uuid",
  "final_grade_id": "uuid",
  "adjusted_score": 5
}
```

Supported review statuses:

- `under_review`
- `need_more_info`
- `accepted`
- `rejected`
- `score_adjusted`

`score_adjusted` requires `adjusted_score`. The backend validates the score is between `0` and the question max score, creates `score_adjustment`, updates `final_grade.score`, keeps the final grade locked, and adjusts the locked published `submission_grade.total_score`.

## Close Appeal

`POST /api/v1/appeals/{id}/close`

```json
{
  "reason": "Appeal result acknowledged and closed"
}
```

## Statistics

`GET /api/v1/appeals/statistics?exam_id={id}`

```json
{
  "statistics": {
    "total": 12,
    "by_status": {
      "submitted": 3,
      "score_adjusted": 2
    },
    "score_adjusted_count": 2,
    "average_handle_hours": 4.5
  }
}
```

## Audit

The handler writes audit events for:

- `appeal.created`
- `appeal.reviewed`
- `appeal.score_adjusted`
- `appeal.closed`
