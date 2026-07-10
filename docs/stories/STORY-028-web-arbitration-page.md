# STORY-028 Web 双评仲裁页面

## Plan

### 目标

实现 EduGrade Enterprise 的 Web 双评仲裁页面，面向仲裁员处理真实 `arbitration_task`。页面必须展示双评分差、原始答案上下文、OCR 文本、AI 建议、Rubric、两名阅卷员分数、分差原因，并通过真实后端接口提交仲裁最终分，提交成功后以后端返回的 `final_grade` 作为结果证据。

### 视觉与交互模型

- visual thesis：克制、紧凑、可追溯的仲裁控制台，用任务队列、证据对照和最终裁决形成稳定的三段式工作面。
- content plan：顶部状态与筛选；左侧仲裁任务列表；中间答案、OCR、AI 建议、Rubric 和双评分差详情；右侧最终分、仲裁说明、学生反馈和提交结果；底部审计状态说明。
- interaction thesis：筛选与选择任务不打断上下文；最终分输入始终贴近满分和双评分差；提交后刷新真实任务与最终成绩状态。

### 范围

- 新增 Web 仲裁页面并接入 `/arbitration` 路由。
- `/arbitration` 从 mock 占位页改为真实 API 页面。
- 新增前端 arbitration API 封装：
  - `GET /api/v1/arbitration-tasks`
  - `GET /api/v1/arbitration-tasks/{id}`
  - `POST /api/v1/arbitration-tasks/{id}/assign`
  - `POST /api/v1/arbitration-tasks/{id}/submit`
- 使用真实 API 展示仲裁任务列表字段：
  - exam
  - question_no
  - anonymous_code
  - reviewer A score
  - reviewer B score
  - difference
  - status
- 使用真实 API 展示详情字段：
  - original answer：`arbitration_task.context.raw_answer`
  - OCR text：`arbitration_task.context.ocr_text`
  - AI suggestion：`arbitration_task.context.ai_suggestion`
  - Rubric：通过 `listQuestions(exam_id)` 匹配 `question_id`
  - reviewer A score
  - reviewer B score
  - difference reason
  - arbitrator final score
  - arbitration explanation
  - student feedback
- 支持状态筛选、任务搜索和“我的任务”过滤；`assigned_to` 过滤调用真实查询参数。
- 支持手动分配给当前会话用户，调用真实 `assign` API；服务端仍会校验仲裁员不能等于前两名阅卷员。
- 支持提交仲裁结果，调用真实 `submit` API；响应中的 `final_grade` 必须在页面中展示。
- 前端校验最终分必须在 `0` 到题目满分之间；后端仍作为最终校验。
- 缺少 `edugrade.access_token` 时显示明确错误，不读取仲裁任务，不提交，不回退 mock 数据。
- 无 `arbitration:manage` 权限时路由层禁止访问；普通阅卷员不会进入该页面。
- 增补真实只读审计查询接口 `GET /api/v1/audit-logs`，支持仲裁页按 `target_type/target_id` 展示 `audit_log`。

### 非范围

- 不新增后端仲裁 API；沿用 STORY-019 已有真实接口。
- 不在本 Story 修改服务端“仲裁员只能读取自己分配任务”的强制访问模型。当前后端详情接口按租户和 `arbitration:manage` 权限隔离，列表支持 `assigned_to` 过滤，提交会校验已分配仲裁员。本 Story 会在前端默认提供“我的任务”过滤和文档中记录残余风险。
- 不伪造阅卷员 A/B 批注。当前 `arbitration_task` 只返回两名阅卷员分数，不返回 comments；页面只展示真实分数，并明确批注需要后续 API 支持。
- 不实现完整审计日志中心页面；`/audit` 仍属于后续 Story。
- 不实现三评、多仲裁员投票、阅卷员质量分析或成绩发布。
- 不实现真实 OCR/AI 聚合；仅展示后端仲裁上下文中已有字段。

### 修改文件

预计新增：

- `apps/web-admin/src/pages/ArbitrationPage.tsx`
- `apps/web-admin/src/api/auth.ts`
- `apps/web-admin/src/api/audit.ts`
- `docs/api/audit.md`
- `docs/stories/STORY-028-approval.md`

预计修改：

- `apps/web-admin/src/api/review.ts`
- `apps/web-admin/src/App.tsx`
- `apps/web-admin/src/router/routes.tsx`
- `apps/web-admin/src/styles.css`
- `services/api-gateway/internal/auth/audit.go`
- `services/api-gateway/internal/auth/types.go`
- `services/api-gateway/internal/auth/store_memory.go`
- `services/api-gateway/internal/auth/store_postgres.go`
- `services/api-gateway/internal/auth/handlers.go`
- `services/api-gateway/internal/auth/handlers_test.go`
- `services/api-gateway/internal/server/server.go`
- `services/api-gateway/migrations/000001_auth_rbac.sql`
- `apps/web-admin/README.md`
- `README.md`
- `docs/stories/README.md`

### 验收标准

- `/arbitration` 不再是 mock 占位页，路由标记为真实 API。
- 页面只调用真实仲裁 API；无 token 时明确失败，不回退 mock 业务数据。
- 任务列表展示 exam、question_no、anonymous_code、A/B 分数、分差和状态。
- 支持状态筛选、关键词搜索和 `assigned_to` 过滤。
- 详情展示原始答案、OCR 文本、AI 建议、Rubric、A/B 分数、分差原因、最终分输入和仲裁说明。
- 当前 API 没有阅卷员批注时，页面必须明确说明，不伪造 comments。
- 提交仲裁结果调用真实 `POST /api/v1/arbitration-tasks/{id}/submit`。
- 提交成功后显示真实 `final_grade`，并刷新任务详情。
- 前端执行 0 到满分校验；后端错误清晰展示。
- 普通阅卷员无 `arbitration:manage` 时不能访问页面。
- 仲裁员不能任意处理不该处理任务的后端限制以当前 API 能力为准；前端默认提供“我的任务”过滤，文档记录后端详情读取残余风险。
- 审计区域通过真实 `GET /api/v1/audit-logs` 展示当前仲裁任务与最终分相关审计记录。
- 前端 typecheck/build 通过。
- 后端 `go test ./...` 和 build 通过，确认本 Story 未破坏已有 API。
- Playwright 桌面和移动视口检查 `/arbitration` 无明显重叠、无整页横向滚动、无控制台错误。

## Plan Review

### 是否越界

未越界。计划只实现 Web 仲裁页面和前端 API 封装，不新增后端权限模型、不实现审计查询服务、不进入成绩发布、报告、申诉或三评流程。

### 是否遗漏显式需求

显式需求已覆盖：任务列表字段、详情字段、提交仲裁结果、写入 `final_grade` 的结果展示、审计提示、盲评期间普通阅卷员不可访问、仲裁员访问边界、全分值校验和真实 API 集成。

### 是否符合当前仓库实际

符合。后端已有 arbitration task 列表、详情、分配和提交接口，提交响应返回 `final_grade`。当前仓库原本没有审计查询 API，已纳入本 Story 的最小后端支撑；当前仍没有 A/B 阅卷批注字段，因此计划明确不伪造 comments。后端详情读取尚未强制 assigned-only，本 Story 不能宣称完全满足该安全边界，只能在前端默认过滤并记录剩余风险。

## Implementation

- 新增后端只读审计查询能力：
  - `auth.AuditRecord`、`auth.AuditFilter` 和 `Store.ListAudits`。
  - Memory/Postgres store 均按 tenant、action、actor、target_type、target_id、limit 查询。
  - `GET /api/v1/audit-logs` 路由要求 `audit:read`。
  - 迁移补充 `idx_audit_log_target` 索引。
  - 新增 auth handler 测试覆盖目标过滤和权限拒绝。
- 新增 `docs/api/audit.md`，记录审计查询接口边界和契约。
- 扩展 `apps/web-admin/src/api/review.ts`，新增 arbitration task 类型和列表、详情、分配、提交 API。
- 新增 `apps/web-admin/src/api/auth.ts`，用于通过真实 `GET /api/v1/auth/me` 获取当前后端用户 ID。
- 新增 `apps/web-admin/src/api/audit.ts`，封装真实审计日志查询。
- 新增 `apps/web-admin/src/pages/ArbitrationPage.tsx`：
  - 任务表展示考试、题号、匿名码、A/B 分数、分差和状态。
  - 默认“我的任务”过滤，使用真实后端用户 ID；读取失败时回退前端会话用户并提示。
  - 详情展示原始答案、OCR 文本、AI 建议、Rubric、双评分差和分差原因。
  - A/B 阅卷员批注因当前 API 不返回，页面明确显示“当前 API 未返回”，不伪造 comments。
  - 支持分配给我、最终分、仲裁说明、学生反馈和提交。
  - 提交成功后展示真实 `final_grade` 并刷新审计记录。
  - 审计面板读取真实 `audit_log`。
- `/arbitration` 路由从 mock 改为真实 API 页面，并要求 `arbitration:manage`、`exam:manage`、`audit:read`。
- 补充响应式样式，保证桌面和移动端无整页横向滚动。

## Implementation Review

- 任务列表字段：已覆盖 exam、question_no、anonymous_code、A/B score、difference、status。
- 详情字段：已覆盖 original answer、OCR text、AI suggestion、Rubric、A/B score、difference reason、final score、arbitration explanation、student feedback。
- 提交仲裁：调用真实 `POST /api/v1/arbitration-tasks/{id}/submit`，响应展示 `final_grade`。
- 审计记录：调用真实 `GET /api/v1/audit-logs` 按 arbitration task 和 final grade 查询。
- 普通阅卷员访问：前端路由要求 `arbitration:manage`，普通 review-only 用户无法进入。
- 仲裁员访问边界：前端默认“我的任务”过滤，提交端后端已校验 assigned_to；详情读取 assigned-only 仍是后端残余风险。
- 分数校验：前端按真实题目满分校验 0 到满分，后端仍做最终校验。
- 真实 API：缺 token 时显示明确错误，不读取任务、不提交、不回退 mock 数据。
- Playwright 桌面/移动检查通过：无整页横向滚动，控制台 0 errors、0 warnings。

## Fixes

- 第一轮实现审阅发现“显示审计记录”不能只做提示，否则仍不满足企业级需求；已补最小真实审计查询 API 和前端审计面板。
- 移动端审阅发现裁决面板不应继承任务列表的高度限制；已修正窄屏样式。

## Approval

Approved。

本 Story 已按计划完成，并通过后端测试、后端构建、前端类型检查、前端构建和 Playwright 桌面/移动检查。仍未实现的 A/B 阅卷员批注、完整审计中心和后端详情 assigned-only 强制读取已作为剩余风险记录，不使用 mock/stub 冒充真实能力。
