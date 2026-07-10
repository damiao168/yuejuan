# Tests

测试目录。

- `e2e`：核心业务链路。
- `load`：性能和容量。
- `security`：权限、租户隔离、文件安全和日志安全。
- `ai-evaluation`：AI 建议分质量评估，使用 synthetic 数据集与离线评估脚本，不使用真实学生隐私数据。

当前阶段按 Story 分步补齐测试实现，不提前进入未规划 Story。

## STORY-041 API E2E

核心 API E2E 位于：

```text
services/api-gateway/internal/server/e2e_core_workflow_test.go
services/api-gateway/internal/server/e2e_postgres_test.go
```

默认 synthetic HTTP E2E：

```powershell
Set-Location services\api-gateway
go test ./internal/server -run TestCoreWorkflowE2EWithSyntheticMemoryStores -count=1
```

测试数据库 E2E 需要显式提供 PostgreSQL 测试库 DSN；未设置时测试会 skip，不冒充已运行数据库链路：

```powershell
Set-Location services\api-gateway
$env:EDUGRADE_E2E_DATABASE_URL='postgres://edugrade:edugrade@127.0.0.1:5432/edugrade_e2e?sslmode=disable'
go test ./internal/server -run TestCoreWorkflowE2EWithPostgresTestDatabase -count=1 -v
```
