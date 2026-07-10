# STORY-030 Web 学情报告页面

## Plan

### 目标

实现 EduGrade Enterprise Web 管理后台的学情报告页面，让教务管理者在 `/reports` 通过真实后端报告 API 查看考试概览、分数分布、班级对比、题目分析、知识点掌握、高频错误、阅卷质量和题目质量，并导出带水印的真实 CSV 报告。

### 视觉与交互模型

- visual thesis：克制、密集、可审计的报告工作台，以数字、图表和空状态表达真实数据边界。
- content plan：考试选择与刷新；核心指标；分数分布与班级对比；题目得分率与题目质量；知识点与高频错误；阅卷质量；导出状态与水印提示。
- interaction thesis：考试切换后并行读取报告；图表 tooltip 只展示真实字段；导出前二次确认，导出后展示真实 watermark 和文件名。

### 范围

- 新增 `/reports` 真实 API 页面，替换当前 mock 占位。
- 新增前端 reports API 封装：
  - `GET /api/v1/exams/{examId}/reports/overview`
  - `GET /api/v1/exams/{examId}/reports/classes`
  - `GET /api/v1/exams/{examId}/reports/questions`
  - `GET /api/v1/exams/{examId}/reports/grading-quality`
  - `POST /api/v1/exams/{examId}/reports/export`
- 使用项目已有 Recharts 绘制：
  - 分数分布图。
  - 班级对比图。
  - 题目得分率图。
  - 知识点掌握率图。
  - 阅卷质量指标图或紧凑指标条。
  - 题目难度、区分度、客观题选项分布。
- 通过真实 API 展示考试概览：
  - 平均分、中位数、最高分、最低分、标准差、及格率、优秀率。
- 通过真实 API 展示：
  - 班级对比。
  - 题目得分率。
  - 知识点掌握率。
  - 高频错误。
  - AI 采纳率、人工改分率、双评分差、仲裁数量、OCR 失败率。
  - 题目难度、区分度、客观题选项分布。
- 没有 `edugrade.access_token` 时显示明确错误，不读取报告、不导出、不回退 mock 数据。
- 后端返回 `empty` 或指标 `available=false` 时显示空状态或不可用状态，不填假数字。
- 导出报告前提示审计与水印，导出调用真实 CSV endpoint，并显示真实 `X-EduGrade-Watermark`。
- 更新文档和 Story 审批记录。

### 非范围

- 不实现 PDF、XLSX 或图像化报告导出；当前后端真实能力为 CSV。
- 不生成新的 AI 教学建议、推荐练习或自然语言总结；只展示后端已有真实字段。
- 不新增报告后端统计能力，除非实现审阅证明现有 API 无法满足显式 Web 页面需求。
- 不实现学生端个人报告页面。
- 不实现完整审计日志中心。
- 不把无数据状态伪装成示例图表或示例统计。

### 修改文件

预计新增：
- `apps/web-admin/src/api/reports.ts`
- `apps/web-admin/src/pages/LearningReportsPage.tsx`
- `docs/stories/STORY-030-approval.md`

预计修改：
- `apps/web-admin/src/App.tsx`
- `apps/web-admin/src/router/routes.tsx`
- `apps/web-admin/src/styles.css`
- `apps/web-admin/README.md`
- `README.md`
- `docs/stories/README.md`
- `docs/stories/STORY-030-web-learning-reports.md`

### 验收标准

- `/reports` 不再是 mock 占位页，路由标记为真实 API。
- 缺少 `edugrade.access_token` 时页面显示明确错误，所有报告图表为空/禁用，导出禁用，不回退 mock 数据。
- 页面可选择真实考试，并读取 overview、classes、questions、grading-quality。
- 考试概览展示平均分、中位数、最高分、最低分、标准差、及格率和优秀率。
- 分数分布图使用 overview distribution。
- 班级对比使用 classes 或 overview class_comparisons。
- 题目得分率使用 questions score_rate。
- 知识点掌握率来自 class reports 的 weak knowledge points 或 student report 不可用时明确记录数据来源边界。
- 高频错误来自 questions frequent_errors 与 class frequent wrong questions。
- 阅卷质量展示 AI 采纳率、人工改分率、双评分差、仲裁数量、OCR 失败率。
- 题目质量展示难度、区分度、客观题选项分布；没有真实选项分布时显示空状态。
- 导出报告前提示审计与水印，导出调用真实 report export API，并展示真实 watermark。
- 前端 typecheck/build 通过。
- 后端 `go test ./...` 和 build 通过，确认未破坏已有报告 API。
- Playwright 桌面和移动视口检查 `/reports` 无明显重叠、无整页横向滚动、无控制台错误。

## Plan Review

### 是否越界

未越界。计划只实现 Web 学情报告页面和前端 API 封装，复用 STORY-022 已完成的真实报告后端 API；不进入 PDF/XLSX、学生端报告、AI 新建议生成、完整审计中心或申诉页面。

### 是否遗漏显式需求

显式需求已覆盖：考试概览报告、分数分布图、班级对比、题目得分率、知识点掌握率、高频错误、阅卷质量分析、题目质量分析、导出报告、使用已有图表库、不用假数据、无数据空状态、真实报告 API 对接。

### 是否符合当前仓库实际

符合。仓库已有 `report` 后端模块，提供 overview、classes、questions、grading-quality 和 export CSV；前端已有 Recharts 依赖、考试 API、统一 requestBlob 与真实 token 门禁模式。当前后端不提供 PDF/XLSX、AI 教学建议生成或独立知识点总览 endpoint，因此本 Story 将使用现有真实 class/question 数据聚合展示，并在无法得到选项分布等字段时显示空状态。

## Implementation

- 新增 `apps/web-admin/src/api/reports.ts`：
  - 定义 overview、class report、question analysis、grading quality、metric、empty state、export 类型。
  - 封装 `GET /reports/overview`、`GET /reports/classes`、`GET /reports/questions`、`GET /reports/grading-quality` 和 `POST /reports/export`。
  - 导出复用 `requestBlob`，读取真实 `X-EduGrade-Watermark`。
- 新增 `apps/web-admin/src/pages/LearningReportsPage.tsx`：
  - 读取真实考试列表，选择考试后并行读取 overview、classes、questions、grading-quality。
  - 缺少 `edugrade.access_token` 时显示明确错误，不读取报告、不导出、不回退 mock 数据。
  - 使用 Recharts 绘制分数分布、班级对比、题目得分率、知识点掌握率、阅卷质量比例和题目质量趋势。
  - 概览指标展示平均分、中位数、最高分、最低分、标准差、及格率、优秀率。
  - 高频错误来自 question frequent_errors；没有题目错误线索时使用 class frequent_wrong_questions。
  - 知识点掌握率来自 class reports 的 weak_knowledge_points 真实聚合。
  - 客观题选项分布只展示真实 option_distribution；没有真实选项 payload 时显示空状态。
  - 导出 CSV 前二次确认审计和水印，导出后展示真实 watermark。
- `/reports` 路由从 mock 改为真实 API 页面，要求 `report:read`；导出按钮按 `report:export` 与真实 token 控制。
- 新增报告页面响应式样式，桌面两列、移动单列，表格使用内部横向滚动而不造成整页溢出。
- 更新 `README.md`、`apps/web-admin/README.md` 和 Story 索引状态。

## Implementation Review

- `/reports` 真实 API 页面：已通过 `routes.tsx mock=false` 和 `LearningReportsPage` 真实 API 调用确认。
- 缺 token 行为：Playwright 在登录壳下清除 `edugrade.access_token` 后进入 `/reports`，页面显示明确错误，导出禁用，图表和表格为空态，无 mock 报告数据。
- 考试概览：平均分、中位数、最高分、最低分、标准差、及格率、优秀率来自 overview stats。
- 分数分布：使用 overview distribution。
- 班级对比：优先使用 overview class_comparisons，必要时使用 class reports 的真实 stats。
- 题目得分率：使用 questions score_rate 和 correct_rate。
- 知识点掌握率：使用 class reports weak_knowledge_points 真实聚合，文档记录当前无独立知识点总览 endpoint。
- 高频错误：使用 questions frequent_errors 或 class frequent_wrong_questions，不造文本。
- 阅卷质量：展示 AI 采纳率、人工改分率、平均双评分差、仲裁数量、OCR 失败率；`available=false` 时显示不可用。
- 题目质量：展示 difficulty、discrimination 和真实 option_distribution；无选项分布时显示空状态。
- 导出报告：调用真实 report export API，导出前二次确认，导出后展示 watermark。
- 前端 typecheck/build、后端 `go test ./...` 和后端 build 均通过。
- Playwright 桌面/移动检查通过：控制台 0 errors、0 warnings，整页无横向溢出。

## Fixes

- 实现审阅发现明细表在空数据时会落到默认表格空态，已补统一中文空状态，满足“没有数据时显示空状态”。

## Approval

Approved。

本 Story 已按计划完成，并通过后端测试、后端构建、前端类型检查、前端构建和 Playwright 桌面/移动检查。当前剩余边界：导出格式为现有后端真实 CSV，不是 PDF/XLSX；页面不生成新的 AI 教学建议；知识点掌握率基于现有 class report 聚合；客观题选项分布只在后端返回真实 payload 时展示。上述边界均已记录，不使用 mock/stub 冒充真实能力。
