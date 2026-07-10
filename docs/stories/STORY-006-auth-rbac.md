# STORY-006 认证与权限系统

## 状态

Approved

## 目标

实现 EduGrade Enterprise 的认证与权限系统基础版本，支持多租户、RBAC、数据范围权限和登录审计。

## Plan

- 新增认证/RBAC migration 和种子数据。
- 实现密码 hash 校验。
- 实现 session token 登录，而不是前端假登录。
- 实现接口：
  - `POST /api/v1/auth/login`
  - `POST /api/v1/auth/logout`
  - `GET /api/v1/auth/me`
- 实现认证中间件和权限中间件。
- 实现 PostgreSQL AuthStore，生产路径使用数据库。
- 实现 Memory AuthStore，仅用于单元测试。
- 审计登录、登出、失败登录。
- 添加接口测试，验证登录、失败登录、me、logout、权限拒绝。

## Plan Review

- 不越界：不实现组织管理、考试、试卷或阅卷业务。
- 当前后端已有 API Gateway，因此认证路由落在 `services/api-gateway`。
- 认证必须服务端生效，不能只在前端隐藏。
- 数据库 migration 只覆盖本 Story 必需的 tenant/user/role/permission/session/audit 基础表，不提前实现全量业务表。
- 种子数据使用 pgcrypto `crypt()` 生成 bcrypt 兼容 hash；默认密码仅用于本地开发说明。

## Implementation

- 新增 `services/api-gateway/migrations/000001_auth_rbac.sql`，包含 tenant、app_user、role、permission、user_role、role_permission、auth_session、audit_log 和种子数据。
- 新增 `internal/auth`，包含密码 hash、token、session、AuthStore、PostgresStore、MemoryStore、登录/登出/me handler、认证中间件和权限中间件。
- 新增 `internal/db`，统一打开 PostgreSQL。
- 将认证路由接入 API Gateway。
- 补充 auth 接口测试和权限拒绝测试。

## Implementation Review

审阅发现并修正：

- `system/info` 仍将 auth 标为 `not_implemented`，已改为 `session_auth`、`rbac_middleware`、`login_audit` 能力。
- `services/api-gateway/README.md` 仍写“登录和权限未实现”，已修正。
- 失败登录在未知用户时缺少 tenant 上下文，已使用 platform tenant 作为失败登录审计兜底。

## Fixes

- 修正系统能力声明。
- 修正 README 当前状态。
- 补充失败登录审计兜底。
- 重新运行 `go test ./...` 和启动验证。

## 自审审批

审批文件：`docs/stories/STORY-006-approval.md`

## 非范围

- 不接 OIDC/SAML/LDAP。
- 不实现完整学校/班级数据范围计算。
- 不实现用户管理 CRUD。
- 不实现前端登录页面。

## 验收标准

- 登录成功返回 token 和用户信息。
- 登录失败写审计且返回统一错误。
- `/api/v1/auth/me` 必须要求登录。
- `/api/v1/auth/logout` 删除 session 并写审计。
- 权限中间件能拒绝缺少权限的用户。
- 生产路径有 PostgreSQL Store。
- 测试通过。
