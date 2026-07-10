# STORY-029 Web 成绩管理与发布页面

## Plan

### 目标

实现 EduGrade Enterprise 的 Web 成绩管理与发布页面，面向学科组长、年级主任和管理员完成最终成绩汇总、发布前质量检查、成绩确认、成绩发布、CSV 导出与审计提示。页面必须对接真实后端 API，不能用 mock 成绩、mock 质量检查或假导出记录。

### 视觉与交互模型

- visual thesis：安静、严肃、可复核的成绩发布控制台，用质量闸门和锁定状态压住高风险操作。
- content plan：顶部考试选择与状态；总览指标；发布前质量检查；成绩表；右侧操作与审计/水印提示。
- interaction thesis：质量检查结果直接决定发布按钮状态；确认和发布都要求明确原因与二次确认；导出前提示审计与水印，导出后显示真实 watermark。

### 范围

- 新增 Web 成绩管理与发布页面并接入 `/scores` 路由。
- `/scores` 从 mock 占位页改为真实 API 页面。
- 新增前端 scores API 封装：
  - `POST /api/v1/exams/{examId}/finalize`
  - `GET /api/v1/exams/{examId}/grades`
  - `GET /api/v1/exams/{examId}/grades/quality`
  - `POST /api/v1/exams/{examId}/confirm-grades`
  - `POST /api/v1/exams/{examId}/publish`
  - `GET /api/v1/exams/{examId}/grades/export`
- 增补后端只读质量检查接口 `GET /api/v1/exams/{examId}/grades/quality`，让 Web 页面能在发布前真实判断按钮启用/禁用。
- 使用真实 API 组成成绩总览：
  - 考试总人数：`listSubmissions(examId)`。
  - 已完成阅卷人数：`listExamGrades(examId)`。
  - 未完成任务、待仲裁、OCR 失败、异常分数：quality issue counts。
  - 是否可发布：publish-stage quality passed。
- 使用真实 API 展示成绩列表：
  - anonymous_code。
  - 学生姓名：仅当当前前端用户具备 `org:manage` 时，调用 org API 读取学生；否则显示脱敏占位。
  - 班级：仅当具备 `org:manage` 时，调用 org class API 映射。
  - 总分、各题分、状态、锁定状态。
- 支持操作：
  - 汇总最终成绩。
  - 确认成绩。
  - 发布成绩，必须二次确认。
  - 导出 CSV，必须先提示导出审计和水印。
- 支持发布后锁定提示。
- 支持读取成绩相关 `audit_log`。若无 `audit:read` 权限，不伪造审计列表，只显示无审计读取权限提示。
- 缺少 `edugrade.access_token` 时显示明确错误，不读取成绩、不提交、不导出，不回退 mock 数据。

### 非范围

- 不实现 Excel `.xlsx` 二进制导出；当前后端真实能力是 CSV。
- 不实现学生端成绩页面；学生查询 API 已在 STORY-020 实现。
- 不实现成绩排名、等级分、报告图表或发布通知。
- 不实现申诉改分；申诉中心属于后续 Web Story。
- 不扩展成绩表直接返回学生姓名/班级；本 Story 用已有 org API 做权限受控的前端映射。
- 不实现完整 `/audit` 审计中心页面。

### 修改文件

预计新增：

- `apps/web-admin/src/api/scores.ts`
- `apps/web-admin/src/pages/ScoreManagementPage.tsx`
- `docs/stories/STORY-029-approval.md`

预计修改：

- `services/api-gateway/internal/score/types.go`
- `services/api-gateway/internal/score/handlers.go`
- `services/api-gateway/internal/score/store_memory.go`
- `services/api-gateway/internal/score/store_postgres.go`
- `services/api-gateway/internal/score/store_test.go`
- `services/api-gateway/internal/server/server.go`
- `services/api-gateway/internal/server/score_route_test.go`
- `docs/api/scores.md`
- `apps/web-admin/src/App.tsx`
- `apps/web-admin/src/router/routes.tsx`
- `apps/web-admin/src/styles.css`
- `apps/web-admin/README.md`
- `README.md`
- `docs/stories/README.md`

### 验收标准

- `/scores` 不再是 mock 占位页，路由标记为真实 API。
- 缺少 token 时页面显示明确错误，所有写操作与导出禁用，不回退 mock 数据。
- 页面可选择真实考试并读取真实 submissions、submission grades、quality report。
- 总览展示考试总人数、已完成阅卷人数、未完成任务、待仲裁、OCR 失败、异常任务和是否可发布。
- 发布按钮根据 `GET /grades/quality?stage=publish` 结果启用/禁用。
- 成绩列表展示 anonymous_code、学生姓名/脱敏占位、班级/脱敏占位、总分、各题分、状态和锁定状态。
- 确认成绩调用真实 confirm API。
- 发布成绩必须二次确认，并调用真实 publish API。
- 导出成绩前提示审计和水印，导出调用真实 CSV endpoint，并显示真实 header watermark。
- 发布后显示锁定提示。
- 审计面板读取真实 `audit_log` 或明确说明无权限/暂无记录，不伪造。
- 后端新增质量检查接口有测试覆盖。
- 前端 typecheck/build 通过。
- 后端 `go test ./...` 和 build 通过。
- Playwright 桌面和移动视口检查 `/scores` 无明显重叠、无整页横向滚动、无控制台错误。

## Plan Review

### 是否越界

未越界。计划只实现 Web 成绩管理与发布页面以及必要的只读质量检查后端接口，不进入报告、申诉、学生端、排名、通知或完整审计中心。

### 是否遗漏显式需求

已覆盖成绩总览、成绩列表、发布前质量检查、确认成绩、发布成绩、导出成绩、发布后锁定提示、导出审计与水印提示、发布按钮按质量检查启用/禁用、发布二次确认、真实 API 对接。

### 是否符合当前仓库实际

符合。后端已有成绩汇总、查询、确认、发布、导出和审计写入；当前缺少只读质量检查接口，本 Story 将补最小 API。学生姓名和班级当前不在 score API 中返回，但已有 org API 可在有权限时映射；无权限时必须脱敏显示。

## Implementation

- 新增后端只读成绩质量检查接口：
  - `score.Store.CheckQuality(ctx, tenantID, examID, requirePendingPublish)`。
  - Memory/Postgres store 复用现有质量检查逻辑，支持确认阶段与发布阶段门禁。
  - `GET /api/v1/exams/{examId}/grades/quality` 路由要求 `score:manage`。
  - `docs/api/scores.md` 补充质量检查接口契约。
- 新增后端测试：
  - `score/store_test.go` 覆盖只读质量检查与发布门禁。
  - `server/score_route_test.go` 覆盖确认前后 `can_publish` 的变化。
- 新增前端 `apps/web-admin/src/api/scores.ts`：
  - 封装 finalize、grades list、quality、confirm、publish、export CSV。
  - 导出接口读取真实 `X-EduGrade-Watermark` 响应头。
- 新增 `apps/web-admin/src/pages/ScoreManagementPage.tsx`：
  - 读取真实考试、submission、final grades、quality、org student/class、audit_log API。
  - 无 `edugrade.access_token` 时显示明确错误，禁用汇总、确认、发布、导出，不回退 mock 数据。
  - 成绩总览展示考试总人数、已完成阅卷人数、未完成任务、待仲裁、OCR 失败、异常任务和是否可发布。
  - 成绩列表展示 anonymous_code、按权限显示学生姓名/班级、总分、各题分、状态和锁定状态。
  - 发布前质量检查结果直接控制发布按钮启用/禁用。
  - 确认成绩、发布成绩调用真实 API；发布必须二次确认。
  - 导出 CSV 前提示审计与水印，导出后展示真实 watermark。
  - 审计面板仅读取真实 `score.*` audit_log；无权限或无记录时明确说明。
- `/scores` 路由从 mock 占位改为真实 API 页面，要求 `score:manage`、`exam:manage`、`submission:manage`。
- 更新响应式样式，保证桌面和移动端无整页横向滚动。
- 更新 `README.md`、`apps/web-admin/README.md` 和 Story 索引状态。

## Implementation Review

- `/scores` 真实 API 页面：已通过 `routes.tsx mock=false` 和 `ScoreManagementPage` 真实 API 调用确认。
- 缺 token 行为：Playwright 在登录壳下清除 `edugrade.access_token` 后进入 `/scores`，页面显示明确错误，成绩表无 mock 行，汇总/确认/发布/导出均禁用。
- 成绩总览：由 submissions、grades 和 quality report 计算，不伪造数据。
- 成绩列表：展示 anonymous_code、学生姓名/班级权限态、总分、各题分、状态和锁定状态。
- 发布质量门禁：`GET /grades/quality?stage=publish` 返回 `can_publish`，发布按钮按结果启用/禁用。
- 确认与发布：分别调用真实 confirm/publish API；发布有二次确认。
- 导出审计与水印：导出前确认提示，导出使用真实 CSV endpoint，并读取真实 watermark header。
- 审计记录：只展示真实 `score.*` audit_log；无权限或无记录不伪造。
- 后端测试、后端构建、前端类型检查、前端构建均通过。
- Playwright 桌面 1280 宽与移动 390 宽检查通过：控制台 0 errors、0 warnings，整页无横向溢出。

## Fixes

- 实现审阅发现审计空态判断使用了未过滤的 `auditLogs.length`，可能在只有非 score 审计时显示空列表；已改为 `scoreAuditLogs` 派生列表并统一用于空态与列表数据源。
- 实现审阅发现质量检查尚未返回时圆形进度会显示非 0 的推断值；已改为无质量结果时显示 0，避免制造伪进度。

## Approval

Approved。

本 Story 已按计划完成，并通过后端测试、后端构建、前端类型检查、前端构建和 Playwright 桌面/移动检查。当前仍保留的边界是：导出格式为现有后端真实 CSV 而非 XLSX；学生姓名和班级仅在当前用户具备 `org:manage` 时通过 org API 映射；Vite 构建仍有既有大 chunk warning。上述边界均已记录，不使用 mock/stub 冒充真实能力。
