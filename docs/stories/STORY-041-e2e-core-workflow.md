# STORY-041 全链路 E2E 测试

## Plan

### 目标

为 EduGrade Enterprise 建立核心业务全链路 E2E 测试，覆盖从管理员登录、组织与考试配置、试卷与答卷上传、OCR/切分/AI mock、证据校验、人工复核、成绩生成确认发布、学生查分、申诉处理到审计日志查看的主要闭环。

本 Story 的重点不是新增业务能力，而是把已经实现的业务模块用可复现的 synthetic 测试串起来，暴露跨模块状态、权限和质量门禁问题。

### 范围

1. 在 API Gateway 增加核心业务 E2E 测试。
2. 测试数据必须显式使用 synthetic 数据，不包含真实学校、真实学生、真实答卷或真实阅卷内容。
3. 覆盖完整正向流程：
   - 管理员登录。
   - 创建租户、学校、年级、班级、学生。
   - 创建考试。
   - 上传试卷文件。
   - 创建试卷、题目和 Rubric。
   - 上传答卷文件。
   - 创建答卷、添加答卷页、质量检查并进入 OCR 准备状态。
   - 创建 OCR mock 任务、启动、提交 OCR mock 结果。
   - 触发答案切分。
   - 记录切分答案并触发主观题 AI mock 阅卷。
   - 触发证据校验。
   - 创建人工复核任务。
   - 教师提交 `human_grade`。
   - 生成 `final_grade`。
   - 确认成绩。
   - 发布成绩。
   - 学生查看已发布成绩。
   - 学生提交申诉。
   - 教师处理申诉。
   - 查看审计日志。
4. 覆盖关键反向路径：
   - 权限不足返回 403。
   - 未完成阅卷不能发布。
   - 越权查看其他学生成绩失败。
   - 人工分数超过满分失败。
5. 为测试数据库场景提供可执行入口：
   - 默认单元测试使用 `httptest` 和 synthetic in-memory stores，保证日常 CI 可稳定执行。
   - PostgreSQL 集成 E2E 使用 `EDUGRADE_E2E_DATABASE_URL` 显式启用，连接测试数据库并应用 migrations 后运行。
6. 输出运行命令和验收记录。

### 非范围

1. 不实现真实 OCR 引擎、真实 LLM 推理或 Agent worker runtime。
2. 不把 mock OCR、mock AI 阅卷或 synthetic 数据包装成真实模型能力。
3. 不启动完整 Docker 私有化环境做浏览器级 Web E2E；本 Story 聚焦 API 核心业务链路。
4. 不引入 testcontainers 或其他新第三方依赖，避免加重 Windows 本地和私有化 CI 的运行要求。
5. 不提前进入 STORY-042 企业级验收文档、STORY-043 上线前代码审查或 STORY-044 自动修复。

### 预计新增文件

1. `services/api-gateway/internal/server/e2e_core_workflow_test.go`
2. `services/api-gateway/internal/server/e2e_postgres_test.go`
3. `docs/stories/STORY-041-e2e-core-workflow.md`
4. `docs/stories/STORY-041-approval.md`

### 预计修改文件

1. `docs/stories/README.md`
2. `services/api-gateway/internal/org/store_memory.go`，仅在确有必要时补齐测试友好的 synthetic UUID 学生 ID 支撑。
3. `services/api-gateway/internal/server/server_test.go` 或相邻测试 helper，按需复用认证与请求断言逻辑。
4. `tests/README.md`，补充 E2E 运行命令。

### 验收标准

1. 存在一条可由 `go test` 默认运行的 synthetic HTTP E2E，逐步断言核心状态。
2. 存在测试数据库 E2E 入口，只有设置 `EDUGRADE_E2E_DATABASE_URL` 时才运行；未设置时明确 skip，不冒充已运行数据库测试。
3. 正向流程覆盖用户要求的 19 个步骤，并在每一步断言关键状态、ID 或审计事件。
4. 反向路径覆盖权限不足、未完成阅卷发布、越权查分、分数超过满分。
5. 所有 mock 行为在测试名、变量或断言中标注为 `synthetic` / `mock`。
6. 给出本地运行命令、测试数据库运行命令和不可运行原因记录。
7. `go test ./...` 通过，或明确记录失败原因并修正后再审批。

## Plan Review

1. 范围没有越过 STORY-041：只新增 E2E 测试、测试支撑和运行说明，不新增真实 OCR/LLM/worker 能力。
2. 显式需求已全部纳入：完整 19 步流程、测试数据库、synthetic 数据、逐步断言、四类负向路径和运行命令均在计划中。
3. 与当前仓库实际匹配：
   - API Gateway 已有完整 HTTP route 和 memory store 测试体系，适合新增默认可跑的 `httptest` E2E。
   - 生产 `server.New` 依赖 PostgreSQL、Redis、MinIO、Qdrant、AI service；测试数据库 E2E 应该显式用 DSN 启用，不能在没有数据库时假装通过。
   - 当前 `submission` 要求 `student_id` 为 UUID 格式，而 `org.MemoryStore` 默认生成 `student-1` 这类 ID；实现时要么补齐测试 UUID 支撑，要么在 E2E 中诚实隔离该限制并使用 PostgreSQL 路径验证真实 UUID ID。
4. 风险控制：
   - 默认 E2E 保持稳定，不依赖 Docker daemon。
   - 测试数据库 E2E 通过环境变量 gate，避免在开发机无数据库时造成全量测试失败。
   - mock OCR/AI 明确标识为 mock，不把当前 placeholder 能力误写成真实推理能力。
5. 审阅结论：计划可执行，进入 Implementation。

## Implementation

### 默认 Synthetic HTTP E2E

新增 `services/api-gateway/internal/server/e2e_core_workflow_test.go`。

该测试使用 `httptest` 驱动真实 HTTP router，并使用 synthetic memory stores 保持日常 CI 稳定。覆盖完整业务链路：

1. 管理员登录。
2. 创建租户、学校、年级、班级、学生。
3. 创建考试。
4. 上传 synthetic 试卷文件。
5. 创建试卷、题目和 approved Rubric。
6. 上传 synthetic 答卷文件。
7. 创建答卷、添加页、质量检查、进入 `ready_for_ocr`。
8. 创建、启动并完成 `mock_ocr` 任务。
9. 触发答案切分并断言生成 1 个 `answer_segment`。
10. 记录 OCR 答案文本。
11. 触发 `mock-llm` 主观题 AI 阅卷，并断言 `mock=true`、`needs_human_review=true`。
12. 触发证据校验，并断言需要人工复核。
13. 创建人工复核任务。
14. 教师提交 `human_grade`。
15. 生成 `final_grade`。
16. 确认成绩。
17. 发布成绩。
18. 学生查看成绩。
19. 学生提交申诉，教师处理申诉。
20. 查询审计日志并断言关键审计动作存在。

同时覆盖负向路径：

1. 权限不足创建考试返回 403。
2. 未完成阅卷发布成绩返回 409，并包含 `unfinished_review_tasks`。
3. 学生越权查看其他学生成绩返回 403。
4. 人工分数超过题目满分返回 400。
5. 成绩未确认前发布返回 409。

### PostgreSQL 测试数据库 E2E

新增 `services/api-gateway/internal/server/e2e_postgres_test.go`。

该测试只在 `EDUGRADE_E2E_DATABASE_URL` 存在时运行，行为如下：

1. 连接 PostgreSQL 测试库。
2. 应用 `services/api-gateway/migrations/*.sql`。
3. 使用 PostgreSQL stores 构建 router。
4. 复用核心 synthetic E2E 业务流。
5. 使用 memory object storage，避免测试依赖真实 MinIO。
6. 显式 seed 学生用户 data scope，用于真实 RBAC/对象级权限验证。

未设置 `EDUGRADE_E2E_DATABASE_URL` 时测试会 `skip`，避免在没有测试库的开发机上伪造数据库 E2E 通过。

### 权限与测试支撑修复

1. 新增 `services/api-gateway/internal/auth/scope.go`：
   - 支持从顶层 `data_scope.student_id` 读取学生 ID。
   - 支持从 PostgreSQL RBAC 查询返回的角色嵌套 data scope 中读取学生 ID，例如 `{"student":{"scope":"self","student_id":"..."}}`。
2. 更新 `score`、`appeal`、`report` handler 的学生 scope 解析，统一使用 `auth.ScopedStudentID`。
3. 新增 `services/api-gateway/internal/auth/scope_test.go`，覆盖 flat 与 role-keyed 两类 data scope。
4. 调整 `org.MemoryStore.CreateStudent`，允许测试传入 synthetic UUID 学生 ID；未传时保持旧行为。

### 文档

更新 `tests/README.md`，补充：

1. 默认 synthetic HTTP E2E 运行命令。
2. PostgreSQL 测试数据库 E2E 环境变量和运行命令。
3. 未配置 DSN 时数据库 E2E 会 skip 的说明。

## Implementation Review

### 验收检查

| 验收项 | 结果 | 证据 |
| --- | --- | --- |
| 默认 synthetic HTTP E2E 存在并可运行 | 通过 | `TestCoreWorkflowE2EWithSyntheticMemoryStores` |
| 覆盖 19 步核心业务流程 | 通过 | 测试从登录一路断言到审计日志 |
| 覆盖权限不足 | 通过 | `e2e_limited` 创建考试返回 403 |
| 覆盖未完成阅卷不能发布 | 通过 | unfinished review task 返回 409 |
| 覆盖越权查看成绩失败 | 通过 | other student 查询本人外成绩返回 403 |
| 覆盖分数超过满分失败 | 通过 | 教师提交 6/5 返回 400 |
| mock 行为显式标记 | 通过 | `mock_ocr`、`mock-llm`、`mock=true` 断言 |
| 测试数据库入口 | 通过但当前环境未执行 | `TestCoreWorkflowE2EWithPostgresTestDatabase`，需 DSN |
| 运行命令 | 通过 | `tests/README.md` |

### 发现的问题

1. PostgreSQL RBAC 返回的 `data_scope` 是角色嵌套结构，原 handler 只读取顶层 `student_id`，会导致真实数据库下学生本人查分/申诉失败。
2. `org.MemoryStore` 默认生成 `student-1`，与 `submission` 对 `student_id` 的 UUID 校验不一致，会阻断真实 HTTP E2E 的“创建学生 -> 创建答卷”链路。
3. 当前本机未配置 `EDUGRADE_E2E_DATABASE_URL`，Docker daemon 不可用，也没有本地 PostgreSQL/psql，无法实际执行数据库 E2E。

## Fixes

1. 新增 `auth.ScopedStudentID`，兼容 flat 与 role-keyed data scope。
2. `score`、`appeal`、`report` handler 改用统一 scope helper。
3. `org.MemoryStore.CreateStudent` 支持测试传入 synthetic UUID。
4. PostgreSQL E2E 测试明确使用环境变量 gate，未配置时只 skip，不把未运行伪装为通过。

## Approval

结论：批准。

批准理由：

1. 默认可运行的 synthetic HTTP E2E 已覆盖 STORY-041 要求的核心链路和负向路径。
2. 测试数据库 E2E 入口已实现，使用真实 PostgreSQL stores、迁移文件和 synthetic 数据。
3. 与数据库 E2E 相关的真实权限 bug 已修复并增加单元测试。
4. 当前无法执行 PostgreSQL E2E 的原因来自本机外部环境：未设置 `EDUGRADE_E2E_DATABASE_URL`、Docker daemon 未运行、未发现本地 PostgreSQL。该限制已记录为剩余风险，并提供了运行命令。

### 运行命令

```powershell
Set-Location services\api-gateway
go test ./internal/auth ./internal/server -run "TestScopedStudentIDSupportsFlatAndRoleKeyedScopes|TestCoreWorkflowE2E" -count=1
go test ./...
go test ./internal/server -run TestCoreWorkflowE2EWithPostgresTestDatabase -count=1 -v
```

测试数据库执行命令：

```powershell
Set-Location services\api-gateway
$env:EDUGRADE_E2E_DATABASE_URL='postgres://edugrade:edugrade@127.0.0.1:5432/edugrade_e2e?sslmode=disable'
go test ./internal/server -run TestCoreWorkflowE2EWithPostgresTestDatabase -count=1 -v
```

### 测试结果

1. `go test ./internal/auth ./internal/server -run "TestScopedStudentIDSupportsFlatAndRoleKeyedScopes|TestCoreWorkflowE2E" -count=1`：通过。
2. `go test ./...`：通过。
3. `go test ./internal/server -run TestCoreWorkflowE2EWithPostgresTestDatabase -count=1 -v`：通过但测试 skip，原因是 `EDUGRADE_E2E_DATABASE_URL` 未设置。
4. `docker version`：Docker client 存在，但 daemon 未运行，无法临时启动测试数据库。

### 剩余风险

1. PostgreSQL E2E 入口已实现，但当前机器没有实际执行数据库链路；需要在具备测试库的 CI 或本地环境补跑。
2. 默认 E2E 使用 memory stores，能覆盖 HTTP 合约和跨模块状态，但不能替代 PostgreSQL 约束、索引和事务级验证。
3. OCR 与主观 AI 仍是 mock，不代表真实 OCR/LLM 推理能力。

### 下一步建议

进入 STORY-042：企业级验收文档。
