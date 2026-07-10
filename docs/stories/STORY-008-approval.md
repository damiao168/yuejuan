# STORY-008 自审审批记录

## Story

STORY-008 考试管理模块

## 审批结论

Approved

## 验收项

| 验收项 | 证据 | 结论 |
| --- | --- | --- |
| 创建考试/作业 | `POST /api/v1/exams` 已实现，`TestCreateListDetailExam` 覆盖 | 通过 |
| 修改基础信息 | `PATCH /api/v1/exams/{id}` 已实现 | 通过 |
| 选择适用班级 | `class_ids` 写入 `exam_class`，测试验证持久化 | 通过 |
| 学科、类型、总分、阅卷模式、申诉、发布策略 | create/update schema 和校验覆盖 | 通过 |
| 状态流转 | `CanTransition` 白名单和 `TestInvalidStatusTransitionRejected` 覆盖 | 通过 |
| 列表和详情 | `GET /api/v1/exams`、`GET /api/v1/exams/{id}` 已实现并测试 | 通过 |
| 删除/归档 | `POST /api/v1/exams/{id}/archive` 已实现并测试 | 通过 |
| 发布后不能随意修改核心配置 | `TestPublishedExamCannotBeModified` 覆盖 | 通过 |
| 权限校验 | 所有考试路由要求 `exam:manage`；`TestExamPermissionDenied` 覆盖 | 通过 |
| 审计日志 | 创建、修改、状态变化、归档均调用 audit | 通过 |
| API 文档 | `docs/api/exams.md` 已新增 | 通过 |
| 不越界 | 未实现试卷、题目、OCR、阅卷、成绩 | 通过 |

## 运行命令与结果

```powershell
go test ./...
docker compose --env-file .env.example -f infra\docker-compose\docker-compose.yml config
```

结果：

```text
ok  	edugrade-enterprise/services/api-gateway/internal/exam
ok  	edugrade-enterprise/services/api-gateway/internal/auth
ok  	edugrade-enterprise/services/api-gateway/internal/org
ok  	edugrade-enterprise/services/api-gateway/internal/server
docker compose config -> passed
```

启动验证：

```powershell
curl http://127.0.0.1:18088/api/v1/system/info
curl -i http://127.0.0.1:18088/api/v1/exams
```

结果：

```text
system/info -> capabilities include exam_management
GET /api/v1/exams without token -> 401 unauthenticated
```

## 剩余风险

- 尚未跑真实 PostgreSQL migration 集成测试。
- 班级是否属于同一 school/tenant 当前依赖数据库外键和服务层 tenant 过滤，后续可加强校验错误信息。
- 发布前质量检查、试卷配置完整性和成绩发布实际动作属于后续 Story。

## 下一步

进入 `STORY-009 试卷与题目配置模块`，实现试卷文件元数据、题目、标准答案、Rubric 版本和配置完整性检查。
