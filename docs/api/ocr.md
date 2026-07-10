# OCR Task API

当前文档属于 `STORY-012 OCR 服务接口与任务队列`。

## 边界

本模块实现 OCR 任务、状态、队列抽象和外部 OCR worker 结果回写。系统不会在本 Story 中执行真实 OCR 推理，也不会生成模拟识别文本。

真实 OCR 引擎、图像预处理、版面解析、答题区域切分和人工校正 UI 属于后续 Story。

## 权限

所有接口必须携带 Bearer token，并要求 `ocr:manage` 权限。服务端按当前登录用户的 `tenant_id` 隔离数据。

## 状态

```text
queued -> processing -> completed
queued -> failed
processing -> failed
```

提交结果前必须先启动任务。低置信度结果不会阻止保存，但会把任务标记为 `requires_human_review=true`。

## POST /api/v1/submissions/{id}/ocr-tasks

为已准备好的 submission 创建 OCR 任务。submission 必须处于 `ready_for_ocr`。

请求：

```json
{
  "engine": "paddleocr",
  "engine_version": "pp-ocrv5",
  "min_confidence": 0.8
}
```

响应：`201 Created`

```json
{
  "task": {
    "id": "ocr_task_001",
    "submission_id": "submission_001",
    "status": "queued",
    "engine": "paddleocr",
    "engine_version": "pp-ocrv5",
    "min_confidence": 0.8,
    "result_count": 0,
    "requires_human_review": false
  }
}
```

未 ready 的 submission 返回 `409 submission_not_ready_for_ocr`。

## GET /api/v1/submissions/{id}/ocr-tasks

列出 submission 下的 OCR 任务。

## GET /api/v1/ocr-tasks/{id}

查询 OCR 任务详情，完成后包含 `results`。

## POST /api/v1/ocr-tasks/{id}/start

worker 或调度器启动任务。仅 `queued` 任务可启动。

## POST /api/v1/ocr-tasks/{id}/results

worker 回写结构化 OCR 结果。仅 `processing` 任务可提交结果。

请求：

```json
{
  "results": [
    {
      "submission_page_id": "page_001",
      "text": "F = ma",
      "bbox": [120, 340, 680, 410],
      "confidence": 0.87,
      "source_image_file_id": "file_001"
    }
  ]
}
```

规则：

- `submission_page_id` 必须属于当前 task 的 submission。
- `bbox` 必须是 4 个数字。
- `confidence` 范围为 0 到 1。
- 结果保存时补写任务的 `ocr_engine` 和 `ocr_version`。
- 任一结果低于任务 `min_confidence` 时，任务标记 `requires_human_review=true`。

## POST /api/v1/ocr-tasks/{id}/fail

标记 OCR 任务失败。

请求：

```json
{
  "error_message": "worker timeout"
}
```

## 审计

以下动作写入 `audit_log`：

- `ocr.task_created`
- `ocr.task_started`
- `ocr.task_completed`
- `ocr.task_failed`

## STORY-049 Real OCR Worker Updates

`STORY-049` adds a real worker-facing OCR loop. The API Gateway still owns authentication, authorization, tenant isolation, audit logging, task state, and result persistence. The Python worker only uses controlled HTTP APIs and does not write the database directly.

### GET /api/v1/ocr-tasks/pending?limit=5

Returns queued OCR tasks for the current tenant, ordered by creation time. The endpoint requires `ocr:manage`.

Response:

```json
{
  "tasks": [
    {
      "id": "ocr_task_001",
      "submission_id": "submission_001",
      "status": "queued",
      "engine": "paddleocr",
      "engine_version": "pp-ocrv5",
      "min_confidence": 0.8,
      "attempt_count": 0
    }
  ]
}
```

### GET /api/v1/ocr-tasks/{id}/input

Returns the minimum input needed by an OCR worker. The response intentionally excludes student identity fields such as candidate number and student name.

Response:

```json
{
  "task": {
    "id": "ocr_task_001",
    "submission_id": "submission_001",
    "status": "processing"
  },
  "pages": [
    {
      "id": "page_001",
      "file_asset_id": "file_001",
      "page_no": 1,
      "status": "uploaded",
      "download_url": "/api/v1/files/file_001/download"
    }
  ]
}
```

### POST /api/v1/ocr-tasks/{id}/results metadata

`bbox` is fixed as `[x, y, width, height]`. `x` and `y` must be non-negative; `width` and `height` must be positive.

Worker result requests may include:

```json
{
  "worker_id": "ocr-worker-1",
  "model_version": "ppocr-v5-server",
  "config_hash": "sha256:...",
  "input_hash": "sha256:...",
  "duration_ms": 1532,
  "preprocess_profile": "default",
  "results": [
    {
      "submission_page_id": "page_001",
      "text": "F = ma",
      "bbox": [120, 340, 680, 410],
      "confidence": 0.87
    }
  ]
}
```

Low-confidence results are persisted and marked for human review by the existing OCR quality gate. Empty OCR output should be reported through `POST /api/v1/ocr-tasks/{id}/fail` with a stable error message such as `empty_ocr_result`.
