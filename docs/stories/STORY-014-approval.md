# STORY-014 自审审批记录

## Story

STORY-014 多智能体 Orchestrator

## 审批结论

Approved

## 验收项

| 验收项 | 证据 | 结论 |
| --- | --- | --- |
| 创建 orchestration run | `TestOrchestrationLifecycleLowConfidenceReviewAndAudit` 覆盖 | 通过 |
| 创建受控 agent task | 同一测试创建 `objective_grading_agent` | 通过 |
| 非法类型拒绝 | `TestInvalidOrchestrationInputsRejected` 覆盖 | 通过 |
| task 状态流转受控 | start/complete、complete before start 测试覆盖 | 通过 |
| failed task retry 受控 | `TestAgentTaskFailRetryAndRetryLimit` 覆盖 | 通过 |
| output/evidence/confidence 保存 | complete 请求和响应断言覆盖 | 通过 |
| 低置信度人工复核 | `confidence=0.52` 进入 `requires_human_review` | 通过 |
| 显式人工复核 | `TestExplicitHumanReviewFlagForcesReview` 覆盖 | 通过 |
| 权限保护 | `orchestrator:manage`、401、403 测试覆盖 | 通过 |
| 审计 | run/task/start/complete/fail/retry audit 测试覆盖 | 通过 |
| 系统能力声明 | `system/info` 增加 Orchestrator 控制面能力 | 通过 |
| API 文档 | `docs/api/orchestrator.md` 已新增 | 通过 |
| 不越界 | 未实现真实 Agent worker、模型调用、Prompt 管理和评分生成 | 通过 |

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

- 当前只提供 Orchestrator 控制面，不包含真实 Agent worker runtime。
- 不调用模型，不生成 OCR、评分或证据判断结果。
- 后续 Story 需要在该控制面上实现客观题/填空题判分、主观题 AI 评分接口和证据校验。

## 下一步

进入 `STORY-015 客观题与填空题判分`，实现规则型评分能力，并继续坚持 AI/模型能力未接入时不伪造结果。
