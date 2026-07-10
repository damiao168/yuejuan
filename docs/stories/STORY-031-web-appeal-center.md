# STORY-031 Web 申诉中心

## Plan

### 目标

实现 EduGrade Enterprise Web 管理后台的申诉中心页面，让教师、仲裁员和管理员通过 `/appeals` 查看真实申诉列表、申诉详情、证据快照、历史改分记录、审计记录，并按权限处理申诉。改分必须二次确认并调用真实后端 API，由后端写 `score_adjustment` 和 `audit_log`。

### 视觉与交互模型

- visual thesis：严肃、可追溯、证据优先的申诉处理台，列表用于分流，详情用于复核和留痕。
- content plan：申诉筛选与统计；申诉列表；详情证据；处理表单；历史改分；审计记录；学生可见处理结果。
- interaction thesis：选中申诉后读取真实详情和审计；状态处理保持轻量；调整分数必须弹出二次确认，确认内容包含原最终分和调整分。

### 范围

- 新增 `/appeals` 真实 API 页面，替换当前 mock 占位。
- 新增前端 appeals API 封装：
  - `GET /api/v1/appeals`
  - `GET /api/v1/appeals/{id}`
  - `POST /api/v1/appeals/{id}/review`
  - `POST /api/v1/appeals/{id}/close`
  - `GET /api/v1/appeals/statistics`
- 申诉列表展示：
  - 学生：有 `org:manage` 时映射学生姓名；否则显示 student_id 和权限受限提示。
  - 班级：有 `org:manage` 时通过学生 class_id 映射；否则显示权限受限。
  - 考试：有 `exam:manage` 时映射考试名称；否则显示 exam_id。
  - 题号、申诉原因、状态、提交时间。
- 申诉详情展示：
  - 学生申诉内容。
  - 原始答卷、OCR。
  - AI 评分、人工评分。
  - 最终分。
  - Rubric。
  - 历史修改记录。
  - 学生可见处理结果。
- 支持处理申诉：
  - 接受。
  - 驳回。
  - 要求补充。
  - 调整分数。
  - 关闭申诉。
  - 处理说明必填。
- 调整分数必须二次确认，提交真实 `review` API。
- 审计记录：
  - 有 `audit:read` 时读取真实 `audit_log`，展示 `appeal.*` 与相关 `score_adjustment` 审计。
  - 无权限时明确说明，不伪造审计。
- 缺少 `edugrade.access_token` 时显示明确错误，不读取申诉、不处理、不回退 mock 数据。
- 学生只能看自己的申诉由后端 scope 强制；Web 管理后台仅实现管理侧页面，不伪造学生端。
- 更新 mock session 增加 `appeal:read`，以匹配真实后端列表/详情路由权限。
- 更新文档和 Story 审批记录。

### 非范围

- 不实现学生端申诉提交/查看页面。
- 不实现附件预览下载；本 Story 只展示 attachment 元数据。
- 不实现图片级原卷渲染；当前 appeal evidence 只有后端已存的 raw answer/OCR/评分/Rubric 快照。
- 不新增后端申诉统计或证据字段，除非实现审阅证明 Web 显式需求无法由现有 API 满足。
- 不实现通知、消息、申诉 SLA、批量处理或多级审批。
- 不让前端自行改分或写审计；改分和审计必须由真实后端完成。

### 修改文件

预计新增：
- `apps/web-admin/src/api/appeals.ts`
- `apps/web-admin/src/pages/AppealCenterPage.tsx`
- `docs/stories/STORY-031-approval.md`

预计修改：
- `apps/web-admin/src/App.tsx`
- `apps/web-admin/src/auth/session.ts`
- `apps/web-admin/src/router/routes.tsx`
- `apps/web-admin/src/styles.css`
- `apps/web-admin/README.md`
- `README.md`
- `docs/stories/README.md`
- `docs/stories/STORY-031-web-appeal-center.md`

### 验收标准

- `/appeals` 不再是 mock 占位页，路由标记为真实 API。
- 缺少 `edugrade.access_token` 时页面显示明确错误，列表为空，处理按钮禁用，不回退 mock 数据。
- 列表展示学生、班级、考试、题号、申诉原因、状态、提交时间；无法映射身份时明确权限受限。
- 筛选支持考试、状态和关键词。
- 详情展示学生申诉内容、原始答卷、OCR、AI 评分、人工评分、最终分、Rubric、历史修改记录。
- 处理申诉调用真实 review API，支持接受、驳回、要求补充、调整分数。
- 调整分数必须二次确认，并带处理说明。
- 关闭申诉调用真实 close API。
- 学生可见处理结果以 `status`、`result_reason`、reviewed_at/closed_at 展示。
- 有 `audit:read` 时展示真实申诉审计；无权限时明确说明。
- 前端 typecheck/build 通过。
- 后端 `go test ./...` 和 build 通过，确认未破坏已有申诉 API。
- Playwright 桌面和移动视口检查 `/appeals` 无明显重叠、无整页横向滚动、无控制台错误。

## Plan Review

### 是否越界

未越界。计划只实现 Web 管理侧申诉中心和前端 API 封装，复用 STORY-021 已完成的真实申诉后端 API；不进入学生端、通知、多级审批、附件预览或新的后端证据采集。

### 是否遗漏显式需求

显式需求已覆盖：申诉列表字段、申诉详情字段、接受/驳回/要求补充/调整分数、处理说明、学生可见处理结果、审计记录、学生只能看自己的申诉、教师/管理员按权限处理、改分二次确认、改分写审计和真实 API 对接。

### 是否符合当前仓库实际

符合。后端已有 appeal list/detail/review/close/statistics，详情返回 evidence 与 adjustments；review 的 `score_adjusted` 会创建 `score_adjustment`、更新最终分并由 handler 写审计。当前 appeal API 不直接返回学生姓名、班级名、考试名和原卷图片，因此本 Story 使用已有 org/exam API 做权限受控映射，并对不可得字段显示明确边界。

## Implementation

- 新增 `apps/web-admin/src/api/appeals.ts`：
  - 定义 Appeal、AppealEvidence、ScoreAdjustment、AppealStatistics 和 review payload 类型。
  - 封装 list/detail/review/close/statistics 真实 API。
- 新增 `apps/web-admin/src/pages/AppealCenterPage.tsx`：
  - 读取真实申诉列表、申诉详情、statistics 和 audit_log。
  - 通过 org/exam API 在有权限时映射学生、班级和考试名称；无权限时明确显示权限受限或 ID。
  - 列表展示学生、班级、考试、题号、申诉原因、状态和提交时间。
  - 详情展示学生申诉内容、附件元数据、原始答卷、OCR、AI 评分、人工评分、最终分、Rubric、学生可见处理结果和历史改分记录。
  - 支持接受、驳回、要求补充、标记处理中和调整分数。
  - 调整分数必须二次确认，确认文案包含当前最终分和调整后分数。
  - 支持关闭申诉，调用真实 close API。
  - 终态申诉禁用继续 review，避免前端尝试重开终态流程。
  - 有 `audit:read` 时读取真实 appeal 与 score_adjustment 审计；无权限或无记录时明确说明。
  - 缺少 `edugrade.access_token` 时显示明确错误，列表为空，处理禁用，不回退 mock 数据。
- `/appeals` 路由从 mock 改为真实 API 页面，路由要求 `appeal:read`，处理动作按 `appeal:manage` 控制。
- mock session 补充 `appeal:read`，匹配真实后端 list/detail 路由权限。
- 新增响应式样式，桌面三栏、移动单列，表格使用内部横向滚动而不造成整页溢出。
- 更新 `README.md`、`apps/web-admin/README.md` 和 Story 索引状态。

## Implementation Review

- `/appeals` 真实 API 页面：已通过 `routes.tsx mock=false` 和 `AppealCenterPage` 真实 API 调用确认。
- 缺 token 行为：Playwright 在登录壳下清除 `edugrade.access_token` 后进入 `/appeals`，页面显示明确错误，列表为空，处理按钮禁用，无 mock 申诉数据。
- 申诉列表字段：已覆盖学生、班级、考试、题号、申诉原因、状态、提交时间。
- 身份映射：使用真实 org/exam API；无权限或无数据时不伪造姓名、班级或考试名。
- 申诉详情：已覆盖学生申诉内容、附件元数据、原始答卷、OCR、AI 评分、人工评分、最终分、Rubric、历史修改记录。
- 处理申诉：调用真实 review API，支持接受、驳回、要求补充、调整分数和处理中。
- 改分二次确认：`score_adjusted` 提交前弹出确认，确认后由后端创建 `score_adjustment` 并写审计。
- 学生可见处理结果：展示 status、result_reason 和 reviewed_at/closed_at。
- 审计记录：调用真实 `GET /api/v1/audit-logs` 读取 appeal 与 score_adjustment 审计；无权限或无记录时明确说明。
- 学生只能看自己的申诉：由后端 data_scope 强制，页面文档和权限面板明确说明。
- 前端 typecheck/build、后端 `go test ./...` 和后端 build 均通过。
- Playwright 桌面/移动检查通过：控制台 0 errors、0 warnings，整页无横向溢出。

## Fixes

- 实现审阅发现当前浏览器热更新保留旧 mock session 时会误判缺少 `appeal:read`；已刷新验证，并将 `appeal:read` 写入 mock session 源码。
- 实现审阅发现终态申诉不应继续开放 review 提交；已在前端禁用终态 review。
- 实现审阅发现原始答卷/OCR 为字符串时不应以 JSON 引号展示；已改为字符串直接展示，结构化评分/Rubric 仍用 JSON 快照。

## Approval

Approved。

本 Story 已按计划完成，并通过后端测试、后端构建、前端类型检查、前端构建和 Playwright 桌面/移动检查。当前剩余边界：本 Story 不实现学生端申诉页面、不预览附件文件、不渲染图片级原卷；页面只展示后端 appeal evidence 已返回的真实快照。上述边界均已记录，不使用 mock/stub 冒充真实能力。
