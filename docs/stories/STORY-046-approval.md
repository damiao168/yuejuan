# STORY-046 完成记录

## Story

生产管理员 Bootstrap 与会话安全加固。

## 规格

- 已明确本轮关闭首个管理员初始化缺口和 Web localStorage token 风险。
- 已明确不实现完整用户管理、MFA/SSO、refresh token、密码轮换和 `lab/` 接入。

## 规格审阅与修改

- 规格审阅后选择运维 CLI bootstrap，而不是公开 HTTP 注册接口。
- 复用现有 `auth_session` 和 token hash 机制，新增 HttpOnly cookie 作为浏览器会话载体。
- 保留 Bearer token 兼容非浏览器调用，同时要求 Web 不再保存 token。

## 实现审阅

- 登录设置 HttpOnly session cookie。
- `/api/v1/auth/me` 可只依赖 session cookie 认证。
- 登出会撤销 session 并清除 cookie。
- CORS 对允许 Origin 返回 credentials。
- bootstrap 创建首个平台管理员，且已有 active 平台管理员时拒绝覆盖。
- Web 源码不再包含 `edugrade.access_token`、`localStorage` 或旧 `hasToken` 门禁。

## 测试

- `go test ./internal/auth -run "TestLoginSetsHttpOnlySessionCookieAndMeAcceptsCookie|TestLogoutClearsSessionCookieAndRevokesCookieSession|TestBootstrapInitialAdmin"`
- `go test ./internal/middleware -run TestCORSAllowsCredentialsForAllowedOrigin`
- `go test ./internal/config -run "TestLoadDefaultsSessionCookieSecureForProduction|TestLoadAllowsSessionCookieSecureOverride"`
- `go test ./...`
- `npm.cmd run typecheck --workspace apps/web-admin`
- `npm.cmd run typecheck`
- `docker compose --env-file infra/docker-compose/.env.example -f infra/docker-compose/docker-compose.yml config`
- `Select-String` 检查 Web token 残留。

## 结论

STORY-046 批准。项目可以继续进入 STORY-047：数据库级多租户约束硬化。
