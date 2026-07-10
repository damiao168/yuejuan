# Exam Management API

当前文档属于 `STORY-008 考试管理模块`。

## 权限

所有考试管理接口必须带 Bearer token，并要求 `exam:manage` 权限。

## 阅卷模式

```text
auto_objective_only
ai_assisted
human_review_required
double_mark
blind_double_mark
```

## 状态流转

```text
draft -> configured -> collecting -> grading -> reviewing -> finalized -> published -> archived
```

允许从任意未归档流程状态进入 `archived`，但不允许跳过中间状态直接发布。`published` 和 `archived` 状态不可修改核心配置。

## POST /api/v1/exams

创建考试。

请求：

```json
{
  "school_id": "school_001",
  "name": "高二物理期末考试",
  "subject": "physics",
  "exam_type": "formal_exam",
  "total_score": 100,
  "grading_mode": "ai_assisted",
  "appeal_enabled": true,
  "publish_policy": "after_admin_approval",
  "class_ids": ["class_1", "class_2"]
}
```

响应：`201 Created`

```json
{
  "exam": {
    "id": "exam_001",
    "status": "draft"
  }
}
```

## GET /api/v1/exams

查询考试列表。支持查询参数：

- `status`
- `school_id`

## GET /api/v1/exams/{id}

查询考试详情。

## PATCH /api/v1/exams/{id}

修改考试基础配置和适用班级。`published` 或 `archived` 状态返回 `409 exam_locked`。

## POST /api/v1/exams/{id}/status

执行受控状态流转。

请求：

```json
{
  "status": "configured"
}
```

非法流转返回 `409 invalid_status_transition`。

## POST /api/v1/exams/{id}/archive

归档考试，内部等价于状态流转到 `archived`。

## 审计

以下动作必须写入 `audit_log`：

- `exam.created`
- `exam.updated`
- `exam.status_changed`
- `exam.archived`
