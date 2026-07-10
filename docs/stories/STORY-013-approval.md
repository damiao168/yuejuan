# STORY-013 自审审批记录

## Story

STORY-013 答题区域切分模块

## 审批结论

Approved

## 验收项

| 验收项 | 证据 | 结论 |
| --- | --- | --- |
| 生成 answer_segment | `TestGenerateSegmentsIdempotentAndManualUpdate` 覆盖 | 通过 |
| 缺配置返回 issue | `TestGenerateSegmentsReportsMissingPageAndAnswerArea` 覆盖 | 通过 |
| 重复执行不重复生成 | 同一测试断言 list 只有一个 segment | 通过 |
| 人工修正 | PATCH 修改 bbox/status/notes，source 变为 manual | 通过 |
| ready 状态要求 | `TestSegmentationRequiresReadySubmission` 覆盖 | 通过 |
| 权限 | 所有路由要求 `segment:manage`，`TestSegmentPermissionDenied` 覆盖 | 通过 |
| 审计 | `segment.generated`、`segment.updated` 测试覆盖 | 通过 |
| 系统能力声明 | `system/info` 包含 answer segmentation capabilities | 通过 |
| API 文档 | `docs/api/answer-segments.md` 已新增 | 通过 |
| 不越界 | 未实现图片裁剪、模型切分、OCR 文本归属、AI 阅卷 | 通过 |

## 运行命令与结果

```powershell
Push-Location .\services\api-gateway
go test ./...
Pop-Location
docker compose --env-file .env.example -f infra\docker-compose\docker-compose.yml config
Push-Location .\services\api-gateway
go build -o ..\..\bin\api-gateway.exe .\cmd\api-gateway
Pop-Location
```

结果：

```text
go test ./... -> passed
docker compose config -> passed
go build -> passed
```

启动验证：

```powershell
GET http://127.0.0.1:18093/api/v1/system/info
GET http://127.0.0.1:18093/api/v1/submissions/submission-1/answer-segments
```

结果：

```text
system/info -> capabilities include answer_segmentation_metadata, answer_segment_manual_review
system/info -> not_implemented includes ocr_engine_inference, ai_grading
GET /api/v1/submissions/submission-1/answer-segments without token -> 401
```

## 剩余风险

- 当前只生成 segment 元数据，不裁剪单题图片。
- 未做坐标自动校准、模型版面识别或 OCR 文本归属。
- 后续 AI/人工阅卷需要继续消费 `answer_segment`。

## 下一步

进入 `STORY-014 多智能体 Orchestrator`，建立受控任务编排、Agent 调用记录、失败重试和人工复核触发入口。
