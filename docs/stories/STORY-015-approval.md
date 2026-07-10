# STORY-015 自审审批记录

## Story

STORY-015 客观题与填空题判分

## 审批结论

Approved

## 验收项

| 验收项 | 证据 | 结论 |
| --- | --- | --- |
| 记录 answer_segment 学生答案 | `TestRecordAnswerRuleGradeListAndAudit` 覆盖 | 通过 |
| single_choice 精确判分 | `TestEngineGradesSupportedQuestionTypes` 覆盖 | 通过 |
| true_false 精确判分 | 同一测试覆盖中文正确输入 | 通过 |
| multiple_choice 全对/错选/少选部分分 | 同一测试覆盖错选 0 分和部分分 | 通过 |
| fill_blank 等价答案/大小写/空格 | 同一测试覆盖 | 通过 |
| numeric 容差和单位 | 同一测试覆盖 `9.81m/s2` | 通过 |
| 缺答案/缺 answer_key | `TestRuleGradeMissingAnswerAndAnswerKey` 覆盖 | 通过 |
| 生成 ai_grade | Handler 测试断言 `suggested_score`、`grader_type`、`mock=false` | 通过 |
| 分数不超过满分 | Engine 测试断言 | 通过 |
| 结果结构完整 | Grade 包含 score/confidence/matched/missing/evidence/risk/review | 通过 |
| 权限保护 | `grading:manage`、401、403 测试覆盖 | 通过 |
| 审计 | `grading.answer_recorded`、`grading.rule_grade_created` 测试覆盖 | 通过 |
| 逻辑不在 Controller | `Engine` 单独测试覆盖 | 通过 |
| 不越界 | 未实现主观题 AI、模型调用、OCR 自动归属和最终成绩 | 通过 |

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

- 当前只支持规则判分，不包含主观题 LLM 评分。
- 选择题涂点识别和 OCR 文本自动归属尚未实现，需要后续 Story 接入真实来源。
- `ai_grade` 已统一记录结果，但 final_grade、human_grade、成绩发布仍未实现。

## 下一步

进入 `STORY-016 主观题 AI 评分接口`，建立可替换 LLM Adapter、schema 校验、mock 标记和人工复核边界。
