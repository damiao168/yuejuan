# STORY-046 生产管理员 Bootstrap 与会话安全加固规格

## 目标

关闭 STORY-045 中“缺少生产管理员初始化流程”和“Web token 存放在 localStorage”的 P0/P1 上线风险。

## 范围

本轮处理：

- 提供运维可执行的首个管理员 bootstrap 能力，用于在生产迁移后创建或激活第一个平台管理员。
- bootstrap 必须要求强密码，且在已有 active 管理员时拒绝覆盖。
- 后端登录成功后设置 HttpOnly session cookie。
- 后端认证中间件支持从 HttpOnly session cookie 读取会话，同时保留 Bearer token 兼容 API/测试调用。
- 后端登出时撤销 session 并清除 session cookie。
- CORS 对受信任 Origin 返回 `Access-Control-Allow-Credentials: true`，支持 Web cookie 会话。
- Web 管理端不再把 access token 写入或读取 `localStorage`，所有 API 请求改为携带 cookie credentials。
- 文件上传 XHR 同步携带 cookie credentials。
- 更新文档和 Story 记录。

本轮不处理：

- 不实现完整用户/角色管理页面。
- 不移除后端 JSON 响应中的 `access_token`，以保留非浏览器 API client 兼容性。
- 不实现 refresh token、OIDC/SSO、MFA 或密码轮换策略。
- 不接入 `lab/` 实验目录。

## 规格审阅

- 公开 bootstrap 注册接口会扩大攻击面；生产首个管理员更适合运维命令或受控函数，不挂公开 HTTP 路由。
- 当前后端已有 `auth_session` 表和 token hash 机制，适合复用为 cookie session，不需要新建 session 存储。
- 前端当前多个页面通过 `edugrade.access_token` 判断是否允许调用真实 API；迁移 cookie 时必须一起清理，否则登录后页面会被旧门禁拦住。
- 开发环境仍是 HTTP，本轮 cookie 需要支持开发可用，同时生产环境默认 `Secure`。
- 保留 Bearer token 能减少对现有测试和非浏览器调用的破坏，但 Web 端不得再保存 token。

## 规格修改

审阅后收敛为最小生产化闭环：

- 后端新增 `BootstrapInitialAdmin` 存储能力和 `bootstrap-admin` CLI 子命令；CLI 通过环境变量读取用户名、显示名和密码。
- bootstrap 默认目标为 `platform` 租户和 `platform_admin` 角色；已有 active 平台管理员时返回错误，不覆盖。
- session cookie 默认名为 `edugrade_session`，HttpOnly，SameSite=Lax；非 development/test 环境默认 Secure，可通过环境变量显式覆盖。
- Web API client 使用 `credentials: "include"`；不再注入 Authorization header。
- 业务页移除 localStorage token 门禁，改为依赖全局登录态和后端 401/403。

## 验收标准

- 登录响应包含 HttpOnly session cookie。
- 只带 session cookie 访问 `/api/v1/auth/me` 成功。
- 登出后 session cookie 被清除，原 cookie 无法继续访问 `/api/v1/auth/me`。
- 允许的跨域请求返回 `Access-Control-Allow-Credentials: true`。
- bootstrap 能在没有 active 平台管理员时创建或激活管理员并分配平台管理员角色。
- bootstrap 在已有 active 平台管理员时拒绝覆盖。
- Web 源码不再读取或写入 `edugrade.access_token`。
- `go test ./...`、`npm.cmd run typecheck`、Compose config 通过。

## 实现

新增：

- `services/api-gateway/internal/auth/bootstrap.go`
- `services/api-gateway/internal/auth/bootstrap_test.go`
- `services/api-gateway/internal/middleware/middleware_test.go`

修改：

- `services/api-gateway/internal/auth/handlers.go`
- `services/api-gateway/internal/auth/handlers_test.go`
- `services/api-gateway/internal/auth/store_postgres.go`
- `services/api-gateway/internal/config/config.go`
- `services/api-gateway/internal/config/config_test.go`
- `services/api-gateway/internal/middleware/middleware.go`
- `services/api-gateway/internal/server/server.go`
- `services/api-gateway/cmd/api-gateway/main.go`
- `apps/web-admin/src/api/client.ts`
- `apps/web-admin/src/api/files.ts`
- `apps/web-admin/src/App.tsx`
- `apps/web-admin/src/pages/*`
- `.env.example`
- `infra/docker-compose/.env.example`
- `infra/docker-compose/docker-compose.yml`
- `apps/web-admin/README.md`
- `docs/deployment/private-deployment.md`
- `docs/deployment/production-readiness-roadmap.md`

实现摘要：

- 登录成功时写入 `edugrade_session` HttpOnly cookie；生产类环境默认 `Secure`，development/test/local 默认允许 HTTP。
- `AuthMiddleware` 支持 Bearer token 和 session cookie 两种来源；登出会撤销 session 并清除 cookie。
- CORS 对允许 Origin 返回 `Access-Control-Allow-Credentials: true`。
- 新增 `BootstrapInitialAdmin` 和 `api-gateway bootstrap-admin` 运维命令；默认创建或激活 `platform/platform_admin` 首个管理员，已有 active 管理员时拒绝覆盖。
- Web API client 和文件上传 XHR 改为携带 cookie credentials，不再读写 `localStorage` token。
- 业务页移除 `edugrade.access_token` 门禁，改为依赖全局登录态和后端认证错误。

## 实现审阅

- 已通过 RED/GREEN 测试验证登录 cookie、cookie-only `/auth/me`、登出清 cookie、CORS credentials、bootstrap 创建和已有管理员拒绝覆盖。
- 已检索 Web 源码，确认没有 `edugrade.access_token`、`localStorage` 或旧 `hasToken` 残留。
- 已更新部署文档，说明 bootstrap 命令和强密码要求。
- 已确认 `lab/` 未接入生产链路，本轮未修改。

## 修改实现

实现审阅后补充了 Compose/env 示例中的 session cookie 和 bootstrap 环境变量，并修正 Web README、私有化部署说明和生产化路线图。

## 测试结果

- 目标 RED 测试已先失败，原因分别是缺少 bootstrap API、缺少 cookie config 字段、缺少 CORS credentials。
- 目标测试实现后通过。
- 全量验证命令见 STORY-046 完成记录。

## 剩余风险

- 后端仍为单 session cookie，没有 refresh token、MFA、SSO、密码轮换和完整账号生命周期管理。
- JSON 登录响应仍保留 `access_token` 以兼容非浏览器 API client；Web 不再保存该 token。
- Web mock 页面仍需 STORY-048 清理或生产隐藏。
- 真实 OCR/LLM/Agent Runtime 仍需后续 Story 接入。
