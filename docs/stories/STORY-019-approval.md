# STORY-019 自审审批记录

## Story

STORY-019 双评与仲裁模块

## 审批结论

Approved

## 验收项

| 验收项 | 证据 | 结论 |
| --- | --- | --- |
| 可配置考试级 double_mark policy | `SetExamDoubleMarkPolicy`、路由测试 | 通过 |
| 可配置题目级 double_mark policy，题目级优先 | `SetQuestionDoubleMarkPolicy`、`TestQuestionDoubleMarkPolicyOverridesExamPolicy` | 通过 |
| 同一 answer_segment 生成两个独立 review_task | `CreateDoubleMarkSession`、store/route 测试 | 通过 |
| 两个 review_task 分配给不同阅卷员 | Store 输入校验 | 通过 |
| 双盲期间普通 review_task API 不暴露对方评分 | `TestDoubleMarkAndArbitrationRoutes` | 通过 |
| 两个评分完成后自动比较分差 | Store 自动状态机 | 通过 |
| 分差不超过阈值时生成 final_grade | `TestDoubleMarkAutoFinalizesByResolutionStrategy` | 通过 |
| 支持五种合分策略 | `average`、`first`、`second`、`higher`、`lower` 子测试 | 通过 |
| 分差超过阈值时创建 arbitration_task | store/route 测试 | 通过 |
| 仲裁员可查看双评分数和上下文 | arbitration route 测试 | 通过 |
| 仲裁员默认不能是前两个阅卷员之一 | store/route 测试 | 通过 |
| 配置允许时可同人仲裁 | `TestAllowSameArbitratorPolicyAllowsReviewerAsArbitrator` | 通过 |
| 仲裁提交写 final_grade | store/route 测试 | 通过 |
| 全流程审计 | 路由测试断言 audit action | 通过 |
| 权限保护 | `review:manage`、`arbitration:manage` 与 403 测试 | 通过 |
| 不越界 | 未实现 Web 页面、成绩发布审批、申诉、真实 OCR/AI 聚合 | 通过 |

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

- `final_grade` 当前只是后端记录，尚未实现整场考试成绩确认、发布、锁定、导出和学生查看。
- 仲裁上下文字段只返回当前已有答案文本和 AI 摘要；真实 OCR/AI 聚合仍未实现。
- 还没有 Web 仲裁页面；前端页面留给后续 Web Story。
- 未实现三评、多仲裁员投票和阅卷员质量分析。

## 下一步

进入 `STORY-020 最终成绩与成绩发布`。
