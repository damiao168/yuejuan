# STORY-009 自审审批记录

## Story

STORY-009 试卷与题目配置模块

## 审批结论

Approved

## 验收项

| 验收项 | 证据 | 结论 |
| --- | --- | --- |
| 试卷文件元数据登记 | `POST /api/v1/exams/{examId}/papers` 已实现；响应明确真实上传属于后续 Story | 通过 |
| 题目创建/列表/修改/删除 | `internal/paper` Handler、Store 和路由已实现 | 通过 |
| 题型受控 | 服务端枚举和数据库 CHECK；`TestInvalidQuestionTypeRejected` 覆盖 | 通过 |
| 标准答案配置 | `question_answer_key` 表和 `AnswerKey` 输入已实现 | 通过 |
| 答题区域 JSON 配置 | `answer_area` JSONB / map 结构已实现 | 通过 |
| Rubric 版本管理 | `rubric_version`、`question_rubric` 和递增版本号已实现 | 通过 |
| Rubric 分值校验 | `TestRubricScoreMismatchRejected` 覆盖 | 通过 |
| locked Rubric 保护 | `TestRubricVersioningAndLock` 覆盖 | 通过 |
| 配置完整性检查 | `POST /api/v1/exams/{examId}/validate-paper-config`；`TestValidatePaperConfigFindsScoreMismatch` 覆盖 | 通过 |
| 跨考试试卷引用防护 | `TestQuestionRejectsPaperFromDifferentExam` 覆盖 | 通过 |
| 权限校验 | 所有接口要求 `exam:manage`；`TestPaperPermissionDenied` 覆盖 | 通过 |
| 审计日志 | 关键修改动作均调用 audit | 通过 |
| 系统能力声明 | `system/info` 包含 paper/question/rubric/config validation，`file_upload` 仍在 not_implemented | 通过 |
| API 文档 | `docs/api/paper-question.md` 已新增 | 通过 |
| 不越界 | 未实现真实文件上传、OCR、答卷采集、AI 评分 | 通过 |

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
GET http://127.0.0.1:18089/api/v1/system/info
GET http://127.0.0.1:18089/api/v1/exams/exam-1/questions
```

结果：

```text
system/info -> capabilities include paper_metadata, question_config, rubric_versioning, paper_config_validation
GET /api/v1/exams/exam-1/questions without token -> 401
```

## 剩余风险

- 尚未跑真实 PostgreSQL migration 集成测试。
- 文件 hash 去重、真实上传、短期下载 URL 和对象存储鉴权属于 STORY-010。
- Rubric 审批流目前只保存状态，审批工作流会在后续阅卷/质量控制 Story 中加强。

## 下一步

进入 `STORY-010 文件上传与对象存储`，实现真实文件上传、MinIO/S3 写入、文件元数据、下载授权、类型/大小/hash 校验和审计。
