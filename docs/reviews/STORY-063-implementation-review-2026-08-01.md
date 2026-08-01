# STORY-063 实现审阅（2026-08-01）

## 审阅范围

本次审阅“单答题片段主观题 AI 建议生成幂等性及持久化运行记录”切片，不把 STORY-063 的批次 Worker 目标误报为已完成。

## 实现内容

- `GradeRequest` 增加可选 `idempotency_key`，限制为 1～128 个安全字符。
- Handler 使用严格 JSON 解码：拒绝未知字段和多余 JSON 文档。
- 请求身份由租户、答题片段、答案版本、题目/Rubric 版本、受治理模型/提示词版本、最低置信度和幂等键稳定计算；未提供幂等键时保留兼容的随机请求模式。
- 存储层按 `adapter_request_id` 读取已有事实；重试直接返回既有 `ai_grade`，并标记 `idempotent_replay`，避免再次调用 Adapter。
- Memory Store 和 PostgreSQL Store 均处理并发唯一冲突；同一身份如果对应不同持久化内容返回 409 `subjective_grade_idempotency_conflict`。
- Adapter 失败和模型输出校验失败也使用同一请求身份，重试不会生成第二条失败事实。
- 新增 `subjective_grading_run` 运行记录及 `ai_grade.subjective_grading_run_id` 关联；运行状态从 `processing` 收敛到 `succeeded`、`failed` 或 `conflict`。
- Worker Runtime 将 `subjective_grading_run` 标记为必须走领域结果适配器的源类型，禁止通用 `/complete` 或 `/fail` 绕过评分事实更新。
- 增加 `/internal/subjective-grading/runs/{runId}/result|failure` 领域回调；结果校验请求身份、版本和租约，失败支持重试/死信状态。
- 增加 `subjective_grading_batch` 持久化表及批次创建/查询 API；批次只保存片段 ID，不携带学生答案，当前明确停留在 `planned` 状态。
- 增加批次入队 API，将片段转换为 `subjective_grading_run` Worker 任务，使用版本元数据和 Runtime 幂等键，回写批次队列计数。
- 增加独立 `subjective-grading-worker` Python 服务、Dockerfile 和 Compose Profile；Worker 只负责租约/调用编排，真实模型仍由 API Gateway 的受治理 Agent 执行。
- 管理端新增主观题批次页面、API 封装和受 `grading:manage` 保护的路由，可创建、入队和刷新真实批次进度。

## 验收结果

| 验收项 | 结果 |
| --- | --- |
| 同一幂等键重复请求只调用一次 Adapter | 通过：`TestSubjectiveGradeIdempotencyAvoidsDuplicateAdapterCall` |
| 未知字段/多余 JSON 被拒绝 | 通过：`TestSubjectiveGradeRejectsUnknownFieldsAndInvalidIdempotencyKey` |
| 幂等键格式校验 | 通过 |
| PostgreSQL 唯一冲突回读与冲突错误 | 已实现，使用现有 `uq_ai_grade_adapter_request` |
| 建议事实具备持久化运行身份和终态 | 已实现，迁移 `000063_story063_subjective_grading_runs.sql` |
| 通用 Worker 完成接口不能绕过主观题领域适配器 | 通过：`TestHandlerRequiresSourceAdapterForManagedCompletion/subjective` |
| Worker 结果回调校验版本并完成任务与运行记录 | 通过：`TestSubjectiveWorkerResultValidatesVersionsAndCompletesDomainRun` |
| 批次片段校验与幂等冲突 | 通过：`TestSubjectiveBatchCreationValidatesSegmentsAndIsIdempotent` |
| 批次入队与重复入队不重复创建任务 | 通过：`TestSubjectiveBatchCreationValidatesSegmentsAndIsIdempotent` |
| Worker 领取、心跳、执行、结果提交生命周期 | 通过：`services/subjective-grading-worker/tests/test_worker.py`（3 项） |
| 管理端批次创建、入队和进度展示 | 通过：TypeScript 检查、生产构建和路由门禁 |
| 既有主观题人工复核策略保持不变 | 通过主观题包回归 |

## 测试证据

- `go test ./internal/subjective ./internal/server ./internal/grading ./internal/modelgovernance -count=1` 通过。
- `git diff --check` 通过（仅保留工作树既有 CRLF 提示）。

## 结论

批准该切片进入 STORY-063；Story 整体仍为 In Progress。下一切片应在 Docker/PostgreSQL 启动后完成真实迁移与 Worker 联调，并补充批次按考试/题目筛选，而不是接入未通过治理的第三方 API。

## 追加审查：批次实时进度与可重复联调

- 修复批次进度只在入队时写入、刷新页面仍显示旧计数的问题：查询批次会从持久化运行记录重新汇总状态。
- 批次运行记录不再提前标记为处理中；创建时为 `queued`，通过有效租约进入执行边界后才转为 `processing`。
- 补充执行接口租约令牌和活动状态校验，避免已过期或不属于当前 Worker 的任务触发模型调用。
- 增加 STORY-063 专属静态门禁和真实联调脚本。脚本要求明确传入已有答题片段 UUID，并轮询校验计数守恒与终态，不伪造“真实答题卡”数据。
- 本地完整 Go 回归、Python Worker 测试、前端类型检查与生产构建均通过；Compose 使用仅供静态解析的占位值通过配置检查。
- Docker Desktop 未运行，故真实 PostgreSQL 迁移、容器启动和端到端 Worker 处理证据仍为待办。该限制不应被静态门禁替代。

## 环境补强复核：2026-08-01

- 已通过备用 Go 模块镜像安装 Staticcheck v0.7.0（2026.1），并完成 `services/api-gateway` 全量静态扫描，无告警。
- Docker Desktop 已启动，Docker Engine 29.6.1 可用；现有 PostgreSQL、API Gateway、Grading Agent、网页和既有 Worker 容器均处于 healthy/running 状态。
- `subjective-grading-worker` 镜像已在真实 Docker 环境构建成功。
- 本地 `.env` 尚未配置 `EDUGRADE_SUBJECTIVE_WORKER_TENANT_CODE`、`EDUGRADE_SUBJECTIVE_WORKER_USERNAME`、`EDUGRADE_SUBJECTIVE_WORKER_PASSWORD`。这三个值属于运行凭据，不能用占位值冒充；在账号配置前不启动该 Worker，也不宣称批次闭环已通过。
