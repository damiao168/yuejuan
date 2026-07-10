# STORY-014 多智能体 Orchestrator

## 状态

Approved

## 目标

建立受控 Agent 工作流编排基础：创建 orchestration run、拆分 agent task、记录输入/输出引用、状态流转、失败重试和人工复核触发。为后续 OCR Agent、证据校验 Agent、评分 Agent、一致性 Agent 等提供统一任务控制面。

## Plan

- 新增 migration：
  - `orchestration_run`
  - `agent_task`
  - `orchestrator:manage` 权限。
- 新增 `internal/orchestrator`：
  - 类型定义和状态机。
  - Store interface。
  - MemoryStore。
  - PostgresStore。
  - Handler。
- 接口：
  - `POST /api/v1/orchestrations`
  - `GET /api/v1/orchestrations/{id}`
  - `GET /api/v1/orchestrations/{id}/tasks`
  - `POST /api/v1/orchestrations/{id}/tasks`
  - `POST /api/v1/agent-tasks/{id}/start`
  - `POST /api/v1/agent-tasks/{id}/complete`
  - `POST /api/v1/agent-tasks/{id}/fail`
  - `POST /api/v1/agent-tasks/{id}/retry`
- 规则：
  - 所有接口要求 `orchestrator:manage`。
  - Orchestrator 只编排，不执行真实 AI。
  - agent_type 必须受控。
  - task 状态受控：`queued -> running -> succeeded/failed/requires_human_review`。
  - failed task 可在最大次数内 retry。
  - 低置信度或 worker 主动标记可进入 `requires_human_review`。
  - 保存 input_ref/output_ref/evidence_ref JSON，不能把学生答案大文本写入普通日志。
  - 所有关键动作写审计。

## Plan Review

- 不越界：不实现真实 Agent 推理，不生成评分，不生成 OCR，不做证据判断。
- 不做假功能：worker 输出只能通过接口回写；系统不编造 agent result。
- 与前序 Story 衔接：target 可以引用 exam、submission、answer_segment、ocr_task。
- 与后续 Story 衔接：客观题/主观题/证据校验 Agent 都将复用 agent_task。

## 非范围

- 真实模型调用。
- Prompt 管理。
- Agent worker 进程。
- 评分逻辑。
- 自动任务调度 DAG。

## 验收标准

- 可以创建 orchestration run。
- 可以创建受控类型的 agent task。
- 非法 agent_type 被拒绝。
- task 状态流转受控。
- failed task 可 retry 且 retry 次数受控。
- complete 可以保存 output_ref/evidence_ref/confidence。
- 低置信度或人工复核标记会进入 `requires_human_review`。
- 所有接口受 `orchestrator:manage` 权限保护。
- 创建 run、创建 task、start、complete、fail、retry 写审计。
- 测试通过。

## Implementation

- 新增 `000009_orchestrator.sql`：
  - `orchestrator:manage` 权限。
  - `orchestration_run` 表。
  - `agent_task` 表。
  - run、task、status 索引。
- 新增/补齐 `internal/orchestrator`：
  - 类型定义、受控 workflow/target/agent type 校验和状态机。
  - MemoryStore，用于测试和无数据库路由默认值。
  - PostgresStore，用于真实部署数据持久化。
  - HTTP Handler，提供 run/task 控制面接口。
- API Gateway 挂载：
  - `POST /api/v1/orchestrations`
  - `GET /api/v1/orchestrations/{id}`
  - `GET /api/v1/orchestrations/{id}/tasks`
  - `POST /api/v1/orchestrations/{id}/tasks`
  - `POST /api/v1/agent-tasks/{id}/start`
  - `POST /api/v1/agent-tasks/{id}/complete`
  - `POST /api/v1/agent-tasks/{id}/fail`
  - `POST /api/v1/agent-tasks/{id}/retry`
- 更新 `system/info` capabilities：
  - `agent_orchestration_control_plane`
  - `agent_task_management`
  - `agent_task_retry`
  - `agent_human_review_trigger`
- 更新 `system/info` not_implemented：
  - `agent_worker_runtime`
- 新增 `docs/api/orchestrator.md`，更新根 README 和 API Gateway README。

## Implementation Review

逐项检查结果：

- 创建 orchestration run：`POST /api/v1/orchestrations` 可创建并返回 `created` 状态，测试覆盖。
- 创建 agent task：`POST /api/v1/orchestrations/{id}/tasks` 可创建受控 agent task，测试覆盖。
- 非法类型拒绝：非法 workflow_type 和 agent_type 返回 `400 invalid_orchestrator_input`，测试覆盖。
- 状态流转受控：queued/start/complete/fail/retry 均走状态机，complete before start 返回冲突，测试覆盖。
- 失败重试受控：`attempt_no < max_attempts` 才可 retry，耗尽返回 `409 agent_task_retry_exhausted`，测试覆盖。
- output/evidence/confidence：complete 保存 `output_ref`、`evidence_ref`、`confidence`。
- 人工复核触发：`confidence < 0.8` 或 `requires_human_review=true` 均进入 `requires_human_review`，测试覆盖。
- 权限：所有 Orchestrator 接口要求 `orchestrator:manage`；未登录返回 401，权限不足返回 403，测试覆盖。
- 审计：run_created、task_created、task_started、task_completed、task_failed、task_retried 均写 audit，测试覆盖。
- 不越界：未实现真实 Agent worker、模型调用、Prompt 管理、OCR/AI/评分生成。

实现审阅中关注点：

- `input_ref`、`output_ref`、`evidence_ref` 仅保存引用和定位信息，不把学生答案长文本写入普通日志。
- run 状态由 task 状态汇总：无 task 为 `created`，存在 queued/running 为 `running`，失败为 `failed`，低置信或显式复核为 `requires_human_review`，全部成功为 `completed`。
- PostgresStore 使用事务更新 task 与 run 状态，避免 task 已变更但 run 状态未更新。
- 当前目录不是 Git 仓库，无法使用 `git diff --check`；已通过 `gofmt`、测试和完整验证替代检查。

## Fixes

- 补齐显式 `requires_human_review=true` 的测试，避免只覆盖低置信度路径。
- 修正 Postgres confidence 参数写入方式，避免把 `*float64` 直接传给 SQL driver。
- 保持 `NewRouterComplete` 向后兼容，已有测试无需强制传入 Orchestrator Store；真实服务启动时注入 PostgresStore。

## Verification

运行命令：

```powershell
Push-Location .\services\api-gateway
gofmt -w internal
go test ./...
Pop-Location
docker compose --env-file .env.example -f infra\docker-compose\docker-compose.yml config
Push-Location .\services\api-gateway
go build -o ..\..\bin\api-gateway.exe .\cmd\api-gateway
Pop-Location
```

结果：

```text
gofmt -w internal -> passed
go test ./... -> passed
docker compose config -> passed
go build -> passed
```

接口验证证据：

```text
system/info -> capabilities include agent_orchestration_control_plane, agent_task_management, agent_task_retry, agent_human_review_trigger
system/info -> not_implemented includes agent_worker_runtime, ocr_engine_inference, ai_grading
POST /api/v1/orchestrations without token -> 401 covered by TestOrchestratorPermissionDeniedAndUnauthenticated
POST /api/v1/orchestrations without orchestrator:manage -> 403 covered by TestOrchestratorPermissionDeniedAndUnauthenticated
```

## Approval

Approved。

本 Story 满足验收标准，可以进入 `STORY-015 客观题与填空题判分`。
