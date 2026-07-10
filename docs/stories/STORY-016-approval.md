# STORY-016 自审审批记录

## Story

STORY-016 主观题 AI 评分接口

## 审批结论

Approved

## 验收项

| 验收项 | 证据 | 结论 |
| --- | --- | --- |
| LLMGradingAdapter 接口 | `internal/subjective/types.go` 定义 | 通过 |
| mock 输出固定可测试 | `TestMockSubjectiveGradeCreatesReviewGrade` 覆盖 | 通过 |
| mock 明确标记 | 响应断言 `mock=true`、`mock_llm_subjective` | 通过 |
| 支持主观题类型 | `Store`/Handler 支持 short_answer、calculation、essay、discussion | 通过 |
| model/prompt 记录 | 测试断言 model_version、prompt_version | 通过 |
| 低置信复核 | mock 低置信和固定 adapter 复核测试覆盖 | 通过 |
| essay/discussion 默认复核 | `TestSubjectiveReviewPolicies` 覆盖 essay | 通过 |
| calculation 低 OCR 复核 | 同一测试覆盖 `low_ocr_confidence` | 通过 |
| schema 校验 | `ValidateOutput` 覆盖分数/置信度/raw_output | 通过 |
| 非法输出 failed 落库 | `TestInvalidAdapterOutputCreatesFailedGrade` 覆盖 | 通过 |
| 权限保护 | `grading:manage`、401、403 测试覆盖 | 通过 |
| 审计 | 成功/失败 audit 测试覆盖 | 通过 |
| 不越界 | 未实现真实模型推理、final_grade、证据校验 | 通过 |

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

- 当前只有 mock LLM adapter，不能代表真实模型评分能力。
- evidence 只记录 adapter 输出，尚未做证据真实性校验。
- 主观题建议分不进入 final_grade，仍需后续人工复核流程。

## 下一步

进入 `STORY-017 证据校验 Agent`，校验 AI 主观题评分中的证据是否可追溯、是否支撑采分点。
