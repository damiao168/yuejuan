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
      ,"regions": [
        {"answer_segment_id":"segment_001","question_id":"question_001","question_no":"Q1","question_type":"text","bbox":[120,340,680,410],"submission_page_id":"page_001"}
      ]
    }
  ]
}
```

`regions` 是可选的、按 `answer_segment` 生成的答题区域。存在合法区域时，worker
下载、解码页面各一次并逐区域 OCR，再把 crop-local bbox 平移回 page-global 坐标；旧任务、
`paper_import_job` 或没有区域的页面继续整页 OCR。区域越界、空区域或非法数字会使
任务 fail-closed，不会静默产生错误证据。响应不包含学生姓名、准考证号或答案全文。

ROI 仅适用于原始页面像素坐标。只要一页有配准/模板坐标区域或待复核、被拒绝区域，
该页整体回退到整页 OCR，防止混合坐标空间或遗漏部分答案。配准后的 `pixel_bbox`
属于 `registered_file_asset_id`，不能直接用于裁剪本接口返回的原始 `file_asset_id`。
小数 bbox 向外取整裁剪，结果平移使用实际整数裁剪起点，保持证据像素一致。

## Local CPU OCR profile

Ryzen 7 5800H / 16 GB 本地建议从以下配置开始（`EDUGRADE_OCR_BATCH_SIZE` 必须保持 1）：

```dotenv
EDUGRADE_OCR_MODEL_VERSION=ppocr-v5-mobile
EDUGRADE_OCR_DEVICE=cpu
EDUGRADE_OCR_CPU_THREADS=4
EDUGRADE_OCR_ENABLE_MKLDNN=auto
EDUGRADE_OCR_ENABLE_HPI=false
EDUGRADE_OCR_USE_TEXTLINE_ORIENTATION=true
EDUGRADE_OCR_TEXT_DET_LIMIT_TYPE=min
EDUGRADE_OCR_TEXT_DET_LIMIT_SIDE_LEN=64
EDUGRADE_OCR_TEXT_RECOGNITION_BATCH_SIZE=1
EDUGRADE_OCR_BATCH_SIZE=1
```

`auto` 仅对 mobile 优先尝试 MKLDNN；已知 oneDNN/PIR readiness 兼容错误会记录 warning
并回退到 plain CPU，未知错误和显式 `true` 失败都会直接暴露。HPI 仅为实验开关，默认关闭。
上述默认值是保守的本地候选配置：本地线程矩阵中 4 threads 的平均耗时最佳；
`max=960 + recognition batch=4` 虽提升整页速度，但合成数学 ROI 的 CER gate 未通过，
因此只保留为 benchmark challenger。区域推理始终使用 `min=64`，避免小 crop 不被放大。
基准命令：

```bash
python services/ocr-worker/benchmarks/benchmark_local.py --sample-dir lab/evals/synthetic/fujian-2024-junior-math-v1/answer-sheets --cpu-threads 8 --run-id local-baseline-v1
python services/ocr-worker/benchmarks/benchmark_local.py --roi-dataset lab/evals/synthetic/fujian-2024-junior-math-v1 --cpu-threads 4 --run-id optimized-safe
```

使用真实脱敏数据时可传 `--manifest`；这些命令需要安装本地 Paddle 依赖或使用已缓存模型的
OCR Docker 镜像。合成集测试不等于生产质量 gate 通过；缺少 bbox IoU、low-confidence
recall 标注时输出 N/A。最终实测与验收限制见 `docs/evaluation/local-ocr-performance-2026-09-01.md`。

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
