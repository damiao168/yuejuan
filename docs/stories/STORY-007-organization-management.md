# STORY-007 组织、学校、班级、学生管理

## 状态

Approved

## 目标

实现组织管理模块：租户、学校、年级、班级、学生、学生 CSV 导入、教师与班级绑定、数据范围权限控制和操作审计。

## Plan

- 新增组织管理 migration。
- 新增 `internal/org`，包含 Store、PostgresStore、MemoryStore、Handlers。
- 新增接口：
  - `POST /api/v1/tenants`
  - `GET /api/v1/tenants`
  - `PATCH /api/v1/tenants/{id}`
  - `POST /api/v1/schools`
  - `GET /api/v1/schools`
  - `POST /api/v1/grades`
  - `GET /api/v1/grades`
  - `POST /api/v1/classes`
  - `GET /api/v1/classes`
  - `POST /api/v1/students`
  - `GET /api/v1/students`
  - `PATCH /api/v1/students/{id}`
  - `POST /api/v1/students/import-csv`
  - `POST /api/v1/classes/{id}/teachers`
- 所有接口走认证中间件和服务端权限中间件。
- 所有查询按 tenant 隔离；platform_admin 可跨租户查看租户列表。
- 创建、修改、禁用、导入和绑定动作写 audit_log。
- 添加测试。

## Plan Review

- 不越界：不实现考试、试卷、文件上传、OCR 或阅卷。
- 当前认证系统已提供 session、permissions 和 audit，因此本 Story 应复用。
- CSV 导入必须返回错误行报告，不能简单吞掉失败行。
- 学生姓名和学号是敏感字段，本 Story 先保留字段，后续 UI/权限 Story 再做脱敏展示。
- 教师与班级绑定需要单独表，不塞进 JSON。

## Implementation

- 新增 `000002_organization.sql`，包含 school、grade、school_class、student、teacher_class 和组织权限种子。
- 新增 `internal/org`，包含类型、PostgresStore、MemoryStore 和 handlers。
- 接入 API Gateway 路由，并使用认证与权限中间件保护。
- 实现学生 CSV 导入，逐行返回错误报告。
- 创建、更新、导入、绑定动作调用 audit。
- 补充组织模块测试。

## Implementation Review

审阅发现并修正：

- CSV 字段数不一致时 `encoding/csv` 会整体失败，不符合“错误行报告”要求；已设置 `FieldsPerRecord = -1` 并逐行记录错误。
- `system/info` 仍把 tenant_management 标为未实现；已修正为 capabilities。
- README 当前状态未反映组织基础 API；已修正。
- 需要更直接的 tenant 隔离证据；已补 `TestMemoryStoreTenantIsolation`。

## Fixes

- 修正 CSV 导入错误行逻辑。
- 修正系统能力声明和 README。
- 补充 tenant 隔离测试。
- 重新运行测试和启动验证。

## 自审审批

审批文件：`docs/stories/STORY-007-approval.md`

## 非范围

- 不实现完整用户管理。
- 不实现 Excel 导入。
- 不实现学生/家长端。
- 不实现字段级脱敏 UI。

## 验收标准

- 所有组织查询带 tenant 隔离。
- CSV 导入返回成功数和错误行。
- 创建/修改/禁用/绑定写审计。
- 无权限访问返回 403。
- 测试通过。
