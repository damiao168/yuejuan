# STORY-032 Web 审计日志页面

## Plan

### 目标

实现 EduGrade Enterprise Web 管理后台的审计日志页面，让具备 `audit:read` 的审计员、管理员按租户查看真实 `audit_log`，按时间、操作人、操作类型、目标类型、考试和 IP 筛选，查看审计详情，并在具备 `audit:export` 时导出带水印的审计 CSV。审计日志不可编辑、不可删除，导出审计日志本身也必须写入审计。

### 视觉与交互模型

- visual thesis：冷静、密集、不可变的合规审计台，强调筛选、追踪和证据只读。
- content plan：筛选栏；审计摘要；只读日志表；详情抽屉/侧栏；导出权限与水印提示；租户隔离说明。
- interaction thesis：筛选立即读取真实 API；点击行查看详情；导出前二次确认并展示真实 watermark。

### 范围

- 新增 `/audit` 真实 API 页面，替换当前 mock 占位。
- 扩展后端审计查询能力：
  - 时间范围筛选。
  - IP 筛选。
  - exam_id 筛选：匹配 `target_type=exam && target_id=exam_id` 或 `target_id=exam_id`，用于当前真实审计模型中的考试级事件。
  - 返回 before_value / after_value 字段；已有事件未记录时为空，前端显示“未记录”。
- 新增后端审计导出接口：
  - `POST /api/v1/audit-logs/export`
  - 要求 `audit:export`。
  - 复用同一筛选条件导出 CSV。
  - 响应头返回 `X-EduGrade-Watermark`。
  - 导出操作写 `audit.exported` 审计。
- 前端审计页面支持：
  - 筛选：时间、操作人、操作类型、目标类型、考试、IP。
  - 列表字段：actor、action、target、reason、ip_address、created_at。
  - 详情字段：actor、action、target、before_value、after_value、reason、ip_address、user_agent、created_at、request_id、tenant_id。
  - 敏感字段脱敏显示：token、password、authorization、secret、credential 等字段在详情 JSON 和表格文本中脱敏。
  - 租户隔离说明：后端按 `user.TenantID` 查询，页面显示当前租户范围，不允许切换租户。
  - 不提供编辑、删除入口。
  - 无 `edugrade.access_token` 时显示明确错误，不读取审计、不导出、不回退 mock 数据。
- 更新文档和 Story 审批记录。

### 非范围

- 不实现审计日志删除、修改、归档或不可篡改链。
- 不实现跨租户审计查看；租户隔离由后端 `tenant_id` 强制。
- 不回填既有历史事件的 before_value/after_value。
- 不实现 PDF/XLSX 导出；本 Story 导出真实 CSV。
- 不实现完整用户管理页；actor 以 actor_id 展示，后续用户权限 Story 再扩展用户显示名映射。

### 修改文件

预计新增：
- `apps/web-admin/src/pages/AuditLogPage.tsx`
- `docs/stories/STORY-032-approval.md`

预计修改：
- `services/api-gateway/internal/auth/types.go`
- `services/api-gateway/internal/auth/handlers.go`
- `services/api-gateway/internal/auth/store_memory.go`
- `services/api-gateway/internal/auth/store_postgres.go`
- `services/api-gateway/internal/auth/handlers_test.go`
- `services/api-gateway/internal/server/server.go`
- `docs/api/audit.md`
- `apps/web-admin/src/api/audit.ts`
- `apps/web-admin/src/App.tsx`
- `apps/web-admin/src/auth/session.ts`
- `apps/web-admin/src/router/routes.tsx`
- `apps/web-admin/src/styles.css`
- `apps/web-admin/README.md`
- `README.md`
- `docs/stories/README.md`
- `docs/stories/STORY-032-web-audit-log-page.md`

### 验收标准

- `/audit` 不再是 mock 占位页，路由标记为真实 API。
- 缺少 `edugrade.access_token` 时页面显示明确错误，列表为空，导出禁用，不回退 mock 数据。
- 普通教师无 `audit:read` 时不能进入全局审计页面。
- 审计日志列表只读，不提供编辑/删除动作。
- 筛选支持时间、操作人、操作类型、目标类型、考试、IP。
- 详情展示 actor、action、target、before_value、after_value、reason、ip_address、user_agent、created_at。
- 敏感字段在详情中脱敏显示。
- 后端按当前用户 tenant_id 隔离查询。
- 导出按钮按 `audit:export` 权限启用/禁用。
- 导出审计日志调用真实 export API，返回 CSV 与真实 watermark。
- 导出操作本身写入 `audit.exported` 审计，并有测试覆盖。
- 前端 typecheck/build 通过。
- 后端 `go test ./...` 和 build 通过。
- Playwright 桌面和移动视口检查 `/audit` 无明显重叠、无整页横向滚动、无控制台错误。

## Plan Review

### 是否越界

未越界。计划只实现 Web 审计日志页面和为显式需求必需的最小后端审计查询/导出能力；不进入审计不可篡改链、归档、跨租户监管、用户管理或 PDF/XLSX 导出。

### 是否遗漏显式需求

显式需求已覆盖：审计日志列表、时间/操作人/操作类型/目标类型/考试/IP 筛选、详情字段、导出权限、敏感字段脱敏、租户隔离、不可编辑不可删除、普通教师不能查看全局审计、导出本身写审计、真实 API 对接。

### 是否符合当前仓库实际

基本符合。当前后端已有 `GET /api/v1/audit-logs` 和 `audit:read` 权限，但只支持 action/actor/target/limit 过滤，不支持导出和部分筛选；本 Story 将在 auth 审计模块中补最小扩展。当前 `audit_log` 主要记录 actor/action/target/reason/ip/user_agent/request_id，没有既有 before/after 历史值，因此页面将展示真实字段，未记录时明确显示“未记录”。

## Implementation

- 扩展后端审计模型：
  - `AuditEvent` / `AuditRecord` 增加 `before_value`、`after_value`。
  - `AuditFilter` 增加 `exam_id`、`ip_address`、`created_from`、`created_to`。
  - Memory/Postgres store 均支持扩展过滤与 before/after JSON 读写。
- 扩展后端审计 handler：
  - `GET /api/v1/audit-logs` 支持时间、IP、考试筛选。
  - 新增 `POST /api/v1/audit-logs/export`，要求 `audit:export`。
  - 导出 CSV 返回 `X-EduGrade-Watermark`，并写入 `audit.exported` 审计。
- 新增迁移 `000018_audit_log_export.sql`：
  - 增加 `audit:export` 权限。
  - 授权 platform_admin、tenant_admin、auditor。
  - 增加审计时间与 IP 索引。
- 补充后端测试：
  - IP/exam/before/after 查询。
  - 导出 CSV、水印和 `audit.exported` 写入。
- 扩展 `apps/web-admin/src/api/audit.ts`：
  - 支持扩展筛选参数。
  - 新增 `exportAuditLogs`。
- 新增 `apps/web-admin/src/pages/AuditLogPage.tsx`：
  - 真实读取审计列表。
  - 支持时间、操作人、操作类型、目标类型、考试、IP、目标 ID 和页内关键词筛选。
  - 点击行打开详情抽屉，展示 actor、action、target、before_value、after_value、reason、ip_address、user_agent、created_at、request_id、tenant_id。
  - 对 password、token、authorization、secret、credential 等敏感字段脱敏显示。
  - 导出前二次确认，调用真实 export API，导出后展示真实 watermark。
  - 页面明确只读，不提供编辑/删除入口。
  - 显示租户隔离说明，不提供跨租户切换。
  - 缺少 `edugrade.access_token` 时显示明确错误，不读取审计、不导出、不回退 mock 数据。
- `/audit` 路由从 mock 改为真实 API 页面，要求 `audit:read`；导出按钮按 `audit:export` 控制。
- mock session 补充 `audit:export`。
- 更新 `docs/api/audit.md`、README 和 Story 索引。

## Implementation Review

- `/audit` 真实 API 页面：已通过 `routes.tsx mock=false` 和 `AuditLogPage` 真实 API 调用确认。
- 缺 token 行为：Playwright 在登录壳下清除 `edugrade.access_token` 后进入 `/audit`，页面显示明确错误，列表为空，导出禁用，无 mock 审计数据。
- 普通教师访问：路由要求 `audit:read`，普通教师无全局审计权限时进入 Forbidden。
- 只读要求：页面没有编辑/删除按钮；后端未提供编辑/删除接口。
- 筛选项：时间、actor、action、target_type、exam_id、ip_address、target_id 均接入真实 API。
- 详情字段：actor、action、target、before_value、after_value、reason、ip_address、user_agent、created_at 均展示；未记录 before/after 时显示“未记录”。
- 敏感字段脱敏：详情 JSON 和文本展示会递归脱敏 password/token/authorization/secret/credential 等字段。
- 租户隔离：后端 ListAudits/ExportAudits 均使用当前用户 `TenantID`；页面显示租户范围且不能切换租户。
- 导出权限：导出路由要求 `audit:export`，前端按钮按权限和 token 控制。
- 导出审计：后端测试覆盖 `audit.exported` 写入和水印。
- 前端 typecheck/build、后端 `go test ./...` 和后端 build 均通过。
- Playwright 桌面/移动检查通过：控制台 0 errors、0 warnings，整页无横向溢出。

## Fixes

- 实现审阅发现 5173 dev server 有旧 Vite HMR 模块图，仍请求修改前的 `audit.ts`，导致空白页；已启动干净的 `http://127.0.0.1:5174/` 验证 `/audit`。
- 实现审阅发现 Ant Design 日期范围默认字符串不是后端支持格式；已改为 `toISOString()` 传入 `created_from` / `created_to`。

## Approval

Approved。

本 Story 已按计划完成，并通过后端测试、后端构建、前端类型检查、前端构建和 Playwright 桌面/移动检查。当前剩余边界：不实现审计不可篡改链、归档、跨租户监管或 PDF/XLSX 导出；既有事件未写入 before/after 时页面显示“未记录”。上述边界均已记录，不使用 mock/stub 冒充真实能力。
