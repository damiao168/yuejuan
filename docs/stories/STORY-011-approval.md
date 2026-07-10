# STORY-011 自审审批记录

## Story

STORY-011 答卷采集与 Submission

## 审批结论

Approved

## 验收项

| 验收项 | 证据 | 结论 |
| --- | --- | --- |
| 创建 submission | `POST /api/v1/exams/{examId}/submissions` 已实现，测试覆盖 | 通过 |
| 关联答卷页面 | `POST /api/v1/submissions/{id}/pages` 校验 file_asset 并创建页面 | 通过 |
| 页面重复检测 | `TestDuplicatePageRejected` 覆盖 | 通过 |
| 无页面/缺页/页数不一致质量门禁 | `TestSubmissionQualityGateFindsMissingPages` 覆盖 | 通过 |
| 状态流转受控 | `TestSubmissionCanBecomeReadyForOCRAfterQualityPass` 和 `TestCannotManuallyMarkQualityChecked` 覆盖 | 通过 |
| 质量结果失效 | 加页后重置 `quality_status=unchecked`，`TestAddingPageAfterQualityPassInvalidatesGate` 覆盖 | 通过 |
| 权限 | 所有路由要求 `submission:manage`，`TestSubmissionPermissionDenied` 覆盖 | 通过 |
| 审计 | 创建、加页、质量检查、状态流转均调用 audit，测试覆盖 | 通过 |
| 系统能力声明 | `system/info` 包含 submission capabilities，OCR 仍为 not_implemented | 通过 |
| API 文档 | `docs/api/submissions.md` 已新增 | 通过 |
| 不越界 | 未实现 OCR、图像质量算法、条码识别、答题区域切分 | 通过 |

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
GET http://127.0.0.1:18091/api/v1/system/info
GET http://127.0.0.1:18091/api/v1/exams/exam-1/submissions
```

结果：

```text
system/info -> capabilities include submission_collection, submission_pages, submission_quality_gate
system/info -> not_implemented still includes ocr, ai_grading
GET /api/v1/exams/exam-1/submissions without token -> 401
```

## 剩余风险

- 尚未跑真实 PostgreSQL migration 集成测试。
- 当前质量门禁是元数据级，不包含图像模糊、倾斜、曝光、条码识别等真实算法。
- `ready_for_ocr` 之后的任务队列、OCR 结果、人工校正属于后续 Story。

## 下一步

进入 `STORY-012 OCR 服务接口与任务队列`，实现 OCR 任务模型、队列抽象、任务状态、OCR 结果结构和低置信度人工复核入口。
