# STORY-012 OCR 服务接口与任务队列

## 状态

Approved

## 目标

实现 OCR 任务模型、任务队列抽象、OCR 结果结构和外部 OCR worker 回写接口，为后续答题区域切分和人工校正提供可靠输入。

## Plan

- 新增 migration：
  - `ocr_task`
  - `ocr_result`
  - `ocr:manage` 权限。
- 新增 `internal/ocr`：
  - 类型定义和状态机。
  - Store interface。
  - Queue interface。
  - MemoryStore/MemoryQueue。
  - PostgresStore。
  - Handler。
- 接口：
  - `POST /api/v1/submissions/{id}/ocr-tasks`
  - `GET /api/v1/submissions/{id}/ocr-tasks`
  - `GET /api/v1/ocr-tasks/{id}`
  - `POST /api/v1/ocr-tasks/{id}/start`
  - `POST /api/v1/ocr-tasks/{id}/results`
  - `POST /api/v1/ocr-tasks/{id}/fail`
- 规则：
  - 创建 OCR 任务前，submission 必须是 `ready_for_ocr`。
  - 创建任务只入队，不执行 OCR。
  - worker 回写结果必须符合结构化 schema。
  - OCR result 必须保存文本、bbox、confidence、page_id、engine、version。
  - 低于置信度阈值的任务标记 `requires_human_review=true`。
  - 状态受控：`queued -> processing -> completed/failed`，可从 `queued` 取消。
  - 所有关键动作写审计。

## Plan Review

- 不越界：不集成真实 PaddleOCR、不做版面解析、不切题、不做 AI 评分。
- 不做假功能：不会生成模拟 OCR 文本；结果只能由 API 回写。
- 与 STORY-011 衔接：只允许 `ready_for_ocr` 的 submission 创建任务。
- 与 STORY-013 衔接：OCR result 的 page/block/bbox/confidence 将作为答题区域切分输入。

## 非范围

- 真实 OCR 引擎推理。
- 图像预处理、纠偏、模糊检测。
- 答题区域切分。
- OCR 人工校正 UI。
- 分布式队列 Redis/Temporal 的真实 worker。

## 验收标准

- 可以为 `ready_for_ocr` 的 submission 创建 OCR 任务。
- 未通过质量门禁的 submission 不能创建 OCR 任务。
- 创建任务会入队并记录状态。
- 可以启动任务、提交结构化 OCR 结果、标记失败。
- 低置信度结果会触发 `requires_human_review`。
- 任务状态流转受控。
- 所有接口受 `ocr:manage` 权限保护。
- 创建、启动、完成、失败写审计。
- 测试通过。

## Implementation

- 新增 `000007_ocr_task.sql`：
  - `ocr:manage` 权限。
  - `ocr_task` 表。
  - `ocr_result` 表。
- 新增 `internal/ocr`：
  - 类型定义和状态机。
  - MemoryStore。
  - MemoryQueue。
  - PostgresStore。
  - Handler。
- API Gateway 挂载：
  - `POST /api/v1/submissions/{id}/ocr-tasks`
  - `GET /api/v1/submissions/{id}/ocr-tasks`
  - `GET /api/v1/ocr-tasks/{id}`
  - `POST /api/v1/ocr-tasks/{id}/start`
  - `POST /api/v1/ocr-tasks/{id}/results`
  - `POST /api/v1/ocr-tasks/{id}/fail`
- 更新 `system/info` capabilities：
  - `ocr_task_management`
  - `ocr_result_ingestion`
  - `ocr_low_confidence_review_trigger`
- 保留 `ocr_engine_inference` 在 `not_implemented`。
- 新增 `docs/api/ocr.md`，更新 README。

## Implementation Review

逐项检查结果：

- `ready_for_ocr` 检查：创建 OCR 任务前会读取 submission 状态，未 ready 返回 `409 submission_not_ready_for_ocr`。
- 入队：创建任务后调用 Queue interface；当前为进程内队列抽象，持久任务记录在 `ocr_task` 表。
- 状态流转：仅允许 `queued -> processing -> completed/failed`，测试覆盖未 start 直接提交结果失败。
- 结构化结果：每条结果保存 page、text、bbox、confidence、engine、version。
- 低置信度：任一结果低于 `min_confidence` 时 `requires_human_review=true`，测试覆盖。
- 页面归属：Handler 和 PostgresStore 均防止把非本 submission 的页面写入任务结果。
- 权限：所有 OCR 接口要求 `ocr:manage`，测试覆盖权限不足。
- 审计：创建、启动、完成、失败均调用 audit。
- 不越界：未实现真实 OCR 推理、图像预处理、版面解析、答题区域切分。

实现审阅中发现并修正的问题：

- Postgres 结果写入需确保 `submission_page_id` 属于该 task 的 submission，已补校验。
- 测试中自定义空 context 不够清晰，已改为 `context.Background()`。

## Fixes

- 增加 Postgres `ensurePageBelongsToSubmission`。
- Handler 提交结果前按 submission page 列表做归属校验。
- 增加创建任务 ready 状态、未 start 提交、低置信度复核、失败任务、权限不足测试。

## Verification

运行命令：

```powershell
Push-Location .\services\api-gateway
go test ./...
Pop-Location
docker compose --env-file .env.example -f infra\docker-compose\docker-compose.yml config
Push-Location .\services\api-gateway
go build -o ..\..\bin\api-gateway.exe .\cmd\api-gateway
Pop-Location
```

结果：

```text
go test ./... -> passed
docker compose config -> passed
go build -> passed
```

启动验证：

```powershell
GET http://127.0.0.1:18092/api/v1/system/info
GET http://127.0.0.1:18092/api/v1/submissions/submission-1/ocr-tasks
```

结果：

```text
system/info -> capabilities include ocr_task_management, ocr_result_ingestion, ocr_low_confidence_review_trigger
system/info -> not_implemented includes ocr_engine_inference, ai_grading
GET /api/v1/submissions/submission-1/ocr-tasks without token -> 401
```

## Approval

Approved。

本 Story 满足验收标准，可以进入 `STORY-013 答题区域切分模块`。
