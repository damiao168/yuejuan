# STORY-018 自审审批记录

## Story

STORY-018 人工复核与阅卷工作流

## 审批结论

Approved

## 验收项

| 验收项 | 证据 | 结论 |
| --- | --- | --- |
| 可创建人工复核任务 | `TestReviewTaskWorkflowCreatesAssignsSubmitsAndReturns`、路由测试覆盖 | 通过 |
| 支持任务来源枚举校验 | `TestCreateTaskRejectsInvalidSource` 覆盖 | 通过 |
| 可查询任务列表和详情 | `TestReviewTaskRoutesCreateAssignSubmitAndAudit` 覆盖 | 通过 |
| 可分配阅卷员 | store 和 route 测试覆盖 | 通过 |
| 支持批量分配 | `TestBatchAssignTasks`、`TestReviewTaskBatchAssignRoute` 覆盖 | 通过 |
| 阅卷员只能提交分配给自己的任务 | `TestReviewerCanOnlySubmitAssignedTask` 覆盖 | 通过 |
| 分数越界会失败 | `TestSubmitGradeValidatesScoreAndRubricSelections` 覆盖 | 通过 |
| Rubric 选择不匹配会失败 | 同一测试覆盖 | 通过 |
| 提交 human_grade 后更新任务状态 | store 和 route 测试覆盖 `submitted` | 通过 |
| 可退回重评并记录 reason | store 和 route 测试覆盖 `return_reason` | 通过 |
| 接口受权限保护 | `review:manage`、401、403 测试覆盖 | 通过 |
| 创建、分配、批量分配、提交、退回写审计 | 路由测试断言 audit action | 通过 |
| 查询结果包含 anonymous_code 且默认不暴露学生姓名 | 路由测试断言 `anonymous_code` 且不包含 `student_name` | 通过 |
| 不越界 | 未实现 Web、双评仲裁、final_grade、成绩发布 | 通过 |

## 运行命令与结果

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

## 剩余风险

- 当前是后端 API 基础闭环，Web 阅卷工作台尚未实现。
- 当前 `review:manage` 是统一权限，后续可拆分为分配、阅卷、审计等更细权限。
- 当前不生成 final_grade，不做双评分差比较和仲裁。
- `private_note` 的展示侧权限隔离需要在后续查询和前端 Story 中继续收敛。

## 下一步

进入 `STORY-019 双评与仲裁模块`。
