# STORY-039 安全加固

## 安全检查清单

| 检查项 | 当前证据 | 本 Story 处理 |
| --- | --- | --- |
| 1. 所有 API 鉴权 | `/health`、`/ready`、`/api/v1/system/info`、`/api/v1/system/status` 公开；业务 API 大多已通过 Bearer Session + RBAC 保护 | 保留 `/health` 公开用于健康检查；将 `/ready`、`/api/v1/system/info`、`/api/v1/system/status` 纳入 `system:read` 鉴权 |
| 2. 所有 `tenant_id` 隔离 | Store 层主要按 `user.TenantID` 查询，文件、成绩、报告、审计均可见租户条件 | 增加路由测试覆盖关键安全面，避免跨租户或未鉴权访问；不做大规模数据层重写 |
| 3. 文件下载鉴权 | `/api/v1/files/{id}/download` 需要 `file:manage`，并按 `tenant_id` 读取 | 保持并补充安全文档说明 |
| 4. 防止越权访问学生成绩 | `student:grade:read` 可读取任意 `studentId` 的成绩 | 增加对象级授权：普通学生只能读取 `data_scope.student_id` 对应成绩；`score:manage` 可读取全量 |
| 5. 防止阅卷员查看学生姓名 | 阅卷任务以 `anonymous_code` 暴露；学生列表需 `org:manage` | 补充测试与清单说明，确保阅卷权限不等于学生姓名读取权限 |
| 6. 密码 hash | 已使用 bcrypt | 保留并记录为已满足 |
| 7. 登录失败限制 | 登录失败会审计，但无限制 | 增加内存登录失败限制，按租户、用户名、IP 组合限流，并审计锁定事件 |
| 8. CORS 配置 | 未见集中 CORS 配置 | 增加显式 CORS allowlist 配置与中间件，不反射任意 Origin |
| 9. 请求体大小限制 | 文件上传有 `MaxBytesReader`；普通 JSON API 依赖默认无全局限制 | 增加全局请求体大小限制，文件上传保留更大的专用上限 |
| 10. 文件类型白名单 | 已按扩展名、声明类型、嗅探类型校验 | 保持并补充恶意文件名测试 |
| 11. 恶意文件名处理 | 已清理路径和控制字符 | 增加保留名/空扩展等加强校验 |
| 12. 审计日志不可通过普通接口删除 | 未发现审计删除路由 | 保持并用路由测试确认普通 DELETE 不存在 |
| 13. 导出操作审计 | 成绩、报告、审计导出均写审计事件 | 保持并补充清单说明 |
| 14. 敏感字段脱敏 | 系统日志有字段级脱敏；审计列表/导出原样返回 before/after | 增加审计输出脱敏，保护 password/token/secret/credential 等高敏字段 |
| 15. Prompt 注入基础防护 | 模型输出有基础 schema 校验；学生答案未标记注入风险 | 增加答案级注入意图检测，只作风险标记和强制人工复核，不允许学生答案覆盖评分规则 |
| 16. 本地 EXE 缓存安全检查 | token 仅内存保存；本地 localStorage 存扫描队列、日志、加密草稿 envelope | 增加本地缓存安全扫描，发现 token/auth/session 等敏感键时在诊断页报警 |

## Plan

### 目标

对 EduGrade Enterprise 做第一轮安全加固，优先补齐容易导致越权、信息泄露、滥用登录、跨源误配、资源耗尽和 Prompt 注入误判的基础防线。

### 范围

1. 后端 API Gateway：
   - 增加集中安全配置：CORS allowlist、全局请求体大小、登录失败限制、HTTP header 上限。
   - 增加安全中间件：安全响应头、CORS、全局 body limit。
   - 将 `/ready`、`/api/v1/system/info`、`/api/v1/system/status` 纳入 `system:read` 鉴权；保留 `/health` 与登录接口公开。
   - 学生成绩读取增加对象级授权，学生只能看自己，成绩管理员可看全量。
   - 登录失败按租户、用户名、IP 做短窗口限制；限制事件写入审计。
   - 审计日志输出对高敏 key 做脱敏，避免普通审计读取/导出泄漏 password、token、secret 等字段。
   - 保持文件上传白名单、下载鉴权与租户隔离，并加强文件名校验测试。
   - 主观题 AI 评分增加 Prompt 注入基础检测，检测后强制人工复核并记录风险标记。
2. Windows EXE 客户端：
   - 增加本地缓存安全检查，扫描 `edugrade.desktop.*` localStorage 键是否包含 token/auth/session/password 等敏感信息。
   - 在诊断页展示安全检查状态，异常时明确告警。
3. Web 管理后台：
   - 适配系统状态接口需要后端 Bearer token 的错误态。
   - 不在本 Story 重写 Web mock 登录体系，避免越过认证产品化 Story 的范围。
4. 文档与测试：
   - Story 文档记录每个修复点、验证命令和剩余风险。
   - 补充后端单元/路由测试和前端类型/构建验证。

### 非范围

1. 不实现完整 WAF、设备指纹、双因素认证、账户冻结后台、验证码或企业 IdP/SSO。
2. 不把当前开发态 Web mock 登录改造成完整真实登录系统。
3. 不实现病毒扫描、内容安全沙箱或 DLP，仅保留文件类型、文件名、大小和私有下载控制。
4. 不实现完整 LLM 安全网关或多轮红队评测；Prompt 注入只做第一轮规则检测和人工复核保护。
5. 不改造数据库 RLS；当前仍由应用层按 `tenant_id` 参数隔离。

### 预计修改文件

1. `docs/stories/README.md`
2. `docs/stories/STORY-039-security-hardening.md`
3. `docs/stories/STORY-039-approval.md`
4. `.env.example`
5. `infra/docker-compose/.env.example`
6. `services/api-gateway/cmd/api-gateway/main.go`
7. `services/api-gateway/internal/config/config.go`
8. `services/api-gateway/internal/middleware/middleware.go`
9. `services/api-gateway/internal/auth/handlers.go`
10. `services/api-gateway/internal/auth/types.go`
11. `services/api-gateway/internal/auth/store_memory.go`
12. `services/api-gateway/internal/auth/store_postgres.go`
13. `services/api-gateway/internal/files/validation.go`
14. `services/api-gateway/internal/score/handlers.go`
15. `services/api-gateway/internal/server/server.go`
16. `services/api-gateway/internal/server/server_test.go`
17. `services/api-gateway/internal/server/score_route_test.go`
18. `services/api-gateway/internal/subjective/validation.go`
19. `services/api-gateway/internal/subjective/handlers.go`
20. `services/api-gateway/internal/subjective/handlers_test.go`
21. `apps/desktop-client/src/lib/localRuntime.ts`
22. `apps/desktop-client/src/types.ts`
23. `apps/desktop-client/src/App.tsx`

### 验收标准

1. `/health` 可公开访问；`/ready`、`/api/v1/system/info`、`/api/v1/system/status` 未登录返回 401，权限不足返回 403。
2. CORS 只允许配置的 Origin，不反射未知 Origin。
3. 请求体超过配置上限时返回 413，文件上传仍使用文件专用上限。
4. 登录失败超过限制后短窗口内返回 429，并产生审计记录。
5. 学生只能读取 `data_scope.student_id` 对应成绩；`score:manage` 可读取全量成绩。
6. 阅卷员权限不能读取学生列表或学生姓名。
7. 文件上传拒绝不安全文件名和不允许扩展名；下载仍需鉴权。
8. 审计日志列表和导出对 password、token、secret、credential 等字段脱敏。
9. Prompt 注入可疑答案不会改变评分规则；评分结果带 `prompt_injection_suspected`，且 `needs_human_review=true`。
10. EXE 诊断页展示本地缓存安全检查结果。
11. 后端测试、前端类型检查和构建通过，或明确记录不可运行原因。

## Plan Review

1. 范围符合 STORY-039：只做第一轮安全加固，不提前进入 STORY-040 AI 评估框架或 STORY-041 E2E。
2. 显式需求均已纳入检查清单，并区分“已满足、需补强、剩余风险”。
3. 与当前仓库实际匹配：Go API 已有 RBAC、审计、租户参数和文件白名单；本 Story 在现有中间件、handler 与测试体系上加固，不重写架构。
4. 对生产影响可控：`/health` 保持公开以免破坏 Docker healthcheck；系统状态改为认证后访问；全局 body limit 采用配置化并保留文件上传专用限制。
5. 风险诚实披露：应用层租户隔离仍不是数据库 RLS；Web mock 登录仍是开发壳层；Prompt 注入只做基础检测，不声称解决全部 LLM 安全问题。

## Implementation

### 后端安全配置与中间件

1. `config` 新增 `SecurityConfig` 与登录失败限制配置：
   - `EDUGRADE_LOGIN_FAILURE_LIMIT`
   - `EDUGRADE_LOGIN_FAILURE_WINDOW`
   - `EDUGRADE_HTTP_MAX_HEADER_BYTES`
   - `EDUGRADE_MAX_REQUEST_BODY_BYTES`
   - `EDUGRADE_CORS_ALLOWED_ORIGINS`
   - `EDUGRADE_CORS_ALLOWED_METHODS`
   - `EDUGRADE_CORS_ALLOWED_HEADERS`
2. `cmd/api-gateway/main.go` 为 `http.Server` 设置 `MaxHeaderBytes`。
3. `middleware` 新增：
   - `SecurityHeaders`：设置 `nosniff`、`DENY` frame、`no-referrer`、`Permissions-Policy`、API CSP。
   - `CORS`：只允许配置 Origin，不反射未知 Origin。
   - `BodyLimit`：普通 JSON/表单请求使用全局 body limit；文件上传跳过全局小限制，继续使用文件专用上限。
4. `infra/docker-compose/docker-compose.yml` 将新增安全环境变量传入 `api-gateway` 容器。

### 鉴权与权限边界

1. 保留 `/health` 公开，用于 Docker/负载均衡健康检查。
2. 将 `/ready`、`/api/v1/system/info`、`/api/v1/system/status` 改为需要 `system:read`。
3. 新增 `auth.RequireAnyPermission` 和 `auth.HasPermission`，支持“管理员或本人”这类对象级授权。
4. 学生成绩接口：
   - `score:manage` 可读取管理范围内学生成绩。
   - 普通 `student:grade:read` 必须匹配 `user.data_scope.student_id`。
5. 增加阅卷员不能访问 `/api/v1/students` 的测试，确保阅卷权限不会泄露学生姓名。

### 登录失败限制与审计

1. 新增 `LoginFailureLimiter`，按 `tenant_code + username + remote_ip` 在窗口期内计数。
2. 超过限制返回 `429 login_rate_limited`，设置 `Retry-After`。
3. 登录失败和限流均写审计事件。
4. `remoteIP` 不再信任未经代理边界校验的 `X-Forwarded-For`，避免审计 IP 被客户端伪造。

### 审计日志脱敏与不可删除

1. 新增 `RedactAuditRecord(s)`，列表和 CSV 导出对 `password`、`token`、`secret`、`credential`、`authorization`、`api_key`、`session` 等字段脱敏。
2. 审计日志仍按业务事实写入 Store；脱敏发生在普通 API 输出边界。
3. 增加测试确认普通 DELETE 审计日志路由不存在。

### 文件安全

1. 保留既有文件下载鉴权、租户隔离、白名单类型校验和对象存储私有路径。
2. `CleanFilename` 增强：
   - 拒绝 Windows 设备保留名，如 `CON.pdf`。
   - 拒绝 `:`、`"`、`<`、`>`、`|` 等危险字符。
   - 继续移除路径前缀和控制字符。
3. 增加恶意文件名测试。

### Prompt 注入基础防护

1. 新增 `PromptGuard`：
   - 明确 `answer_text` 是不可信学生内容。
   - 为模型 adapter 输入附加守护说明。
2. `InspectPromptInjection` 检测常见覆盖评分规则、忽略指令、套取 system prompt、强制满分等模式。
3. `ApplyPromptGuard` 在模型输出中记录 `prompt_guard` 元数据。
4. 命中可疑注入时追加 `prompt_injection_suspected`，并强制 `needs_human_review=true`。
5. 继续保留模型输出 schema 校验，不允许无效输出进入成功评分结果。

### EXE 本地缓存安全检查

1. `localRuntime` 新增 `scanLocalCacheSecurity()`。
2. 仅扫描 `edugrade.desktop.*` 命名空间下的 localStorage，不读取或展示敏感值。
3. 检测疑似 `access_token`、`refresh_token`、`auth`、`session`、`password`、`secret`、`credential` 等缓存键或 JSON 字段。
4. 诊断页新增“缓存安全”状态，正常显示“未发现敏感缓存”，异常只显示键名和风险说明。

### Web 管理后台权限适配

1. `/system/status` 前端路由权限从 `system:manage` 对齐为后端 `system:read`。
2. mock session 增加 `system:read`，仅作为开发态 UI 权限；真实 API 仍依赖后端 Bearer token。

## Implementation Review

1. 所有 API 鉴权：`/health` 为明确例外；`/ready`、`system/info`、`system/status` 已加 `system:read`，测试覆盖 401/403。
2. tenant 隔离：本 Story 未改写 Store 层，但关键业务仍使用 `user.TenantID` 参数；文件、成绩、报告、审计测试继续通过。
3. 文件下载鉴权：保持 `file:manage + tenant_id` 读取，文件测试继续覆盖上传、下载、删除和元数据不暴露存储路径。
4. 学生成绩越权：新增路由测试确认学生只能读取自己 `data_scope.student_id` 的成绩。
5. 阅卷员学生姓名：新增组织路由测试确认无 `org:manage` 的阅卷权限不能列学生。
6. 密码 hash：bcrypt 逻辑未破坏，认证测试继续通过。
7. 登录失败限制：新增测试覆盖 401 到 429、`Retry-After` 和 `auth.login_rate_limited` 审计。
8. CORS：新增测试覆盖允许 Origin 与未知 Origin。
9. 请求体大小：新增测试覆盖普通 JSON 超限返回 413。
10. 文件类型白名单与恶意文件名：既有白名单测试继续通过，新增 `CON.pdf`、ADS 风格文件名等拒绝测试。
11. 审计不可删除：新增普通 DELETE 路由不存在测试。
12. 导出操作审计：既有成绩/报告/审计导出审计测试继续通过。
13. 敏感字段脱敏：新增审计列表和导出脱敏测试。
14. Prompt 注入：新增主观题测试确认可疑答案被标记并强制人工复核。
15. EXE 缓存安全：类型检查、生产构建和 Playwright 快照确认诊断页展示缓存安全状态。

## Fixes

1. 修正实现审阅发现的 Docker Compose 配置遗漏：新增安全环境变量写入 `api-gateway.environment`。
2. 修正 `CON.pdf` 保留名判断大小写问题：扩展名也统一转大写后截取基础名。
3. 补充审计日志不可删除测试，避免只凭路由人工检查得出结论。
4. 保持 `/health` 公开，避免“所有接口鉴权”的机械实现破坏容器健康检查。

## Verification

1. `gofmt -w ...`：通过。
2. `go test ./...`：通过。
3. `go build -o ..\..\bin\api-gateway.exe .\cmd\api-gateway`：通过。
4. `npm.cmd run typecheck`：通过；保留 npm `store-dir` 配置警告。
5. `npm.cmd run build`：通过；保留 Vite 大 chunk 警告。
6. `docker compose --env-file .\infra\docker-compose\.env.example -f .\infra\docker-compose\docker-compose.yml config`：通过。
7. `go test -race ./internal/auth ./internal/server`：未运行成功；当前 Windows 环境缺少 `gcc`，`runtime/cgo` 无法构建 race runtime。
8. Playwright 桌面端诊断页：
   - URL：`http://127.0.0.1:5180`
   - 截图：`output/playwright/story-039-desktop-cache-security.png`
   - 控制台 error：0。

## Approval

批准。

STORY-039 的显式要求均已完成第一轮落地：安全检查清单、代码加固、逐项说明、测试与剩余风险记录均已闭环。该 Story 不声称完成全部生产安全体系，但已经把当前仓库中最直接的鉴权、对象级授权、登录滥用、CORS、请求体、文件名、审计脱敏、Prompt 注入和 EXE 缓存安全风险推进到可验证状态。

### 剩余安全风险

1. 应用层租户隔离仍依赖所有 Store 正确传入 `tenant_id`，尚未启用 PostgreSQL Row-Level Security。
2. CORS allowlist 已实现，但生产域名、反向代理与 TLS/HSTS 策略仍需在真实部署环境复核。
3. 登录失败限制为进程内存实现，多实例部署需要 Redis/数据库集中限流。
4. Web 管理后台仍是开发态 mock session 壳层，真实 Web 登录与 token 生命周期需要后续 Story 产品化。
5. Prompt 注入防护是基础规则检测，不替代完整 LLM 安全网关、红队评估和模型侧结构化防护。
6. 文件安全未包含病毒扫描、宏检测、内容拆弹或 DLP。
7. 本地 EXE 缓存安全检查只能发现本应用命名空间下的明显敏感缓存键，不能替代 Windows DPAPI/Stronghold 等安全存储。
