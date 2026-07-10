# STORY-049 Real OCR Worker 接入规格

## Plan

### 目标

把 OCR 从“任务接口和外部回写协议”推进到“真实 OCR worker 可闭环处理答卷页”的阶段。完成后，系统应能从现有 `ocr_task` 任务出发，由独立 worker 读取真实答卷页文件，执行 OCR/版面文本识别，回写 `ocr_result`，并在低置信度、空文本、页读取失败、引擎异常时进入可追踪的失败或人工复核路径。

本 Story 是生产化路线图中的 P0。它关闭的缺口是 `ocr_engine_inference`，不是完整 Agent Worker Runtime。队列租约、死信、跨 worker 调度等通用运行时能力仍归 STORY-051。

### 背景

已具备：

- STORY-012 已实现 OCR task/result API：
  - `POST /api/v1/submissions/{id}/ocr-tasks`
  - `GET /api/v1/submissions/{id}/ocr-tasks`
  - `GET /api/v1/ocr-tasks/{id}`
  - `POST /api/v1/ocr-tasks/{id}/start`
  - `POST /api/v1/ocr-tasks/{id}/results`
  - `POST /api/v1/ocr-tasks/{id}/fail`
- `ocr_task`、`ocr_result` 已持久化。
- 低置信度结果会标记 `requires_human_review=true`。
- API 已有 `ocr:manage` 权限和审计事件。
- STORY-048 已把 Web 生产菜单中未生产化 mock 页面隐藏，后续真实 OCR 结果不会被假 UI 混淆。
- `lab/` 仍是智能体训练实验，不接入本 Story。

当前缺口：

- 没有真实 OCR 引擎推理。
- 没有 worker 进程消费 OCR 任务。
- 没有从对象存储读取 submission page 原图/PDF 页面的生产流程。
- 没有记录 OCR 引擎版本、模型版本、配置 hash、输入文件 hash 和运行耗时。
- 没有 OCR 样本集评估报告，无法证明中文、手写、公式、表格、低质扫描的实际效果。

## 技术路线与参考

### 候选路线

#### A. Go API Gateway 内嵌 OCR

优点：

- 部署组件少。
- 调用链短。

缺点：

- PaddleOCR、Surya、docTR 等主要 Python 生态接入困难。
- GPU/CPU 推理依赖会污染 API Gateway 镜像。
- OCR 任务耗时长，容易拖慢核心 API 服务。

结论：不采用。API Gateway 应继续负责认证、任务状态、审计和业务边界。

#### B. 独立 Python OCR worker，通过现有 API 闭环

优点：

- 贴合 PaddleOCR、OpenCV、Surya、docTR 生态。
- 不改变现有 Go API 的核心职责。
- 可以先用 polling/单 worker 闭环，后续 STORY-051 再迁移到 River/Temporal 风格运行时。
- 容易输出离线评估报告、模型版本和配置 hash。

缺点：

- 需要新增 worker 镜像、配置、运行说明。
- 第一版不是最终队列运行时，吞吐和并发控制较保守。

结论：本 Story 推荐采用。

#### C. 只做 OCR 评估，不接生产 worker

优点：

- 风险最低，能先看模型效果。

缺点：

- 不能关闭真实 OCR worker 缺口。
- API、Web、阅卷链路仍然没有真实 OCR 闭环。

结论：不采用为主线；评估工具作为 B 的验收组成部分。

### 推荐开源路线

首选生产候选：PaddleOCR。

理由：

- 官方项目覆盖 OCR、文档解析、PP-Structure 等能力，适合中文、多语种和版面类场景。
- 与 Python worker、OpenCV 预处理、私有化部署更匹配。
- 当前项目目标是中文试卷、手写答案、表格和低质扫描，PaddleOCR 是最适合先做生产候选的开源起点。

对照评估：

- Surya：适合作为 layout、table、reading order、LaTeX OCR 对照候选，尤其用于复杂文档解析评估。
- docTR：适合作为 PyTorch/TensorFlow OCR 研究基线，便于评估但不作为首选生产接入。
- Tesseract：适合作为轻量 fallback 或扫描工作站本地退化能力，不作为中文手写和复杂版面的主路线。
- 多模态大模型 OCR：只作为疑难件辅助研究，不作为本 Story 生产基础链路，避免成本、幻觉、隐私和吞吐风险。

评估参考：

- OCRBench v2、OmniDocBench 可作为能力维度参考，但不能直接替代本项目样本集。最终选择必须以本项目真实/脱敏试卷样本、扫描质量、题型和人工复核流程为准。

## 范围

本 Story 实现：

- 新增独立 `ocr-worker` 服务目录，推荐 Python。
- 提供可运行 worker：
  - 读取配置。
  - 获取待处理 OCR task。
  - 调用 `POST /ocr-tasks/{id}/start`。
  - 下载 submission page 对应文件。
  - 运行预处理和 OCR。
  - 将结构化 `text`、`bbox`、`confidence` 回写到 `POST /ocr-tasks/{id}/results`。
  - 失败时调用 `POST /ocr-tasks/{id}/fail`。
- 新增 worker 对接所需的 API 缺口，保持最小：
  - 需要能按租户/权限列出 `queued` OCR task，或提供 worker 专用 pending endpoint。
  - 需要能获得 task 对应 submission pages 和 file asset download URL/内容。
  - 需要 worker 身份使用 HttpOnly cookie 或服务账号 token，不能用硬编码默认账号。
- 新增 OCR 结果元数据：
  - `engine`
  - `engine_version`
  - `model_version`
  - `config_hash`
  - `input_file_hash`
  - `duration_ms`
  - 可选 `preprocess_profile`
- 新增评估工具和样本集规范：
  - `docs/evaluation/ocr-sample-set.md`
  - `docs/evaluation/ocr-metrics.md`
  - `reports/ocr-evaluation/` 输出模板或示例报告。
- 新增 Docker Compose 集成：
  - `ocr-worker` profile 默认为可选启用。
  - 不替换 `ai-services-placeholder`，但 system/info 中 `ocr_engine_inference` 不能继续列为未实现。
- 更新 API 文档、部署文档、Story 索引。

本 Story 不实现：

- 不做完整 River/Temporal worker runtime；后续 STORY-051 处理。
- 不做主观题 AI 评分；后续 STORY-050 处理。
- 不做 OCR 人工校正 UI；后续可进入 Quality/Review 相关 Story。
- 不做完整扫描仪驱动；STORY-053 处理。
- 不承诺某个 OCR 模型直接达到生产效果；本 Story 要求有可运行接入和本项目样本集评估门槛。
- 不接入 `lab/`。

## 设计

### 服务边界

新增 `services/ocr-worker`，负责真实 OCR 推理。API Gateway 仍负责：

- 认证和授权。
- OCR task 状态机。
- OCR result 持久化。
- 低置信度人工复核标记。
- 审计。
- 多租户隔离。

worker 只通过公开/受控 API 访问业务数据，不直接写数据库。

### Worker 运行模式

第一版采用保守 polling：

1. worker 使用服务账号登录或读取服务账号 token。
2. 调用 pending task endpoint 获取有限数量的 `queued` 任务。
3. 对每个 task 调用 start。
4. 获取 submission pages。
5. 下载每页文件。
6. 对图片或 PDF 渲染页执行预处理：
   - 灰度化。
   - 纠偏。
   - 去噪。
   - 裁边。
   - 空白页检测。
7. 使用 PaddleOCR 执行识别。
8. 将每个文本块回写为 `ocr_result`。
9. 如果识别结果为空、平均置信度低于阈值、bbox 越界、页数不一致，则回写低置信度结果或 fail，并进入人工复核路径。

并发限制：

- 默认单 worker、低并发。
- 每次拉取 task 数量可配置。
- 每个 task 单独失败，不影响其他 task。

### API 缺口

现有 API 可以创建、启动、完成和失败 task，但 worker 缺少可靠取活接口。实现阶段需要补：

- `GET /api/v1/ocr-tasks/pending?limit=5`
  - 需要 `ocr:manage` 或未来 `ocr:worker` 权限。
  - 只返回当前租户 queued task。
  - 默认按 `created_at` 升序。
- `GET /api/v1/ocr-tasks/{id}/input`
  - 返回 task、submission、pages、file assets 的最小输入描述。
  - 不返回学生敏感信息。
  - 下载文件仍走已有私有文件下载接口。

如果实现阶段发现已有 submissions/pages/files API 足够拼出输入，可以不新增 input endpoint，但 pending endpoint 仍建议新增。

### 数据模型补强

现有 `ocr_task` 和 `ocr_result` 可支撑基础闭环，但生产追溯不足。需要新增 migration：

- `ocr_task`
  - `model_version TEXT`
  - `config_hash TEXT`
  - `input_hash TEXT`
  - `duration_ms INT`
  - `worker_id TEXT`
  - `attempt_count INT DEFAULT 0`
- `ocr_result`
  - `model_version TEXT`
  - `config_hash TEXT`
  - `input_hash TEXT`
  - `preprocess_profile TEXT`

约束：

- 不允许跨租户回写。
- 结果 bbox 必须是 `[x, y, width, height]` 或 `[x1, y1, x2, y2]` 中的一种，规格必须固定为一种。本 Story 固定为 `[x, y, width, height]`，后端和 worker 都要校验。
- OCR 结果 text 允许为空的情况只能通过 fail 或专门 `empty_page` issue 表达，不能把空文本当正常结果。

### 配置

worker 配置来自环境变量：

- `EDUGRADE_API_BASE_URL`
- `EDUGRADE_OCR_WORKER_TENANT_CODE`
- `EDUGRADE_OCR_WORKER_USERNAME`
- `EDUGRADE_OCR_WORKER_PASSWORD` 或服务账号 token
- `EDUGRADE_OCR_ENGINE=paddleocr`
- `EDUGRADE_OCR_ENGINE_VERSION=pp-ocrv5`
- `EDUGRADE_OCR_MODEL_VERSION`
- `EDUGRADE_OCR_MIN_CONFIDENCE=0.8`
- `EDUGRADE_OCR_POLL_INTERVAL`
- `EDUGRADE_OCR_BATCH_SIZE`
- `EDUGRADE_OCR_DEVICE=cpu|gpu`
- `EDUGRADE_OCR_PREPROCESS_PROFILE=default`

配置要求：

- 不提供默认生产密码。
- 不把 token、学生答案原文、完整 OCR 文本写入普通日志。
- 日志只记录 task id、tenant id、submission id、页号、耗时、结果数量、平均置信度、错误码。

### 评估门槛

必须新增样本集规范，而不是用通用榜单代替验收：

样本至少覆盖：

- 印刷体中文。
- 手写中文短答。
- 数字和英文。
- 数学公式。
- 表格。
- 涂改。
- 低清扫描。
- 倾斜。
- 阴影。
- 双面多页。
- 空白页。

指标：

- CER。
- WER。
- bbox IoU 或区域命中率。
- 页级成功率。
- 空白页误判率。
- 低置信样本召回率。
- 平均耗时。
- P95 耗时。
- 失败率。

第一版验收不要求达到固定营销指标，但必须输出报告，并明确哪些样本类型未达生产阈值、必须进入人工复核。

## 测试要求

实现阶段必须先写失败测试，再实现：

- API 测试：
  - pending endpoint 只返回本租户 queued task。
  - pending endpoint 不返回 processing/completed/failed task。
  - input endpoint 不泄露学生敏感信息。
  - OCR result 元数据落库。
  - bbox schema 被校验。
  - worker fail 会保留错误码和审计。
- Worker 单元测试：
  - PaddleOCR adapter 可以被 fake adapter 替换测试。
  - 空 OCR 结果会 fail 或进入人工复核。
  - 低置信度结果会回写并触发 `requires_human_review`。
  - 文件下载失败会 fail task。
  - API 401/403 不会无限重试。
- 集成测试：
  - 使用小型本地样本图像跑 worker dry-run。
  - 至少一条真实图片经过 worker 回写到 `ocr_result`。
  - Docker Compose config 通过。
  - `system/info` 从 `not_implemented` 移除 `ocr_engine_inference`，并增加真实 OCR worker capability。

## 验收标准

- `services/ocr-worker` 可本地运行。
- 至少支持 PaddleOCR 作为首选真实引擎。
- worker 能通过现有或新增 API 完成 task start/results/fail 闭环。
- OCR 结果来自真实图片/PDF 页推理，不允许用 hard-coded 文本、mock OCR、stub OCR 冒充。
- 结果包含 text、bbox、confidence、engine、engine_version、model_version、config_hash、input_hash、duration_ms。
- 低置信度、空文本、页读取失败、引擎异常都有明确处理。
- 有 OCR 样本集规范和评估报告输出。
- Docker Compose 可选 profile 能启动 worker 或至少通过 config 校验。
- API Gateway `system/info` 不再把 `ocr_engine_inference` 列为未实现。
- 文档明确剩余风险：模型效果未达到某些题型时必须人工复核，不得直接进入最终成绩。
- `lab/` 未接入生产链路。

## 外部参考

- PaddleOCR GitHub: https://github.com/PaddlePaddle/PaddleOCR
- PaddleOCR PP-OCRv5 multilingual recognition docs: https://github.com/PaddlePaddle/PaddleOCR/blob/main/docs/version3.x/algorithm/PP-OCRv5/PP-OCRv5_multi_languages.en.md
- Surya OCR GitHub: https://github.com/datalab-to/surya
- docTR GitHub: https://github.com/mindee/doctr
- Tesseract OCR GitHub: https://github.com/tesseract-ocr/tesseract
- OCRBench v2: https://99franklin.github.io/ocrbench_v2/
- OmniDocBench GitHub: https://github.com/opendatalab/OmniDocBench
- OpenCV docs: https://docs.opencv.org/

## Plan Review

审阅发现：

- 如果直接引入完整队列运行时，会和 STORY-051 重叠；本 Story 应保持 worker 最小闭环。
- 如果只做 PaddleOCR 接入而不做样本集评估，无法证明真实可用；必须把评估报告作为验收门槛。
- 如果 worker 直接写数据库，会绕过 API Gateway 的权限、审计和租户边界；必须通过 API 回写。
- 如果普通日志记录完整 OCR 文本，会泄露学生答案；日志必须脱敏。
- 如果仅用通用 OCR 榜单决定技术路线，会忽略本项目中文手写、低质扫描、表格和公式的真实场景；最终选择必须以本项目样本集为准。
- 如果用 mock adapter 通过验收，会重复 STORY-041 的问题；mock/fake 只能用于测试，不能作为验收证据。

## Spec Fixes

根据审阅，规格修正为：

- 明确本 Story 不实现 River/Temporal 通用 worker runtime。
- 明确 worker 不直接写数据库，只通过 API。
- 明确 bbox schema 固定为 `[x, y, width, height]`。
- 明确普通日志不能记录完整 OCR 文本和学生答案。
- 明确 PaddleOCR 是首选生产候选，但必须由本项目样本集评估报告决定是否进入试点。
- 明确 fake adapter 只能用于单元测试，真实验收必须跑真实图片/PDF 页。

## Historical Implementation Entry Criteria

## Implementation

STORY-049 implemented the first real OCR worker loop without connecting `lab/` to the production path.

Changed backend files:
- `services/api-gateway/internal/ocr/handlers.go`
- `services/api-gateway/internal/ocr/types.go`
- `services/api-gateway/internal/ocr/validation.go`
- `services/api-gateway/internal/ocr/store_memory.go`
- `services/api-gateway/internal/ocr/store_postgres.go`
- `services/api-gateway/internal/ocr/handlers_test.go`
- `services/api-gateway/internal/server/server.go`
- `services/api-gateway/internal/handlers/handlers.go`
- `services/api-gateway/migrations/000021_story049_ocr_worker_metadata.sql`

Backend behavior:
- Added `GET /api/v1/ocr-tasks/pending` for tenant-scoped queued task polling.
- Added `GET /api/v1/ocr-tasks/{id}/input` for minimal worker input without student identity fields.
- Persisted worker metadata: `model_version`, `config_hash`, `input_hash`, `duration_ms`, `worker_id`, `attempt_count`, and `preprocess_profile`.
- Fixed bbox validation to require `[x, y, width, height]` with non-negative origin and positive size.
- Moved `ocr_engine_inference` from `not_implemented` into system capabilities.

Changed worker/deployment files:
- `services/ocr-worker/ocr_worker/api.py`
- `services/ocr-worker/ocr_worker/config.py`
- `services/ocr-worker/ocr_worker/engine.py`
- `services/ocr-worker/ocr_worker/runner.py`
- `services/ocr-worker/ocr_worker/__main__.py`
- `services/ocr-worker/pyproject.toml`
- `services/ocr-worker/Dockerfile`
- `infra/docker-compose/docker-compose.yml`
- `infra/docker-compose/.env.example`

Worker behavior:
- Uses the API Gateway login endpoint and Bearer token.
- Polls pending OCR tasks, starts each task, downloads page inputs, calls the OCR engine, and completes or fails the task through API endpoints.
- Uses a PaddleOCR adapter as the production candidate. Unit tests use a fake engine only as a test double.
- Fails empty OCR output with `empty_ocr_result` and download failures with `download_failed`.

Changed evaluation/docs files:
- `docs/evaluation/ocr-sample-set.md`
- `docs/evaluation/ocr-metrics.md`
- `docs/api/ocr.md`
- `scripts/check-story049-ocr-worker.mjs`

Evaluation behavior:
- Added project-specific OCR sample-set requirements.
- Added metrics and first-pilot gates for CER, WER, bbox, blank page false positives, low-confidence recall, failure rate, and duration.
- Added a static STORY-049 check to prevent missing worker modules, missing evaluation docs, missing Compose profile, and incorrect `system/info` capability state.

## Implementation Review

Acceptance review:
- Real worker exists under `services/ocr-worker` and can be run as `python -m ocr_worker`.
- PaddleOCR is the default production candidate; fake OCR is limited to unit tests.
- Worker closes the task loop through API Gateway: pending, input, start, results, and fail.
- Result metadata is stored on task/result records for replay and traceability.
- Low-confidence results are completed so the existing backend quality gate can mark human review.
- Empty output and download failures take explicit failure paths.
- Docker Compose exposes `ocr-worker` only behind the optional `ocr` profile.
- `system/info` no longer lists `ocr_engine_inference` as not implemented.
- `lab/` remains disconnected from production.

Residual risks:
- This story does not prove OCR quality on real exam scans. It creates the runnable integration and evaluation gate; a real authorized sample-set run is still required before pilot.
- The first worker uses conservative polling, not the full Agent Worker Runtime planned in STORY-051.
- Service account provisioning for `ocr_worker` must be handled operationally; no committed default password is provided.

## Fixes

Fixes after implementation review:
- Removed a syntax fragment in `api.py` left from the interrupted edit.
- Added explicit Python package build metadata so the Docker image does not rely on implicit package discovery.
- Corrected the Compose verification command to use `--profile ocr`.
- Tightened the static check so it parses the capability and `not_implemented` sections separately.
- Added API documentation for pending/input endpoints and OCR result metadata.

## Approval

Approved after fresh verification on 2026-07-08.

Verification evidence:
- `go test ./...` from `services/api-gateway`: passed.
- `python -m unittest discover -s tests` from `services/ocr-worker`: 4 tests passed.
- `npm.cmd run check:story049` from repo root: STORY-049 OCR worker checks passed.
- `docker compose --env-file infra\docker-compose\.env.example -f infra\docker-compose\docker-compose.yml --profile ocr config`: passed.

Conclusion: STORY-049 is complete for the planned scope: real OCR worker integration, backend worker endpoints, metadata persistence, optional Compose deployment, and evaluation gates. The remaining OCR quality proof belongs to running the new sample-set evaluation in a pilot/pre-production environment, not to replacing this story with mock OCR.

## Implementation Entry Criteria

进入实现前需要确认：

- 接受“独立 Python OCR worker + 现有 API 闭环 + PaddleOCR 首选候选”的路线。
- 接受 STORY-049 不等待 STORY-051 的完整队列运行时。
- 接受第一版 OCR 生产能力以“可运行闭环 + 样本集评估 + 低置信人工复核”为验收，而不是承诺所有题型自动识别准确。
