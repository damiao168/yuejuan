# STORY-041 Approval

结论：批准。

## 修改摘要

1. 新增核心业务 synthetic HTTP E2E，覆盖登录、组织、考试、试卷、答卷、OCR mock、答案切分、AI mock、证据校验、人工复核、成绩确认发布、学生查分、申诉处理和审计日志。
2. 新增 PostgreSQL 测试数据库 E2E 入口，使用 `EDUGRADE_E2E_DATABASE_URL` 显式启用。
3. 修复真实 PostgreSQL RBAC `data_scope` 嵌套结构下学生本人查分/申诉 scope 解析问题。
4. 调整 memory org store，支持测试传入 synthetic UUID 学生 ID。
5. 补充 E2E 运行命令文档。

## 新增文件

1. `services/api-gateway/internal/server/e2e_core_workflow_test.go`
2. `services/api-gateway/internal/server/e2e_postgres_test.go`
3. `services/api-gateway/internal/auth/scope.go`
4. `services/api-gateway/internal/auth/scope_test.go`
5. `docs/stories/STORY-041-e2e-core-workflow.md`
6. `docs/stories/STORY-041-approval.md`

## 修改文件

1. `docs/stories/README.md`
2. `tests/README.md`
3. `services/api-gateway/internal/org/store_memory.go`
4. `services/api-gateway/internal/score/handlers.go`
5. `services/api-gateway/internal/appeal/handlers.go`
6. `services/api-gateway/internal/report/handlers.go`

## 运行命令

```powershell
Set-Location services\api-gateway
go test ./internal/auth ./internal/server -run "TestScopedStudentIDSupportsFlatAndRoleKeyedScopes|TestCoreWorkflowE2E" -count=1
go test ./...
go test ./internal/server -run TestCoreWorkflowE2EWithPostgresTestDatabase -count=1 -v
```

测试数据库命令：

```powershell
Set-Location services\api-gateway
$env:EDUGRADE_E2E_DATABASE_URL='postgres://edugrade:edugrade@127.0.0.1:5432/edugrade_e2e?sslmode=disable'
go test ./internal/server -run TestCoreWorkflowE2EWithPostgresTestDatabase -count=1 -v
```

## 测试结果

1. 定向测试：通过。
2. `go test ./...`：通过。
3. PostgreSQL E2E：当前环境未设置 `EDUGRADE_E2E_DATABASE_URL`，测试明确 skip。
4. Docker：client 存在，但 daemon 未运行，无法临时启动测试库。

## 剩余风险

1. PostgreSQL E2E 已实现但未在当前机器实际执行；需在有测试数据库的 CI 或本地环境补跑。
2. 默认 E2E 使用 memory stores，不能替代数据库事务和约束验证。
3. OCR 与 AI 仍是 mock，不能代表真实推理能力。

## 下一步建议

进入 STORY-042：企业级验收文档。
