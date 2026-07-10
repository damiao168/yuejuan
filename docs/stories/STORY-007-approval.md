# STORY-007 自审审批记录

## Story

STORY-007 组织、学校、班级、学生管理

## 审批结论

Approved

## 验收项

| 验收项 | 证据 | 结论 |
| --- | --- | --- |
| 租户管理 | 已实现 `POST/GET/PATCH /api/v1/tenants`，并有 migration 支撑 | 通过 |
| 学校管理 | 已实现 `POST/GET /api/v1/schools`，测试覆盖创建和列表 | 通过 |
| 年级管理 | 已实现 `POST/GET /api/v1/grades` | 通过 |
| 班级管理 | 已实现 `POST/GET /api/v1/classes` | 通过 |
| 学生管理 | 已实现 `POST/GET/PATCH /api/v1/students/{id}` | 通过 |
| CSV 导入 | `POST /api/v1/students/import-csv` 返回 created 和 errors，测试覆盖错误行 | 通过 |
| 教师与班级绑定 | `POST /api/v1/classes/{id}/teachers` 写入 teacher_class | 通过 |
| 数据范围权限控制 | 所有组织路由走 AuthMiddleware + RequirePermission；Store 查询带 tenantID；测试覆盖 tenant 隔离 | 通过 |
| 操作审计 | 创建、修改、导入、绑定动作调用 audit | 通过 |
| 无权限返回 403 | `TestOrganizationPermissionDenied` 覆盖 | 通过 |
| 不越界 | 未实现考试、试卷、OCR、阅卷业务 | 通过 |

## 运行命令与结果

```powershell
go test ./...
```

结果：

```text
ok  	edugrade-enterprise/services/api-gateway/internal/auth
ok  	edugrade-enterprise/services/api-gateway/internal/config
ok  	edugrade-enterprise/services/api-gateway/internal/org
ok  	edugrade-enterprise/services/api-gateway/internal/server
```

启动验证：

```powershell
curl http://127.0.0.1:18087/api/v1/system/info
curl -i http://127.0.0.1:18087/api/v1/schools
```

结果：

```text
system/info -> capabilities include tenant_management, organization_management, student_csv_import
GET /api/v1/schools without token -> 401 unauthenticated
```

## 剩余风险

- 尚未用真实 PostgreSQL 容器执行 migration 集成测试。
- 学生敏感字段的脱敏展示还未在 UI/API 响应策略中细化。
- 数据范围目前按 tenant 强制隔离，学校/班级/教师分配粒度将在后续业务 Story 继续加强。
- CSV 只支持基础文本 CSV，不支持 Excel。

## 下一步

进入 `STORY-008 考试管理模块`，实现考试创建、班级选择、阅卷模式、发布策略和考试状态流转校验。
