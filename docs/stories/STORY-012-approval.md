# STORY-012 自审审批记录

## Story

STORY-012 OCR 服务接口与任务队列

## 审批结论

Approved

## 验收项

| 验收项 | 证据 | 结论 |
| --- | --- | --- |
| ready submission 创建 OCR 任务 | `TestCreateStartCompleteOCRTaskWithLowConfidence` 覆盖 | 通过 |
| 未 ready submission 拒绝创建 | `TestOCRTaskRequiresReadySubmission` 覆盖 | 通过 |
| 创建任务入队 | MemoryQueue 断言队列长度 | 通过 |
| 启动任务 | `POST /api/v1/ocr-tasks/{id}/start` 已实现并测试 | 通过 |
| 提交结构化结果 | `POST /api/v1/ocr-tasks/{id}/results` 保存 result schema | 通过 |
| 低置信度触发人工复核 | 低于 `min_confidence` 时 `requires_human_review=true`，测试覆盖 | 通过 |
| 状态流转受控 | `TestCompleteBeforeStartRejected` 覆盖 | 通过 |
| 失败任务 | `TestOCRTaskFail` 覆盖 | 通过 |
| 页面归属保护 | Handler 和 PostgresStore 均校验 page 属于 task submission | 通过 |
| 权限 | 所有路由要求 `ocr:manage`，`TestOCRPermissionDenied` 覆盖 | 通过 |
| 审计 | 创建、启动、完成、失败均调用 audit，测试覆盖 | 通过 |
| 系统能力声明 | 声明 OCR 任务/结果接口，真实推理仍为 not_implemented | 通过 |
| API 文档 | `docs/api/ocr.md` 已新增 | 通过 |
| 不越界 | 未实现真实 OCR 引擎、图像预处理、版面解析、切题 | 通过 |

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
GET http://127.0.0.1:18092/api/v1/system/info
GET http://127.0.0.1:18092/api/v1/submissions/submission-1/ocr-tasks
```

结果：

```text
system/info -> capabilities include ocr_task_management, ocr_result_ingestion, ocr_low_confidence_review_trigger
system/info -> not_implemented includes ocr_engine_inference, ai_grading
GET /api/v1/submissions/submission-1/ocr-tasks without token -> 401
```

## 剩余风险

- 当前队列为接口抽象和任务表持久化，未接 Redis/Temporal/Celery worker。
- 未实现真实 OCR 引擎推理、图像预处理和人工校正 UI。
- OCR result 还未进入答题区域切分，属于 STORY-013。

## 下一步

进入 `STORY-013 答题区域切分模块`，基于 OCR result、试题 answer_area 和 submission page 生成 answer_segment。
