# STORY-047 完成记录

## Story

数据库级多租户约束硬化。

## 规格

- 已明确本轮关闭数据库层跨租户父子引用写入风险。
- 已明确采用 PostgreSQL `(tenant_id, id)` 复合唯一约束和复合外键。
- 已明确本轮不启用全量 RLS，不处理 JSONB 内部 id 引用，不接入 `lab/`。

## 规格审阅与修改

- 规格审阅后放弃首轮全量 RLS，避免引入连接池 session 变量、管理员绕过和大范围查询重构。
- 规格审阅后保留旧单列外键，避免破坏既有迁移和 store 假设。
- 规格审阅后要求迁移前检查跨租户脏数据，发现即失败，不静默修复。

## 实现审阅

- 新增 `000020_story047_tenant_constraints.sql`。
- 新增核心父表 `(tenant_id, id)` 唯一约束。
- 新增核心子表 `(tenant_id, fk)` 复合外键。
- 新增迁移静态覆盖测试。
- 修复 Postgres E2E SQL splitter 对 dollar-quoted block 的支持。
- 修复 Postgres E2E 种子数据 UUID 参数类型推断。
- 修复 score Postgres store 在同一事务中未关闭父 `Rows` 就查询子 items 导致真实 PostgreSQL bad connection 的问题。

## 测试

- `go test ./internal/auth -run TestStory047MigrationAddsTenantScopedDatabaseConstraints -count=1`
- `go test ./internal/server -run TestE2ESplitSQLStatementsKeepsDollarQuotedBlocksTogether -count=1`
- 临时 `postgres:16-alpine` 容器完整执行 `services/api-gateway/migrations/*.sql`
- 临时 `postgres:16-alpine` 容器执行直接 SQL 跨租户负例，验证 `user_role`、`exam`、`exam_paper` 跨租户写入被拒绝
- `EDUGRADE_E2E_DATABASE_URL=postgres://... go test ./internal/server -run TestCoreWorkflowE2EWithPostgresTestDatabase -count=1`
- `go test ./...`

## 结论

STORY-047 批准。项目可以继续进入 STORY-048：Web mock 页面退场与生产菜单收敛。

## 剩余风险

- 本轮主要阻止数据库写入污染，不替代 API 读路径权限校验。
- JSONB 内部 id 引用仍需后续业务校验策略处理。
- RLS 仍可作为后续纵深防护评估，不阻塞当前生产化路线。
