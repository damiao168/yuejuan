# STORY-051 Agent Worker Runtime 规格

## Plan

### 目标

把 STORY-049 OCR worker 和 STORY-050 image-quality worker 中已经出现的任务领取、lease、幂等、重试、失败隔离和审计能力抽象成通用 Agent Worker Runtime。

完成后，生产链路中的 OCR、版面识别、图像预处理、AI 评分、证据校验、报告生成、导出、桌面端同步等后台任务，不再各自发明一套队列协议。系统应提供统一的任务记录、状态机、worker 领取协议、结果回写协议、死信处理、指标和审计。

目标流程：

```text
业务动作创建 source record
-> API Gateway 同事务创建 agent_worker_task
-> worker/runtime claim with lease
-> worker 执行真实能力
-> heartbeat / complete / fail / cancel
-> runtime 写审计、指标、attempt 历史
-> source record 幂等激活结果
```

本 Story 关闭的是 `agent_worker_runtime` 缺口，不是主观题 AI 模型、模板配准、答案分组或生产部署 Runbook。

### 背景

已具备：

- STORY-049 已有真实 OCR worker、OCR 任务元数据和评估规范。
- STORY-050 已有 image-quality worker、不可变 quality run、claim + lease、标准化 RGB PNG 资产和质量门禁。
- API Gateway 使用 Go + PostgreSQL，业务状态、审计、多租户约束已经以 PostgreSQL 为事实源。
- `lab/` 仍是智能体训练实验约束，不接入生产链路。

当前缺口：

- OCR 和 image-quality 已经分别实现了相似的领取、lease、回写逻辑，继续扩展会造成重复状态机。
- 缺少跨任务类型统一的 `queued/leased/running/succeeded/failed/dead_letter/cancelled` 状态语义。
- 缺少统一幂等键、attempt 历史、retry/backoff、worker heartbeat、dead letter 和 cancel 协议。
- 缺少能让生产运维查看队列长度、失败率、重试次数、P95 耗时、worker 心跳、死信数量的统一指标。
- 缺少把任务创建、领取、完成、失败、重试、取消统一写入 audit log 的机制。
- 当前 worker 崩溃恢复规则分散，后续接入 AI 评分、证据校验和导出任务时风险会放大。

### 技术路线

推荐路线：Go API Gateway 内新增 PostgreSQL-backed Agent Worker Runtime，外部暴露语言无关 HTTP worker 协议；River 作为 Go + PostgreSQL 队列内核的首选实现方向，但不得把 River 表结构或 Go 类型暴露给 Python worker。

选择理由：

- River 的定位贴合 Go + PostgreSQL 任务队列，适合和现有业务事务保持一致。
- 当前 OCR 和 image-quality 的真实执行逻辑在 Python，Python worker 不应直接连业务数据库或依赖 River 内部表。
- 因此运行时边界必须是本项目自己的 HTTP 协议：claim、heartbeat、complete、fail、cancel。
- River 可以在 Go 侧承担 enqueue、retry、scheduled job、worker supervision 等能力；即使实现阶段采用本项目自有表保存业务级任务状态，外部协议也必须保持可迁移。
- Temporal 适合后续长流程、多服务补偿、人工暂停恢复和复杂编排，不作为 STORY-051 首版默认依赖。
- Asynq 依赖 Redis 作为核心队列，会让任务状态和业务事实源分裂，不适合作为评分主队列首选。
- NATS JetStream 更适合作为事件流、边缘同步或桌面端消息通道，不适合作为首版业务任务状态机主干。

### 开源参考

- River：Go + PostgreSQL 背景任务队列，适合作为首版 Go 侧队列内核参考。
- Temporal：Durable Execution 和 Workflow/Activity 模型适合后续复杂长流程。
- Asynq：Redis-backed Go task queue，适合作为对照但不作为本项目首选主干。
- NATS JetStream：持久化消息和事件流能力强，适合作为后续事件总线或边缘同步候选。

参考链接见文末。

## Scope

本 Story 实现：

- 新增通用 worker runtime 数据模型。
- 新增通用 worker runtime Go 包，建议路径：`services/api-gateway/internal/workerruntime`。
- 新增 migration，建议编号：`services/api-gateway/migrations/000023_story051_agent_worker_runtime.sql`。
- 新增任务状态机和合法状态迁移校验。
- 新增任务创建、claim、heartbeat/start、complete、fail、cancel、dead-letter 查询能力。
- 支持任务类型：
  - `ocr`
  - `layout`
  - `preprocess`
  - `image_quality`
  - `ai_grade`
  - `evidence_verify`
  - `report_generate`
  - `export`
  - `desktop_sync`
- 支持状态：
  - `queued`
  - `leased`
  - `running`
  - `succeeded`
  - `failed`
  - `dead_letter`
  - `cancelled`
- 每个任务必须包含：
  - `tenant_id`
  - `task_type`
  - `source_type`
  - `source_id`
  - `idempotency_key`
  - `payload_schema_version`
- 支持 retryable error 与 terminal error 分离。
- 支持 max attempts、backoff、lease timeout 和 worker crash recovery。
- 支持 attempt 历史与 worker heartbeat。
- 支持 audit log 记录任务 create、claim、start、heartbeat timeout、complete、fail、retry、dead_letter、cancel。
- 支持最小指标查询：队列长度、失败率、重试次数、P95 耗时、worker 心跳、死信数量。
- 至少接入一个真实已有 worker 路径作为验收样板。首选 STORY-050 image-quality，因为它已有清晰的 run、source hash 和标准化资产回写语义。
- 为 STORY-049 OCR 提供兼容适配方案，避免后续再复制一套 claim/lease。
- 更新 Story 索引、生产路线图和相关部署说明。

本 Story 不实现：

- 不实现完整 Temporal workflow。
- 不实现主观题 AI 模型推理。
- 不实现模板配准、按题切图或答案分组。
- 不重写 OCR/PaddleOCR 或 image-quality 的核心图像算法。
- 不让 Python worker 直连 PostgreSQL。
- 不把 `lab/` 接入生产链路。
- 不新增 Web 管理大屏。首版只提供 API 或 SQL 可验证指标。
- 不承诺桌面端离线同步生产可用，`desktop_sync` 只预留任务类型。

## Runtime Boundary

### 控制面

Go API Gateway 负责：

- 创建和持久化任务。
- 校验租户、权限、source record 是否存在。
- 控制状态迁移。
- 发放 lease token。
- 校验 idempotency key。
- 写 audit log。
- 暴露指标。
- 调用 source-specific activator，把 runtime result 幂等激活到业务表。

### 执行面

Python/其他 worker 负责：

- 通过 HTTP claim 任务。
- 通过受控下载接口读取输入资产。
- 执行真实 OCR、图像质量检测、AI 推理或导出逻辑。
- 通过 HTTP heartbeat。
- 通过 HTTP complete/fail 回写结构化结果或错误。

worker 不直接写数据库，不直接改业务状态，不绕过 API Gateway 权限与租户边界。

### River 的位置

River 不是外部 worker 协议。实现阶段可以把 River 用作 Go 侧队列引擎，但必须满足：

- River job args 只保存 `agent_worker_task.id` 或必要的轻量引用。
- Python worker 不读取 River 表。
- 业务可见状态以 `agent_worker_task` 和 source record 为准。
- 不因为后续更换 River/Temporal 而改变 Python worker 的 HTTP 协议。

如果实现阶段发现 River 直接接入会显著放大改动，可以先实现 PostgreSQL-backed runtime 表和 HTTP 协议，但 Story 文档必须记录 River 延后原因、风险和下一步迁移点。不能把“未接 River”伪装成“River 已接入”。

## Data Model

### `agent_worker_task`

```sql
CREATE TABLE agent_worker_task (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  task_type TEXT NOT NULL,
  queue_name TEXT NOT NULL,
  source_type TEXT NOT NULL,
  source_id UUID NOT NULL,
  status TEXT NOT NULL,
  priority INT NOT NULL DEFAULT 100,
  payload JSONB NOT NULL DEFAULT '{}',
  payload_schema_version TEXT NOT NULL,
  result JSONB NOT NULL DEFAULT '{}',
  result_schema_version TEXT NULL,
  idempotency_key TEXT NOT NULL,
  dedupe_key TEXT NULL,
  max_attempts INT NOT NULL DEFAULT 3,
  attempt_count INT NOT NULL DEFAULT 0,
  retry_backoff_seconds INT NOT NULL DEFAULT 60,
  not_before TIMESTAMPTZ NULL,
  lease_token TEXT NULL,
  lease_expires_at TIMESTAMPTZ NULL,
  leased_by TEXT NULL,
  started_at TIMESTAMPTZ NULL,
  completed_at TIMESTAMPTZ NULL,
  cancelled_at TIMESTAMPTZ NULL,
  error_code TEXT NULL,
  error_detail JSONB NOT NULL DEFAULT '{}',
  created_by UUID NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

约束建议：

- `(tenant_id, id)` 复合唯一。
- `(tenant_id, task_type, idempotency_key)` 唯一，防止重复创建同一业务任务。
- `status` 使用 check constraint 限制枚举。
- `task_type` 使用 check constraint 限制首版任务类型。
- `max_attempts >= 1`。
- `attempt_count >= 0`。
- `payload` 不保存学生完整原文答案、完整 OCR 文本或敏感成绩，只保存 asset id、source id、hash、profile、版本等引用。

### `agent_worker_task_attempt`

```sql
CREATE TABLE agent_worker_task_attempt (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  task_id UUID NOT NULL,
  attempt_no INT NOT NULL,
  worker_service TEXT NOT NULL,
  worker_instance_id TEXT NOT NULL,
  lease_token TEXT NOT NULL,
  status TEXT NOT NULL,
  started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  heartbeat_at TIMESTAMPTZ NULL,
  completed_at TIMESTAMPTZ NULL,
  duration_ms INT NULL,
  error_code TEXT NULL,
  error_detail JSONB NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

约束建议：

- `(tenant_id, task_id, attempt_no)` 唯一。
- `task_id` 使用同租户外键指向 `agent_worker_task`。
- attempt 历史不可覆盖，只允许补充完成时间和错误信息。

### `agent_worker_heartbeat`

```sql
CREATE TABLE agent_worker_heartbeat (
  tenant_id UUID NOT NULL,
  worker_service TEXT NOT NULL,
  worker_instance_id TEXT NOT NULL,
  queue_name TEXT NOT NULL,
  last_seen_at TIMESTAMPTZ NOT NULL,
  metadata JSONB NOT NULL DEFAULT '{}',
  PRIMARY KEY (tenant_id, worker_service, worker_instance_id, queue_name)
);
```

### Source Link

每个 runtime task 必须引用业务 source：

- `source_type=image_quality_run` 时，`source_id=submission_page_quality_run.id`。
- `source_type=ocr_task` 时，`source_id=ocr_task.id`。
- 后续 `source_type=ai_grade_job`、`evidence_check`、`report_job` 等按业务表扩展。

runtime 的 `succeeded` 不等于业务结果可见。业务结果必须由 source-specific activator 进行幂等激活。例如 image-quality 需要校验 source file hash 和 latest run，OCR 需要校验 task 状态和 result version。

## Status Machine

允许迁移：

```text
queued -> leased
leased -> running
leased -> queued
leased -> failed
leased -> dead_letter
leased -> cancelled
running -> succeeded
running -> queued
running -> failed
running -> dead_letter
running -> cancelled
queued -> cancelled
failed -> queued
dead_letter -> queued
```

语义：

- `queued`：可被领取，或等待 `not_before` 到期。
- `leased`：worker 已领取 lease，但尚未声明进入执行。
- `running`：worker 已开始执行并通过 heartbeat 保活。
- `succeeded`：runtime task 完成，result 已被接受。
- `failed`：不可重试业务失败，或人工决定终止。
- `dead_letter`：可重试失败达到 `max_attempts`。
- `cancelled`：任务被用户、系统或 source 状态变更取消。

lease 规则：

- claim 时生成 `lease_token`。
- complete/fail/heartbeat 必须携带当前 `lease_token`。
- lease 过期后，任务可以重新进入 `queued` 或被新的 worker claim。
- 过期 lease 的迟到 complete 必须被拒绝，除非 source-specific activator 能证明结果和当前 source 完全一致且未被更新。

retry 规则：

- fail 请求必须包含 `retryable`。
- `retryable=true` 且 `attempt_count < max_attempts` 时，任务回到 `queued`，设置 `not_before`。
- `retryable=true` 且达到上限时，任务进入 `dead_letter`。
- `retryable=false` 时，任务进入 `failed`。
- dead letter 重新投递必须写审计，且保留历史 attempt。

## API Design

内部 worker API 建议路径：

### 创建任务

```text
POST /api/v1/internal/worker/tasks
```

用途：业务模块或受控内部流程创建 runtime task。普通前端用户不直接调用。

请求字段：

```json
{
  "tenant_id": "uuid",
  "task_type": "image_quality",
  "queue_name": "image-quality",
  "source_type": "image_quality_run",
  "source_id": "uuid",
  "idempotency_key": "image-quality-run:uuid",
  "payload_schema_version": "image-quality.v1",
  "payload": {
    "submission_page_id": "uuid",
    "source_file_asset_id": "uuid",
    "source_sha256": "..."
  },
  "priority": 100,
  "max_attempts": 3
}
```

同一 `(tenant_id, task_type, idempotency_key)` 重复请求必须返回已有任务，不创建重复任务。

### 领取任务

```text
POST /api/v1/internal/worker/tasks/claim
```

请求字段：

```json
{
  "queue_name": "image-quality",
  "worker_service": "image-quality-worker",
  "worker_instance_id": "host-pid-random",
  "limit": 1,
  "lease_seconds": 300
}
```

响应返回任务、payload、attempt_no、lease_token、lease_expires_at。claim 必须按租户、队列、状态、`not_before` 和优先级筛选。

### 进入运行和心跳

```text
POST /api/v1/internal/worker/tasks/{taskId}/heartbeat
```

请求字段：

```json
{
  "lease_token": "...",
  "worker_service": "image-quality-worker",
  "worker_instance_id": "host-pid-random",
  "state": "running",
  "progress": {
    "phase": "normalize",
    "percent": 60
  }
}
```

首次 heartbeat 可以把任务从 `leased` 推进到 `running`。后续 heartbeat 延长 lease 或更新 worker 心跳。

### 完成任务

```text
POST /api/v1/internal/worker/tasks/{taskId}/complete
```

请求字段：

```json
{
  "lease_token": "...",
  "result_schema_version": "image-quality-result.v1",
  "result": {
    "quality_run_id": "uuid",
    "normalized_file_asset_id": "uuid",
    "quality_status": "passed",
    "result_version": "..."
  },
  "duration_ms": 1234
}
```

complete 必须：

- 校验 lease。
- 写 attempt 完成信息。
- 写 task result。
- 调用 source-specific activator。
- activator 成功后设置 `succeeded`。
- activator 失败时按错误类型进入 retry 或 failed，不能把 runtime succeeded 留给未激活业务结果。

### 失败任务

```text
POST /api/v1/internal/worker/tasks/{taskId}/fail
```

请求字段：

```json
{
  "lease_token": "...",
  "retryable": true,
  "error_code": "object_storage_timeout",
  "error_detail": {
    "phase": "download"
  },
  "duration_ms": 5000
}
```

### 取消任务

```text
POST /api/v1/internal/worker/tasks/{taskId}/cancel
```

用于 source record 被替换、删除、人工撤销或系统维护。取消必须写审计。

### 查询指标

```text
GET /api/v1/internal/worker/metrics
```

首版返回结构化 JSON：

```json
{
  "queues": [
    {
      "queue_name": "image-quality",
      "queued": 12,
      "leased": 1,
      "running": 3,
      "dead_letter": 0,
      "failed_last_hour": 2,
      "retry_last_hour": 4,
      "p95_duration_ms": 1800
    }
  ],
  "workers": [
    {
      "worker_service": "image-quality-worker",
      "worker_instance_id": "host-pid-random",
      "queue_name": "image-quality",
      "last_seen_at": "2026-07-09T00:00:00Z"
    }
  ]
}
```

## Integration Strategy

### Pilot: image-quality

STORY-051 的首个真实接入建议使用 image-quality：

- `RunQualityCheck` 创建 `submission_page_quality_run` 后，同时创建 `agent_worker_task`。
- 现有 image-quality claim/result endpoint 可以保留为兼容包装，但内部应委托到 runtime claim/complete/fail。
- source-specific activator 继续执行 STORY-050 的校验：
  - source file asset 未变化。
  - source sha256 匹配。
  - run 仍是当前页面最新 run。
  - normalized file asset 属于同租户。
  - quality status 合法。

验收时必须证明：至少一个 image-quality 任务是通过 runtime task 领取、heartbeat、complete，并激活到 `submission_page` 的。

### Adapter: OCR

OCR 接入可以先做兼容层：

- 创建 OCR task 时生成 `agent_worker_task(task_type=ocr, source_type=ocr_task)`。
- OCR worker 的 pending/claim 逻辑逐步切到 runtime claim。
- OCR complete/fail 最终通过 runtime complete/fail 调用 OCR source activator。

如果实现阶段迁移 OCR 风险过大，本 Story 至少必须留下 OCR adapter 规格、表字段和测试入口，且明确后续迁移 Story。不能新增第三套 OCR 队列协议。

### Future Task Types

`layout`、`ai_grade`、`evidence_verify`、`report_generate`、`export`、`desktop_sync` 只要求首版枚举、payload 版本规则和状态机可支持。除非已有真实业务 source，不需要在本 Story 实现推理或导出逻辑。

## Security And Privacy

- worker 使用服务账号或内部服务凭证，不使用默认管理员账号。
- worker 请求必须受内部认证保护，不能开放给普通浏览器会话。
- 所有 task 查询和 claim 必须带 tenant boundary。
- payload、result、error_detail 和日志不得保存完整学生答案、完整 OCR 文本、成绩明细、密码、token、密钥。
- `worker_instance_id` 只作诊断，不作权限来源。
- complete/fail 必须校验 lease token，不能只靠 task id。
- cancel、dead-letter requeue 和人工重试必须写 audit log。
- 任何 source activator 都必须幂等，重复 complete 不能产生重复业务结果。

## Acceptance Criteria

功能验收：

- 能创建通用 runtime task。
- 重复 idempotency key 不会创建重复任务。
- worker 能 claim 到 task，并获得 lease token。
- worker heartbeat 能把任务推进到 `running` 并更新心跳。
- worker complete 能把任务推进到 `succeeded`，并激活至少一个真实业务 source。
- worker fail retryable 能按 backoff 回到 `queued`。
- retry 达到上限进入 `dead_letter`。
- terminal fail 进入 `failed`。
- cancel 能让 `queued/leased/running` 任务进入 `cancelled`。
- lease 过期后任务可被重新 claim。
- 过期 lease 的 complete 被拒绝。
- dead letter requeue 保留历史 attempt，并写审计。

生产验收：

- 至少 image-quality worker 通过 runtime 完成一条真实任务闭环。
- OCR 不能再新增独立的第三套 claim/lease 协议。
- 指标接口能返回队列长度、失败率、重试次数、P95 耗时、worker 心跳和死信数量。
- audit log 能看到 create、claim、running、complete、fail、retry、cancel。
- 所有新增表都有 tenant 约束和必要索引。
- Python worker 不直连 PostgreSQL。
- `lab/` 没有被引入生产路径。

测试验收：

- Go 单元测试覆盖状态迁移、幂等创建、claim lease、retry/dead-letter、cancel、metrics 聚合。
- API handler 测试覆盖权限、租户隔离、非法 lease、迟到 complete。
- image-quality runtime adapter 测试覆盖真实 source activator。
- 如迁移 OCR adapter，覆盖 OCR task 创建和 runtime task 的对应关系。
- Docker Compose 配置检查通过。
- 现有 STORY-049 和 STORY-050 静态检查、Go 测试、Python 测试不回退。

建议验证命令：

```powershell
cd D:\project\yuejuan\services\api-gateway
go test ./...
```

```powershell
cd D:\project\yuejuan\services\image-quality-worker
python -m pytest -q
```

```powershell
cd D:\project\yuejuan
npm.cmd run check:story050
docker compose --env-file infra/docker-compose/.env.example -f infra/docker-compose/docker-compose.yml --profile quality config
```

实现阶段应新增 `check:story051` 或等价静态检查，验证 migration、runtime package、handlers、docs 和 compose 线索都存在。

## Implementation Plan Draft

实现前按以下顺序推进：

1. 写 runtime 数据模型和 migration 测试。
2. 写状态机单元测试。
3. 实现 `internal/workerruntime` store、types、service。
4. 实现 claim/heartbeat/complete/fail/cancel handler。
5. 实现 audit event 写入点。
6. 实现 metrics 查询。
7. 接入 image-quality pilot，并保留兼容 endpoint。
8. 评估 OCR adapter 的最小迁移点，至少避免后续新增重复协议。
9. 更新 Docker/env/docs/check script。
10. 运行 Go、Python、静态和 Compose 验证。

## Plan Review

规格审阅结论：需要修正后可进入实现。

审阅发现：

- 原路线图写“River 首版”容易被误解为 Python worker 直接依赖 River。这样会突破 worker 不直连数据库的边界，也会把 Go 队列库暴露成跨语言协议。
- 如果一次迁移 OCR、image-quality、AI grade、report、export 全部 worker，Story 会过大，风险不利于审阅。
- 原先的 worker 状态和业务状态边界不够清晰，可能出现 runtime succeeded 但业务 source 未激活的假完成。
- 指标和审计不能只作为“后续优化”，否则运行时上线后无法定位死信和重复执行。
- payload 隐私边界必须写清，否则后续 OCR/AI 可能把学生原文答案写入任务日志。

## Spec Fixes

已根据规格审阅完成以下修正：

- 明确 River 是 Go 侧队列内核候选，不是外部 worker 协议。
- 明确 HTTP worker protocol 是稳定边界，Python worker 不直连 PostgreSQL。
- 把首个真实接入收敛为 image-quality pilot，OCR 先做 adapter 约束。
- 增加 source-specific activator，防止 runtime 完成和业务结果激活脱节。
- 增加租户、隐私、审计、指标、lease 过期、迟到 complete、dead-letter requeue 的验收要求。
- 明确 `lab/` 不接入生产链路。

规格修正后结论：Spec Ready。下一步可以进入实现，但实现完成前不得把 STORY-051 标记为 Approved。

## Implementation

已实现 PostgreSQL-backed Agent Worker Runtime，并接入 OCR 与 image-quality 两条真实 Python worker 路径。

新增：

- `services/api-gateway/migrations/000023_story051_agent_worker_runtime.sql`
- `services/api-gateway/internal/workerruntime/types.go`
- `services/api-gateway/internal/workerruntime/validation.go`
- `services/api-gateway/internal/workerruntime/store_memory.go`
- `services/api-gateway/internal/workerruntime/store_postgres.go`
- `services/api-gateway/internal/workerruntime/handlers.go`
- `services/api-gateway/internal/workerruntime/store_memory_test.go`
- `services/api-gateway/internal/workerruntime/handlers_test.go`
- `scripts/check-story051-agent-worker-runtime.mjs`

主要实现：

- 新增 `agent_worker_task`、`agent_worker_task_attempt`、`agent_worker_heartbeat`。
- 使用 PostgreSQL `FOR UPDATE SKIP LOCKED` 完成并发安全领取。
- 实现幂等创建、lease token、heartbeat 续租、attempt 历史、指数退避、terminal failure、dead letter、人工 requeue、cancel。
- 实现任务和 worker 指标：状态数量、最近一小时失败/重试、P95 耗时、worker heartbeat。
- 新增通用内部 API：create、claim、get、heartbeat、complete、fail、cancel、requeue、metrics。
- runtime payload 拒绝学生身份、完整答案/OCR 文本、密码、token、secret 等敏感键。
- runtime create/claim/running/complete/fail/retry/dead-letter/cancel/requeue 写入 audit log。
- image-quality 原兼容端点内部改用 runtime claim/lease/attempt；成功和异常结果同步激活 quality run 与 runtime task。
- OCR task 创建时同步创建 runtime task；OCR worker 改用 runtime claim/heartbeat，OCR source 完成或失败后再完成/失败 runtime task。
- 通用 complete/fail 禁止直接结束 `ocr_task` 和 `image_quality_run`，必须走 source adapter，防止 runtime succeeded 但业务结果未激活。
- `/api/v1/system/info` 已把 `agent_worker_runtime` 从 `not_implemented` 移到 `implemented`。
- OCR lease 新增 `EDUGRADE_OCR_LEASE_SECONDS` 配置。

技术路线落地说明：

- 本 Story 没有声称 River 已接入。
- 首版实际采用本项目 PostgreSQL runtime + 稳定 HTTP worker protocol。
- River 仍保留为 Go 侧队列内核升级候选。当前 OCR/image-quality 执行面是 Python，直接让 Python 依赖 River 内部表会破坏跨语言边界并造成两套 claim 机制，因此本 Story 不做这种接入。
- 后续引入 River 或 Temporal 时，Python worker 协议和业务 source activator 不需要改变。

## Implementation Review

实现审阅发现：

- 通用 complete/fail 最初可以绕过 OCR/image-quality source 激活，存在假完成风险。
- PostgreSQL metrics 最初在 queue 结果集未关闭时查询 retry count，低连接数环境可能阻塞。
- heartbeat 最初只记录心跳，没有延长 lease，长 OCR 任务仍可能被重复领取。
- dead-letter requeue 最初保留已耗尽的 `max_attempts`，下一次失败会立即重新进入死信。
- image-quality worker 最初没有把处理异常回写为 retryable failure，quality run 可能停在 processing。
- OCR lease 最初不可通过环境变量配置。

审阅结论：上述问题必须修正后才能批准。

## Fixes

已完成：

- 为 OCR/image-quality 增加 source-specific completion adapter，并阻止通用端点绕过。
- metrics 先关闭 queue rows，再查询 retry count。
- heartbeat 校验 worker identity，并按 `lease_seconds` 续租。
- dead-letter 人工 requeue 自动增加至少一次可用尝试额度，同时保留 attempt 历史。
- image-quality worker 捕获处理异常并回写 `retryable_error`。
- OCR worker 对下载异常写 retryable failure，对空结果写 terminal failure。
- 增加 `EDUGRADE_OCR_LEASE_SECONDS` 和 Compose/env 配置。
- 增加敏感 payload 拒绝测试、source adapter 防绕过测试、heartbeat 续租测试和人工 requeue 测试。

## Approval

结论：Approved。

验证时间：2026-07-10。

已通过：

- `go test ./...`
- `go vet ./internal/workerruntime ./internal/ocr ./internal/imagequality`
- OCR worker：`python -m pytest -q`，9 passed。
- image-quality worker：`python -m pytest -q`，5 passed。
- `npm.cmd run check:story049`
- `npm.cmd run check:story050`
- `npm.cmd run check:story051`
- `docker compose ... --profile ocr --profile quality config --quiet`

保留风险：

- 本机 Docker daemon 未启动，因此未在容器 PostgreSQL 中实际应用 `000023` migration；SQL 结构、约束线索和 Compose 配置已静态验证。该项必须在 STORY-052 预生产 Runbook 验收中实际执行并留证。
- source record 与 runtime task 当前由同一 HTTP 请求顺序创建，但还不是同一个数据库 transaction。创建失败会显式返回错误，不会假报成功；后续应在引入 River transactional enqueue 或专用 transaction coordinator 时收敛。
- River/Temporal 均未接入，不能在部署材料中宣称已经使用。

## References

- [River documentation](https://riverqueue.com/docs)
- [River GitHub](https://github.com/riverqueue/river)
- [Temporal documentation](https://docs.temporal.io/)
- [Asynq GitHub](https://github.com/hibiken/asynq)
- [NATS JetStream documentation](https://docs.nats.io/nats-concepts/jetstream)
