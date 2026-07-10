# STORY-027 Web 阅卷工作台

## Plan

### 目标

实现 EduGrade Enterprise 的 Web 阅卷工作台，面向阅卷员高效处理真实 `review_task`，在一个专业、高密度界面中查看原始答卷页、answer segment、OCR 文本、AI grade、Rubric 和证据校验信息，并通过真实后端接口提交 `human_grade`。

### 视觉与交互模型

- visual thesis：克制、低噪声、长时间可用的三栏阅卷控制台，强调原图、证据和最终分的稳定对照。
- content plan：顶部任务队列与考试筛选；左侧原图/答题区域查看器；中间 OCR、AI 证据和风险；右侧 Rubric 与人工评分；底部同类答案、历史样例和快捷键提示。
- interaction thesis：缩放/旋转/拖拽只影响答卷查看器；快捷键聚焦评分动作；提交后自动切换到下一条可处理任务。

### 范围

- 新增 Web 阅卷工作台页面并接入 `/grading` 路由。
- 新增前端 review/grading API 封装：
  - `GET /api/v1/review-tasks`
  - `GET /api/v1/review-tasks/{id}`
  - `POST /api/v1/review-tasks/{id}/submit`
  - `POST /api/v1/review-tasks/{id}/return`
  - `PUT /api/v1/answer-segments/{id}/answer`
  - `GET /api/v1/answer-segments/{id}/ai-grades`
  - `POST /api/v1/answer-segments/{id}/rule-grade`
  - `POST /api/v1/answer-segments/{id}/subjective-ai-grade`
  - `POST /api/v1/ai-grades/{id}/verify-evidence`
- 使用现有真实 API 串联工作台上下文：
  - `review_task` 提供任务元数据、anonymous_code、question_id、answer_segment_id、submission_id。
  - `listAnswerSegments(submission_id)` 查找当前 answer segment。
  - `listSubmissionPages(submission_id)` + 带鉴权 `downloadFileBlob` 展示原始答卷页面。
  - `listOcrTasks(submission_id)` / `getOcrTask` 展示 OCR 文本。
  - `listQuestions(exam_id)` 查找题目与 Rubric。
  - `listAiGrades(answer_segment_id)` 展示 AI 建议分、置信度、采分点、风险和 mock 标记。
- 支持人工评分面板：
  - 题目满分。
  - 最终分输入。
  - Rubric 点选与分值合计。
  - 常用评语。
  - 学生可见反馈。
  - 教师私密备注。
  - 采纳 AI 分。
  - 标记争议/退回重评。
  - 提交并自动进入下一份。
- 支持快捷键：
  - `1-9` 快速给分。
  - `A` 采纳 AI 分。
  - `R` 标记争议。
  - `Ctrl+Enter` 提交。
- 支持无 token、无权限、无任务、上下文缺失、AI mock 等真实状态提示。

### 非范围

- 不实现真实主观题模型推理。现有 `subjective-ai-grade` 默认 adapter 是 mock，页面必须明确显示 `mock=true`，不能把它当真实 AI 能力。
- 不实现同类答案检索、历史样例库真实服务；底部区域只展示“待后续 API 支持”的空状态。
- 不实现图片裁剪、OCR 可编辑校正、复杂 OCR 文字空间叠加；只根据现有 OCR result bbox 做基础叠加提示。
- 不实现 028 双评仲裁页面。
- 不实现成绩发布、申诉或报告能力。
- 不新增后端“聚合上下文”接口，除非实现过程中发现当前真实接口无法满足基本工作台。

### 修改文件

预计新增：

- `apps/web-admin/src/api/review.ts`
- `apps/web-admin/src/pages/GradingWorkbenchPage.tsx`
- `docs/stories/STORY-027-approval.md`

预计修改：

- `apps/web-admin/src/App.tsx`
- `apps/web-admin/src/auth/session.ts`
- `apps/web-admin/src/router/routes.tsx`
- `apps/web-admin/src/styles.css`
- `apps/web-admin/README.md`
- `README.md`
- `docs/stories/README.md`

### 验收标准

- `/grading` 不再是 mock 占位页，路由标记为真实 API。
- 缺少 `edugrade.access_token` 时页面显示明确错误/禁用提交，不回退 mock 数据。
- 能读取真实 `review_task` 列表并选择任务。
- 能基于真实接口展示 answer segment、题目、Rubric、OCR 任务结果、AI grade。
- 能通过鉴权 blob 展示答卷页面；图片支持缩放、旋转、拖拽，PDF/不可预览文件显示明确说明。
- AI grade 的 suggested score、confidence、matched/missing points、evidence、risk flags、needs human review、mock 状态清晰展示。
- 能执行规则判分、主观题 AI 接口调用和证据校验；mock AI 必须明确标记。
- 人工评分提交调用真实 `POST /review-tasks/{id}/submit`，分数不得超过题目满分。
- Rubric selections 使用真实 Rubric points，提交 payload 与后端契约一致。
- 提交成功后自动进入下一条未提交任务。
- 无权限不可提交。
- 快捷键 `1-9`、`A`、`R`、`Ctrl+Enter` 可用，且不干扰输入框编辑。
- 前端 typecheck/build 通过。
- 后端 `go test ./...` 和 build 通过，确认本 Story 未破坏现有 API。
- Playwright 桌面和移动视口检查 `/grading` 无明显重叠、无整页横向滚动、无控制台错误。

## Plan Review

### 是否越界

未越界。计划只实现 Web 阅卷工作台本身和调用现有真实阅卷/评分接口，不进入 Story 028 双评仲裁、Story 029 成绩发布、Story 030 报告等后续页面。

### 是否遗漏显式需求

已覆盖原始提示中的三栏布局、底部区域、获取 review_task、展示 answer_segment/OCR/AI grade/Rubric、提交 human_grade、快捷键、分数校验、自动下一份、无权限禁用、mock 标记等要求。

### 是否符合当前仓库实际

符合。后端已有 review task、human grade、answer segment、OCR task、file download、question/Rubric、AI grade 和 evidence check API。当前不足是没有单个 answer segment 查询与聚合上下文接口，计划通过 `submission_id` 列表接口和 `question_id` 过滤来拼装，不虚构后端能力。

## Implementation

- 新增 `apps/web-admin/src/api/review.ts`，封装真实 review task、human grade、answer record、rule grade、subjective AI grade、AI grade list 和 evidence check API。
- 新增 `apps/web-admin/src/pages/GradingWorkbenchPage.tsx`，实现阅卷工作台。
- `/grading` 路由从 mock 占位改为真实 API 页面，并要求 `review:manage`、`grading:manage`、`segment:manage`、`ocr:manage`、`file:manage`、`exam:manage`、`submission:manage`、`evidence:manage`。
- 工作台通过现有真实接口拼装上下文：
  - `review_task` 列表与详情。
  - `answer_segment` 列表中过滤当前 segment。
  - `submission_page` + 鉴权 blob 下载展示原始页面。
  - `ocr_task` 详情读取 OCR results。
  - `question` + Rubric 读取评分标准。
  - `ai_grade` 列表展示 AI/规则建议。
- 左侧原始答卷查看器支持原图/OCR 切换、缩放、旋转、拖拽和基础 bbox 高亮。
- 中间证据面板展示 AI 建议分、置信度、采分点、证据、风险、mock 标记和证据校验结果。
- 右侧人工评分面板支持最终分、Rubric selections、常用评语、学生反馈、私密备注、采纳 AI、标记争议、提交。
- 支持 `1-9` 快速给分、`A` 采纳 AI、`R` 标记争议、`Ctrl+Enter` 提交，并避开输入框。
- 底部同类答案和历史样例明确显示“待后续 API 支持”，不伪造数据。
- 更新 Web README 和根 README。

## Implementation Review

- `review_task` 获取：`listReviewTasks` 和 `getReviewTask` 已接入。
- answer_segment/OCR/AI grade/Rubric 展示：通过真实 segment、OCR task、question、ai grade API 串联，缺失上下文显示 warning。
- human_grade 提交：`submitHumanGrade` 使用真实 `POST /review-tasks/{id}/submit`，前端校验 0 到满分。
- 快捷键：已实现 `1-9`、`A`、`R`、`Ctrl+Enter`，输入控件聚焦时不触发。
- 提交后自动下一份：提交成功后刷新任务列表并选择下一条未提交任务。
- 无权限/无 token：按钮禁用，缺 token 时显示明确错误，不回退 mock 数据。
- mock 标记：主观题 AI 接口产生的 `mock=true` 结果以 `MOCK AI` 标签显示。
- 同类答案/历史样例：没有真实 API，不显示假内容。
- Playwright 桌面/移动检查通过：无整页横向滚动，控制台 0 errors、0 warnings。

## Fixes

- 未新增后端代码，因此无需后端修复。
- 当前工作台使用跨模块真实 API 拼装上下文；没有引入伪聚合接口或 mock 任务数据。
- Vite 构建仍有既有大 chunk warning，记录为剩余风险，不在本 Story 做路由拆包。

## Approval

Approved。

本 Story 已按计划完成，并通过后端测试、后端构建、前端类型检查、前端构建和浏览器检查。所有未实现能力均标注为后续 API 或后续 Story，不用 mock/stub 冒充真实能力。
