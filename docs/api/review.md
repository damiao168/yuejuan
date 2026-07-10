# Human Review API

当前文档属于 `STORY-018 人工复核与阅卷工作流`。

## 边界

本模块实现后端人工复核任务和人工评分记录：

- 创建 review_task。
- 查询 review_task 列表和详情。
- 分配阅卷员。
- 批量分配阅卷员。
- 阅卷员提交 human_grade。
- 退回重评。

本模块不实现 Web 阅卷工作台、成绩确认或成绩发布。双评分差比较、仲裁任务和 `final_grade` 写入属于 `STORY-019`，见 `docs/api/arbitration.md`。

## 权限

所有接口必须携带 Bearer token，并要求 `review:manage` 权限。服务端按当前登录用户的 `tenant_id` 隔离数据。

阅卷员只能提交分配给自己的任务。查询结果包含 `anonymous_code`，默认不返回学生姓名。

## POST /api/v1/review-tasks

创建人工复核任务。

请求：

```json
{
  "answer_segment_id": "segment-id",
  "source": "evidence_verification_failed",
  "priority": 3,
  "assigned_to": "reviewer-user-id"
}
```

支持的 `source`：

- `ai_low_confidence`
- `ocr_low_confidence`
- `subjective_default_review`
- `evidence_verification_failed`
- `double_mark_required`
- `score_anomaly`
- `manual_sample`

响应：`201 Created`

```json
{
  "task": {
    "id": "review-task-id",
    "answer_segment_id": "segment-id",
    "anonymous_code": "ANON-001",
    "source": "evidence_verification_failed",
    "status": "pending",
    "priority": 3
  }
}
```

## GET /api/v1/review-tasks

查询复核任务列表。

支持查询参数：

- `status`
- `assigned_to`

响应：`200 OK`

```json
{
  "tasks": []
}
```

## GET /api/v1/review-tasks/{id}

查询复核任务详情。

响应：`200 OK`

```json
{
  "task": {
    "id": "review-task-id",
    "anonymous_code": "ANON-001",
    "status": "assigned"
  }
}
```

## POST /api/v1/review-tasks/{id}/assign

分配单个复核任务。

请求：

```json
{
  "assigned_to": "reviewer-user-id"
}
```

响应：`200 OK`

## POST /api/v1/review-tasks/batch-assign

批量分配复核任务。服务端按事务方式处理；任一任务不存在或状态不允许分配时整体失败。

请求：

```json
{
  "task_ids": ["task-a", "task-b"],
  "assigned_to": "reviewer-user-id"
}
```

响应：`200 OK`

```json
{
  "tasks": []
}
```

## POST /api/v1/review-tasks/{id}/submit

提交人工评分。只有分配给当前登录用户的任务可以提交。

请求：

```json
{
  "score": 4,
  "rubric_selections": [
    {
      "point_id": "p1",
      "score": 4
    }
  ],
  "comments": "clear answer",
  "private_note": "teacher-only note",
  "student_feedback": "Good work.",
  "reason": "manual review completed"
}
```

校验：

- `score` 必须在 0 到题目满分之间。
- `rubric_selections[].point_id` 必须属于题目最新 Rubric points。
- 任务必须处于 `assigned`、`in_progress` 或 `returned`。

响应：`201 Created`

```json
{
  "task": {
    "status": "submitted"
  },
  "human_grade": {
    "score": 4,
    "max_score": 5,
    "grade_round": "single"
  }
}
```

## POST /api/v1/review-tasks/{id}/return

退回重评。

请求：

```json
{
  "reason": "needs second look"
}
```

响应：`200 OK`

## 审计

以下动作写入 `audit_log`：

- `review.task_created`
- `review.task_assigned`
- `review.tasks_batch_assigned`
- `review.human_grade_submitted`
- `review.task_returned`
