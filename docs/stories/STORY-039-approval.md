# STORY-039 Approval

结论：批准。

## 修改摘要

1. 后端增加安全中间件、CORS allowlist、全局请求体限制、HTTP header 上限配置和安全响应头。
2. 系统诊断 API 改为 `system:read` 鉴权，保留 `/health` 公开。
3. 登录失败增加短窗口限制，限流事件写入审计，并停止信任未校验的 `X-Forwarded-For`。
4. 学生成绩读取增加对象级授权，防止学生越权读取他人成绩。
5. 审计日志列表和导出增加高敏字段脱敏，普通接口不可删除审计日志。
6. 文件名校验增强，Prompt 注入基础检测落地，EXE 诊断页增加本地缓存安全检查。

## 新增文件

1. `services/api-gateway/internal/auth/login_limiter.go`
2. `docs/stories/STORY-039-security-hardening.md`
3. `docs/stories/STORY-039-approval.md`

## 修改文件

1. `.env.example`
2. `infra/docker-compose/.env.example`
3. `infra/docker-compose/docker-compose.yml`
4. `apps/desktop-client/src/App.tsx`
5. `apps/desktop-client/src/lib/localRuntime.ts`
6. `apps/desktop-client/src/styles.css`
7. `apps/desktop-client/src/types.ts`
8. `apps/web-admin/src/auth/session.ts`
9. `apps/web-admin/src/router/routes.tsx`
10. `services/api-gateway/cmd/api-gateway/main.go`
11. `services/api-gateway/internal/auth/audit.go`
12. `services/api-gateway/internal/auth/handlers.go`
13. `services/api-gateway/internal/auth/handlers_test.go`
14. `services/api-gateway/internal/config/config.go`
15. `services/api-gateway/internal/files/handlers_test.go`
16. `services/api-gateway/internal/files/validation.go`
17. `services/api-gateway/internal/middleware/middleware.go`
18. `services/api-gateway/internal/org/handlers_test.go`
19. `services/api-gateway/internal/score/handlers.go`
20. `services/api-gateway/internal/server/score_route_test.go`
21. `services/api-gateway/internal/server/server.go`
22. `services/api-gateway/internal/server/server_test.go`
23. `services/api-gateway/internal/subjective/handlers.go`
24. `services/api-gateway/internal/subjective/handlers_test.go`
25. `services/api-gateway/internal/subjective/types.go`
26. `services/api-gateway/internal/subjective/validation.go`
27. `docs/stories/README.md`

## 运行命令

```powershell
gofmt -w ...
Push-Location .\services\api-gateway; go test ./...; Pop-Location
Push-Location .\services\api-gateway; go build -o ..\..\bin\api-gateway.exe .\cmd\api-gateway; Pop-Location
npm.cmd run typecheck
npm.cmd run build
docker compose --env-file .\infra\docker-compose\.env.example -f .\infra\docker-compose\docker-compose.yml config
npx.cmd --yes --package @playwright/cli playwright-cli open http://127.0.0.1:5180
npx.cmd --yes --package @playwright/cli playwright-cli screenshot --filename output/playwright/story-039-desktop-cache-security.png
```

## 测试结果

1. 后端 `go test ./...`：通过。
2. 后端 `go build`：通过。
3. 前端 `npm.cmd run typecheck`：通过；保留 npm `store-dir` 警告。
4. 前端 `npm.cmd run build`：通过；保留 Vite 大 chunk 警告。
5. Docker Compose `config`：通过。
6. Playwright EXE 诊断页：通过；截图 `output/playwright/story-039-desktop-cache-security.png`，控制台 error 为 0。
7. `go test -race ./internal/auth ./internal/server`：当前环境缺少 `gcc`，无法构建 `runtime/cgo`，未作为通过项。

## 剩余风险

1. 多实例登录限流需要 Redis/数据库集中状态。
2. 租户隔离尚未升级到数据库 RLS。
3. Prompt 注入防护仍为基础规则检测，需要后续评估集和红队测试加强。
4. 文件安全未包含病毒扫描和宏/主动内容检测。
5. Web 管理后台真实登录体系仍需后续 Story 产品化。

## 下一步建议

进入 STORY-040：AI 评估集与评分质量测试。
