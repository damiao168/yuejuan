# STORY-063：主观题 AI 影子批处理与人工复核衔接

状态：In Progress（幂等与持久化运行记录切片已完成）

## 背景

当前主观题接口已经能够加载答题片段、Rubric 和受治理的本地 Grading Agent，并将结果落入 `ai_grade`。但请求 ID 在 HTTP Handler 中每次随机生成，重复点击、网络重试或未来 Worker 重试可能重复调用模型并写入多条建议；同时还没有批次级进度、任务租约和失败重试记录。

## 本 Story 范围

1. 建立主观题影子评分的批次/任务编排，使任务只携带答题片段 ID 和版本元数据，不携带学生答案全文。
2. 通过受治理的内部 Grading Agent 生成建议，结果只能进入 `ai_grade` 和人工复核队列，不得直接改写最终分。
3. 以答案版本、Rubric 版本、模型/提示词版本和幂等键组成稳定请求身份；重试必须幂等，版本变化必须隔离。
4. 管理员可看到批次进度、失败与重试；阅卷老师在现有工作台看到建议、证据和人工确认入口。

## 非范围

- 本 Story 不接入外部厂商 API；外部厂商仍受 STORY-061 的原生适配、沙箱评测和发布门禁约束。
- 本 Story 不做模型微调，不把 AI 建议写入最终成绩，不替代双评/仲裁。
- 本 Story 不宣称真实打印、扫描或 Production Ready；这些仍由 STORY-060/067 的现场验证和发布门禁覆盖。

## 首个实现切片

先实现单答题片段建议生成的幂等协议：调用方可提供 `idempotency_key`，服务端将其与租户、片段、答案版本、Rubric、模型和提示词版本绑定，重复请求返回同一 `ai_grade`；同一请求身份出现不同结果时返回冲突，不产生第二条评分事实。该切片为后续 Worker 租约、批次重试和断点续跑提供可验证基础。

当前已进一步落地 `subjective_grading_run` 持久化源记录：每次建议生成先进入 `processing`，成功、模型失败和输出校验失败分别更新为终态，并将运行 ID 关联到 `ai_grade`。该记录仍不代表最终成绩。

Worker Runtime 已将 `subjective_grading_run` 纳入领域源激活保护；在主观题结果回调实现前，通用 Worker 完成/失败接口会拒绝直接结束该任务。

现已增加内部领域回调：`POST /api/v1/internal/subjective-grading/runs/{runId}/result` 和 `/failure`。回调会校验任务归属、租约、请求 ID 以及模型/Rubric/提示词版本；成功结果先幂等落库，再完成 Worker 任务并更新运行终态。

新增批次记录接口：`POST /api/v1/subjective-grading-batches` 创建仅包含答题片段 ID 的批次，`GET /api/v1/subjective-grading-batches/{batchId}` 查询进度计数。相同幂等键但片段集合变化会返回冲突；当前批次状态为 `planned`，尚未声称已创建 Worker 任务。

现已增加 `POST /api/v1/subjective-grading-batches/{batchId}/enqueue`：按受治理模型策略为每个片段创建 `subjective_grading_run` 和 `ai_grade` Worker Runtime 任务，任务只含 ID/版本元数据；重复入队复用 Worker Runtime 幂等键。

新增可选 Compose Profile `subjective-grading` 和独立 `services/subjective-grading-worker` Python Worker。Worker 领取单个任务、续租、调用 API Gateway 的受保护执行边界，再提交领域结果或失败；模型调用仍由 API Gateway 连接的受治理 Grading Agent 执行，Worker 不直接接触未授权模型凭据。

管理端新增 `/grading/subjective-batches` 页面，可创建批次、入队、刷新进度并查看失败计数；该页面只面向具备 `grading:manage` 的管理员。

## 验收证据

- 同一幂等键重复请求不会重复调用 Adapter，也不会产生第二条 `ai_grade`。
- 每条建议都有对应的 `subjective_grading_run` 身份和终态；运行记录重复创建保持幂等。
- Worker 结果回调必须通过租约与版本校验，成功后任务和领域运行记录均收敛。
- 批次创建限制 1～1000 个不重复答题片段，并验证每个片段已有答案、Rubric 和受支持题型。
- 批次入队后进度计数反映实际 Worker 任务状态，重复入队不产生重复任务。
- Worker 配置缺失或租约参数不安全时启动即拒绝；Worker Runtime 源类型和领域回调保持不变。
- 未知 JSON 字段被拒绝；幂等键格式错误被拒绝。
- 同一身份的不同持久化结果返回冲突；答案/Rubric/模型/提示词版本变化生成新的请求身份。
- 现有主观题人工复核策略、审计和受治理模型策略保持不变。
- Go 单元测试、API Gateway 相关回归和前端既有检查通过。

## 后续切片

完成幂等基础后，再增加 `subjective_grading_run` 源记录、`subjective-grading` Worker 队列、结果回调和管理员进度视图；每一切片都必须保留版本校验和人工复核边界。

## 2026-08-01 批次进度与联调门禁补强

- 批次创建的运行记录现在先进入 `queued`，只有持有有效 Worker 租约的执行请求才能转为 `processing` 并调用受治理 Grading Agent。
- 执行边界同时校验任务归属、租约令牌和任务状态，过期、伪造或已经终止的租约返回冲突，不触发模型调用。
- 管理端刷新批次时，后端会依据持久化运行记录重新汇总排队、处理中、成功和失败数量；批次全部成功才进入 `completed`，存在最终失败则进入 `failed`。
- 新增 `npm run check:story063`，固定检查路由、迁移、租约保护、Worker、Compose 配置、管理端权限和真实联调入口。
- 新增 `infra/docker-compose/scripts/story063-subjective-smoke-test.ps1`，用于在已有真实答题片段上创建批次、入队并轮询至终态。脚本不会输出密码或访问令牌。
- 当时机器 Docker Desktop 未启动，因此该次记录只证明代码回归、Compose 静态解析和联调脚本语法；它不是当前环境状态，也不能据此宣称生产验收通过。

## 2026-08-09 Worker 自动门禁与批次可靠性整改

- Python CI 已纳入主观题 Worker 的 editable 安装、Ruff、pytest 和依赖检查。
- Compose CI 已纳入 `subjective-grading` profile、镜像构建和配置烟测；Worker 增加运行健康标记、断线重登和本地 HTTP 协议烟测。
- 管理页已收敛幂等键生命周期、非重叠轮询和可见错误；后端批量加载片段上下文，并以结构化结果报告部分成功。
- 本机 Worker `5` 项测试、Ruff、STORY-063 门禁和 Compose 配置检查通过；随后隔离 Compose 已验证主观题批次、专用 Worker 服务账号、租约/执行、模型协议适配、AI 建议、审计和数据库收敛，Worker 健康检查通过。该结果仍不证明真实模型效果、设备或学校现场能力。

当前状态、验证强度和未覆盖边界统一见 [`docs/verification-status.md`](../verification-status.md)。
