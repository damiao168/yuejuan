# STORY-017 自审审批记录

## Story

STORY-017 证据校验 Agent

## 审批结论

Approved

## 验收项

| 验收项 | 证据 | 结论 |
| --- | --- | --- |
| 超分会失败并强制人工复核 | `TestVerifyFailsWhenSuggestedScoreExceedsMax` 覆盖 | 通过 |
| 不存在的采分点会失败 | `TestVerifyFailsWhenRubricPointIsUnknown` 覆盖 | 通过 |
| 缺失 Rubric 时采分点无法校验会失败 | `TestVerifyFailsWhenRubricIsMissingForPointReferences` 覆盖 | 通过 |
| 空 evidence 会失败 | `TestVerifyFailsWhenEvidenceIsEmpty` 覆盖 | 通过 |
| OCR 低置信度触发人工复核 | `TestVerifyFlagsLowOCRConfidenceForReview` 覆盖 | 通过 |
| evidence 文本不在答案中会失败 | `TestVerifyFailsWhenEvidenceTextIsNotInAnswer` 覆盖 | 通过 |
| evidence bbox 越出 answer_segment 会失败 | `TestVerifyFailsWhenEvidenceBBoxLeavesSegment` 覆盖 | 通过 |
| 校验结果记录 agent job | `TestVerifyEvidenceRouteRecordsJobAndAudit` 覆盖 | 通过 |
| 接口受权限保护 | `evidence:manage`、401、403 测试覆盖 | 通过 |
| 写审计 | `evidence.checked` audit 测试覆盖 | 通过 |
| 不越界 | 未实现 NLP、视觉理解、语义充分性判断、final_grade 更新 | 通过 |

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

- 当前只是规则级证据校验，不代表语义证据充分性。
- 当前不做图片视觉理解，bbox 只校验坐标是否在 answer_segment 范围内。
- 证据校验失败只输出 `needs_human_review=true` 和 agent job，人工复核任务生成、最终成绩和成绩发布仍属于后续 Story。

## 下一步

进入后续人工复核、最终成绩和成绩发布相关 Story。
