# Orchestrator API

当前文档属于 `STORY-014 多智能体 Orchestrator`。

## 边界

本模块实现受控 Agent 工作流编排控制面：创建 orchestration run、创建 agent task、记录输入/输出/证据引用、状态流转、失败重试和人工复核触发。

本模块不会执行真实 Agent worker，不会调用模型，不会生成 OCR、评分或证据判断结果。真实 worker runtime、模型推理、Prompt 管理和评分逻辑属于后续 Story。

## 权限

所有接口必须携带 Bearer token，并要求 `orchestrator:manage` 权限。服务端按当前登录用户的 `tenant_id` 隔离数据。

## Run 状态

```text
created
running
completed
failed
requires_human_review
```

创建 run 后为 `created`。创建 task 后 run 进入 `running`。task 失败会使 run 进入 `failed`，retry 后回到 `running`。低置信度或明确人工复核标记会使 run 进入 `requires_human_review`。所有 task 成功后 run 进入 `completed`。

## Task 状态

```text
queued -> running -> succeeded
queued -> running -> requires_human_review
queued -> failed
running -> failed
failed -> queued
```

`failed -> queued` 只允许在 `attempt_no < max_attempts` 时执行。

## POST /api/v1/orchestrations

创建编排 run。

请求：

```json
{
  "workflow_type": "grading_pipeline",
  "target_type": "answer_segment",
  "target_id": "00000000-0000-0000-0000-000000000701"
}
```

受控 `workflow_type`：

- `ocr_pipeline`
- `segmentation_pipeline`
- `grading_pipeline`
- `review_pipeline`
- `custom`

受控 `target_type`：

- `exam`
- `submission`
- `answer_segment`
- `ocr_task`

响应：`201 Created`

```json
{
  "run": {
    "id": "run_001",
    "workflow_type": "grading_pipeline",
    "target_type": "answer_segment",
    "target_id": "00000000-0000-0000-0000-000000000701",
    "status": "created"
  }
}
```

## GET /api/v1/orchestrations/{id}

查询 run 详情，包含当前 run 下的 tasks。

## GET /api/v1/orchestrations/{id}/tasks

列出 run 下的 agent tasks。

## POST /api/v1/orchestrations/{id}/tasks

创建 agent task。

请求：

```json
{
  "agent_type": "objective_grading_agent",
  "input_ref": {
    "answer_segment_id": "segment_001",
    "rubric_version_id": "rubric_v3"
  },
  "max_attempts": 3
}
```

`input_ref` 只保存引用，不保存学生答案长文本或大段 OCR 原文。受控 `agent_type` 包括：

- `ocr_agent`
- `layout_agent`
- `segmentation_agent`
- `objective_grading_agent`
- `fill_blank_agent`
- `subjective_grading_agent`
- `essay_grading_agent`
- `evidence_check_agent`
- `consistency_check_agent`
- `anomaly_detection_agent`
- `fairness_check_agent`
- `analytics_agent`
- `audit_agent`

## POST /api/v1/agent-tasks/{id}/start

启动 task。仅 `queued` task 可启动。

## POST /api/v1/agent-tasks/{id}/complete

由外部 worker 回写 task 结果引用。仅 `running` task 可完成。

请求：

```json
{
  "output_ref": {
    "grading_result_id": "grade_ref_001"
  },
  "evidence_ref": {
    "file_asset_id": "file_001",
    "bbox": [1, 2, 3, 4]
  },
  "confidence": 0.92,
  "requires_human_review": false
}
```

规则：

- `confidence` 范围必须是 0 到 1。
- `confidence < 0.8` 会把 task 状态置为 `requires_human_review`。
- `requires_human_review=true` 会把 task 状态置为 `requires_human_review`。
- `output_ref` 和 `evidence_ref` 保存引用和定位信息，不代表系统已经完成真实 AI 推理。

## POST /api/v1/agent-tasks/{id}/fail

标记 task 失败。

请求：

```json
{
  "error_message": "worker timeout"
}
```

## POST /api/v1/agent-tasks/{id}/retry

在重试次数未耗尽时，将失败 task 重新置为 `queued`，并递增 `attempt_no`。

重试耗尽返回 `409 agent_task_retry_exhausted`。

## 审计

以下动作写入 `audit_log`：

- `orchestrator.run_created`
- `orchestrator.task_created`
- `orchestrator.task_started`
- `orchestrator.task_completed`
- `orchestrator.task_failed`
- `orchestrator.task_retried`
