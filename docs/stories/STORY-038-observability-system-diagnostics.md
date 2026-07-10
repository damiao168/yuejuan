# STORY-038 可观测性与系统诊断

## Plan

### 目标

为 EduGrade Enterprise 补齐第一轮可观测性与系统诊断能力，覆盖后端结构化日志、请求追踪、服务状态接口、管理后台系统状态页和 Windows EXE 客户端诊断页。

### 范围

1. 后端 API Gateway：
   - 保留并增强 JSON 结构化日志。
   - 为请求链路加入 `request_id` 与 `trace_id`，同时写入响应头。
   - API 请求日志包含方法、路径、状态码、耗时、客户端地址、`request_id`、`trace_id`。
   - 错误日志包含足够排障上下文，但过滤答卷原文、学生隐私、分数等敏感字段。
   - 加入慢请求日志预留，按环境变量配置阈值，输出明确的系统日志事件。
   - 扩展 `/ready` 与新增 `/api/v1/system/status`，返回 DB、Redis、MinIO、Qdrant、AI 服务状态。
   - 保持审计日志和系统日志的职责分离说明，普通系统日志不替代业务审计。
2. Web 管理后台：
   - 新增系统状态页面，调用真实系统状态 API。
   - 展示依赖状态、运行信息、日志策略、审计/系统日志边界。
   - 不用 mock 数据冒充真实状态；接口不可用时显示真实错误。
3. Windows EXE 客户端：
   - 增强系统诊断页，展示服务端连接、当前版本、本地缓存、上传队列和最近错误日志。
   - 调用真实后端状态接口；不可用时明确显示失败原因。
4. 文档与测试：
   - 补充系统状态 API 文档。
   - 增加或更新后端测试、前端类型检查与构建验证。

### 非范围

1. 不接入完整 OpenTelemetry Collector、Grafana Dashboard 或真实分布式 Trace 后端。
2. 不实现数据库层真实慢 SQL 捕获，只做慢请求/慢查询预留说明和配置。
3. 不实现生产级日志采集 Agent 或日志中心。
4. 不把审计日志改为系统日志，也不删除现有审计 API。

### 预计修改文件

1. `docs/stories/README.md`
2. `services/api-gateway/internal/config/config.go`
3. `services/api-gateway/internal/logger/logger.go`
4. `services/api-gateway/internal/middleware/middleware.go`
5. `services/api-gateway/internal/deps/deps.go`
6. `services/api-gateway/internal/handlers/handlers.go`
7. `services/api-gateway/internal/httpx/response.go`
8. `services/api-gateway/internal/server/server.go`
9. `services/api-gateway/internal/server/server_test.go`
10. `apps/web-admin/src/api/system.ts`
11. `apps/web-admin/src/pages/SystemStatusPage.tsx`
12. `apps/web-admin/src/router/routes.tsx`
13. `apps/web-admin/src/App.tsx`
14. `apps/web-admin/src/styles.css`
15. `apps/desktop-client/src/api/client.ts`
16. `apps/desktop-client/src/App.tsx`
17. `docs/api/system-status.md`

### 验收标准

1. 后端日志输出为 JSON，并自动附带 `request_id` 与 `trace_id`。
2. 响应头包含 `X-Request-ID` 与 `X-Trace-ID`。
3. `/api/v1/system/status` 返回整体状态、运行信息、依赖检查结果和日志策略。
4. 服务状态覆盖 Postgres、Redis、MinIO、Qdrant、AI 服务；未配置的外部服务明确显示 `not_configured`。
5. 普通系统日志不会记录答卷原文、学生姓名、学号、分数等敏感字段。
6. Web 管理后台有系统状态页面，并调用真实 API。
7. EXE 系统诊断页展示服务端连接、版本、本地缓存、上传队列、最近错误日志。
8. 审计日志和系统日志的分离边界在页面或文档中明确说明。
9. 后端测试通过；前端类型检查和构建通过，或明确记录不可运行原因。

## Plan Review

1. 范围没有越过 Story 038：只做可观测性、状态接口、诊断页面和相关文档测试，不提前进入 Story 039 安全加固。
2. 显式需求全部覆盖：结构化日志、`request_id`/`trace_id`、API 日志、错误日志、慢查询预留、健康检查、五类依赖状态、Web 状态页、EXE 诊断页、审计/系统日志分离均已纳入。
3. 与当前仓库匹配：后端已有 JSON Logger、`/health`、`/ready` 和依赖 Checker；Web 已有 Ant Design 路由结构；EXE 已有基础诊断页和本地日志能力，本 Story 在这些基础上增强。
4. 风险控制：外部 Qdrant/AI 服务可能未配置，状态接口必须返回明确 `not_configured`，不能伪装健康。

## Implementation

### 后端

1. `config` 增加 Qdrant、AI 服务与可观测性配置：
   - `EDUGRADE_QDRANT_URL`
   - `EDUGRADE_QDRANT_API_KEY`
   - `EDUGRADE_AI_SERVICE_URL`
   - `EDUGRADE_SLOW_REQUEST_THRESHOLD`
2. `logger` 增强：
   - 结构化 JSON 日志自动写入 `request_id` 与 `trace_id`。
   - 增加 `Warn` 日志级别。
   - 系统日志字段按敏感 key 脱敏，覆盖 `answer`、`score`、`student`、`token`、`secret` 等字段。
3. `middleware` 增强：
   - 请求中间件支持 `X-Request-ID`、`X-Trace-ID` 和 W3C `traceparent`。
   - 响应头写回 `X-Request-ID` 与 `X-Trace-ID`。
   - Access Log 记录系统日志事件 `api_request`。
   - 慢请求日志预留事件 `slow_request`，同时标明数据库慢查询捕获未实现。
   - Panic 恢复日志只记录 panic 类型和 stack，不记录 panic 原文。
4. `deps` 增强：
   - 增加 `not_configured` 状态。
   - 增加 Qdrant `/healthz` 检查。
   - 增加 AI 服务 `/health` 检查。
   - 依赖检查结果包含耗时与检查时间。
5. `handlers` 增强：
   - `/health` 增加 `health: healthy`，兼容 Docker 健康检查。
   - 新增 `/api/v1/system/status`，返回整体状态、运行信息、依赖状态和日志策略。
   - `/api/v1/system/info` 增加可观测性相关 capabilities。
6. `httpx` 错误响应增加 `trace_id`。

### Web 管理后台

1. 新增 `apps/web-admin/src/api/system.ts`。
2. 新增 `apps/web-admin/src/pages/SystemStatusPage.tsx`。
3. 路由新增 `/system/status`，导航名称为“系统状态”。
4. 页面调用真实 `/api/v1/system/status`；后端不可用时显示真实错误，不使用 mock 数据。
5. 页面展示依赖状态、运行信息、日志与追踪策略、审计日志/系统日志边界。

### Windows EXE 客户端

1. API 客户端新增 `systemStatus()`。
2. 诊断页增强：
   - 服务端连接与系统状态检查。
   - 当前版本与运行时信息。
   - 本地缓存大小估算。
   - 加密离线草稿 envelope 数量。
   - 上传队列 pending/uploading/failed 摘要。
   - 最近 error 级本地日志。
3. 诊断页继续明确标记未配置能力，不把 SQLite、安全存储、自动更新伪装为已实现。

### 前端兼容修正

在 Web 与 EXE 入口接入 `@ant-design/v5-patch-for-react-19`，消除 Ant Design 在 React 19 下的运行时兼容提示。

## Implementation Review

1. 结构化日志：通过 logger 测试验证 JSON 日志包含 `request_id` 与 `trace_id`。
2. API 请求日志：通过 server 测试验证 Access Log 包含关联 ID、`api_request` 和 `system` 日志流。
3. 错误日志与敏感数据：通过 logger 测试验证学生字段、答案字段、分数字段会被脱敏。
4. 慢查询预留：后端输出 `slow_request` 与 `slow_query_log_placeholder`，系统状态接口返回 `slow_query_log: placeholder_not_implemented`。
5. 健康检查与服务状态：`/health`、`/ready` 保留；`/api/v1/system/status` 新增并覆盖依赖状态。
6. Web 页面：Playwright 打开 `/system/status`，在 API 未启动时显示真实 `Failed to fetch` 错误态，无布局重叠。
7. EXE 诊断页：Playwright 打开系统诊断页，新增服务端、本地缓存、队列、错误日志面板，无布局重叠。
8. 审计/系统日志分离：Web 页面、EXE 页面和 API 文档均标明系统日志不替代 audit_log。

## Fixes

1. 修正 `Recover` 日志：不记录 panic 原文，只记录 panic 类型和 stack。
2. 扩展敏感字段脱敏：字段名包含 `student` 时也统一脱敏。
3. 修正前端 React 19 + Ant Design 控制台兼容提示：新增官方 patch 包并在 Web/EXE 入口导入。
4. 修正 Playwright 截图命令参数，使用当前 CLI 支持的 `--filename`。

## Verification

1. `gofmt -w ...`：通过。
2. `go test ./...`：通过。
3. `go build -o ..\..\bin\api-gateway.exe .\cmd\api-gateway`：通过。
4. `npm.cmd run typecheck`：通过。
5. `npm.cmd run build`：通过。
6. `docker compose --env-file .\infra\docker-compose\.env.example -f .\infra\docker-compose\docker-compose.yml config`：通过。
7. Playwright Web 截图：`output/playwright/story-038-web-system-status.png`。
8. Playwright EXE 截图：`output/playwright/story-038-desktop-diagnostics-after-patch.png`。

## Approval

批准。

Story 038 的显式需求均已覆盖：后端可观测性、请求追踪、API/错误日志、慢查询预留、健康检查、五类依赖状态、Web 系统状态页、EXE 诊断页、审计日志与系统日志分离、敏感数据日志防护和验证记录均已完成。

剩余风险：

1. 当前没有真实 PostgreSQL/Redis/MinIO/Qdrant/AI 服务联调环境，依赖状态接口已通过单元测试和 Docker Compose 配置验证，但未在真实私有化环境跑通。
2. 慢查询仍是预留标识，真实 SQL 级慢查询采集需要后续 APM 或数据库驱动插桩。
3. Vite 构建仍提示大 chunk 警告，属于既有包体积问题，建议后续做代码拆分。
