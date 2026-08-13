# Assessment Domain

本领域把“学科、题型、证据和评分策略”从页面文案与通用 Prompt 中抽离，作为版本化、可审计的考试配置。它建立在现有 `exam`、`paper`、`grading`、`subjective` 与 `review` 事实之上，不替换既有评分记录。

## 领域边界

- `subject_profile`：租户内、按学段与学科版本化的解析、证据和默认评分策略。
- `question_archetype`：跨学科通用题型原型及其响应结构、证据类型和默认评分模式。
- `question_assessment_config`：考试草稿期内题目选用的 Profile、Archetype、证据范围、风险级别和评分策略。
- `exam_question_snapshot`：考试确认就绪时冻结的题目、Rubric、Profile 和评分策略快照。
- `scoring_evidence`：指向原答卷或派生 Artifact 的结构化证据；不复制或覆盖原图。

## 生命周期与不变量

```text
题目与 Rubric 草稿
  -> 配置 Assessment Profile
  -> 确认考试就绪
  -> 冻结 ExamQuestionSnapshot
  -> 答卷采集 / 评分 / 复核
```

以下规则由服务端和数据库共同保证：

1. 每个进入评分流程的题目必须存在对应的 `ExamQuestionSnapshot`。
2. 快照创建后不可原地修改或删除；后续改动只能用于新考试或显式 Regrade 版本。
3. `R3 + extended_response` 禁止 `AI_FAST_CONFIRM`，必须进入人工主评或双评。
4. 配置中的证据类型必须属于所选 Profile 和 Archetype 的允许集合。
5. 所有租户数据通过复合外键和租户索引隔离；跨租户引用由数据库拒绝。

## 消费约定

- 客观题规则评分读取快照中的题型与评分策略版本，并将快照 ID 写入评分事实。
- 主观题 AI 请求读取快照中的学段、学科、Archetype、证据范围、风险级别和 Rubric；不得从页面文字推断。
- 人工复核工作台展示同一快照，使教师看到的 Rubric 与评分时使用的版本一致。
- 模型输出只是候选分和证据，最终分数仍沿用现有 `question_grade`、`human_grade` 与仲裁事实链。

## 多学科基础

首批支持初中、高中两个学段和九个学科：语文、数学、英语、物理、化学、生物、历史、地理、道德与法治/思想政治。每个学科至少提供两个可用 Archetype；Profile 是配置模板，不代表真实公式 OCR、化学 Parser 或新评分模型已经实现。

## API

- `GET /api/v1/assessment/subject-profiles`
- `GET /api/v1/assessment/question-archetypes`
- `GET|PUT /api/v1/exams/{examId}/questions/{questionId}/assessment-profile`
- `GET /api/v1/exams/{examId}/questions/{questionId}/assessment-snapshot`

API 契约以 `services/api-gateway/openapi/edugrade-api.openapi.json` 为准；前端不得维护第三套枚举定义。A03 完成后，Web 与桌面端统一使用生成 SDK。
