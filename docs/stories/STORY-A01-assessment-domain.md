# STORY-A01：多学科 Assessment Domain Foundation

状态：Implemented（领域、冻结快照、评分链路、配置 UI 与契约已完成定向验证）

## 目标

在现有 `exam`、`paper`、`grading` 和 `review` 事实模型之上，建立版本化的学科配置、通用题型原型、评分证据类型、风险策略和考试题目快照。后续评分、人工复核与 AI 准入应读取冻结快照，不再根据页面文案或通用 Prompt 猜测学科能力。

## 当前交付范围

- `SubjectProfile`：按租户、学段、学科和版本表达 parser 要求、证据要求与默认评分策略。
- `QuestionArchetype`：首批统一八类题型，不绑定地区、年份或业务题号。
- 题目 Assessment Profile：考试进入 ready 前，为题目选择 Profile、题型、允许证据、评分策略和风险等级。
- `ExamQuestionAssessmentSnapshot`：考试 ready 后冻结 Profile 版本、题型、Rubric 和评分策略，供后续链路追溯。
- `ScoringEvidence`：保存结构化证据和原始答卷引用；不复制、覆盖或伪造原始答卷。
- OpenAPI：定义查询 Profile、查询题型、读取/保存题目配置、读取冻结快照五个操作。

## 统一枚举

### 学段

- `junior`
- `senior`

### 学科

- `chinese`
- `mathematics`
- `english`
- `physics`
- `chemistry`
- `biology`
- `history`
- `geography`
- `ethics_politics`

这组代码是 A 系列领域模型的规范值。仓库旧考试数据仍可能使用 `math` 和 `politics`；兼容映射必须集中在领域边界，不能让新代码继续产生两套含义相同的值。

### Question Archetype

- `selected_response`
- `exact_text`
- `numeric_expression`
- `structured_steps`
- `short_constructed`
- `extended_response`
- `diagram_graph`
- `table_experiment`

### Scoring Mode

- `RULE_AUTO`
- `AI_ASSIST`
- `AI_FAST_CONFIRM`
- `HUMAN_PRIMARY`
- `DUAL_HUMAN`
- `MANUAL_ONLY`

### Exam Risk Tier

- `R1`：形成性、低风险考试。
- `R2`：校内总结性考试。
- `R3`：高责任评价。

## API 契约

### 查询 Subject Profile

`GET /api/v1/assessment/subject-profiles?stage=&subject=`

返回当前租户可用的版本化 Profile。`parser_policy` 表达所需 parser 能力与质量条件，不代表这些 parser 已经真实部署。

### 查询 Question Archetype

`GET /api/v1/assessment/question-archetypes`

返回统一题型、响应结构、允许证据和默认评分模式，不返回地区题号映射。

### 配置考试题目

`GET /api/v1/exams/{examId}/questions/{questionId}/assessment-profile`

返回考试 ready 前的当前配置及 `revision`，用于刷新后继续编辑并安全提交下一次修改。未配置时返回 404。

`PUT /api/v1/exams/{examId}/questions/{questionId}/assessment-profile`

请求包含：

- `subject_profile_id`
- `archetype_code`
- `allowed_evidence_types`
- `risk_tier`
- `scoring_policy.mode`
- `scoring_policy.confidence_threshold`（可选）
- `scoring_policy.require_evidence`
- `scoring_policy.human_review_below_confidence`（布尔开关）
- `expected_revision`

`expected_revision` 用于并发更新保护。考试进入 ready 后不得原地修改。`R3 + extended_response + AI_FAST_CONFIRM` 在服务端与 OpenAPI 请求 Schema 中均被禁止。

### 读取冻结快照

`GET /api/v1/exams/{examId}/questions/{questionId}/assessment-snapshot`

返回冻结的 Profile ID/代码/版本、学段、学科、题型、允许证据、风险等级、Profile 快照、题型快照、Rubric 快照、评分策略快照和内容哈希。后续 Regrade 如需新规则，应引用新版本，不应覆盖原快照。

## 学科证据边界

- 语文、英语写作：支持 multi-trait Rubric 与文本证据。
- 数学、物理：支持 `math_expression`、`math_step`、`unit_value`。
- 化学：支持 `chemical_equation`。
- 历史、地理、道德与法治/思想政治、生物：支持 `concept`、`relation`、`text_span`。
- 地理图表与实验题：支持 `diagram_feature`、`table_cell`。

上述是领域契约和后续能力输入，不是 parser 运行成功声明。

## 强制策略

- R3 开放题默认使用 `HUMAN_PRIMARY` 或 `DUAL_HUMAN`。
- R3 的 `extended_response` 禁止使用 `AI_FAST_CONFIRM`。
- Profile、Rubric 和评分策略在考试 ready 后冻结。
- 原始答卷始终是证据来源；派生文本、公式、表格和图形结构不能覆盖原图。
- 后续 AI 准入还必须结合 OCR/parser 质量、评测证据和校准状态；A01 只建立领域基础，不直接判定 AI 可用。

## 明确未实现

- 不在 A01 实现真实数学公式 OCR、化学表达式 parser、图表/表格 parser 或新的中文手写识别模型。
- 不实现答案聚类、新评分模型、模型微调或新的通用 Prompt。
- 不推翻现有 `human_grade`、`scoring_run` 和最终成绩事实。
- Profile 中声明某种 parser 要求，不等于运行环境已安装、接通或验证该 parser。

## 验收口径

- OpenAPI 能解析，五个新增 operationId 唯一且请求/响应 Schema 可机器检查。
- `junior/senior` 与九学科只使用一组 A 系列规范枚举。
- 八种 Question Archetype 和六种 Scoring Mode 均有契约定义。
- OpenAPI 明确禁止 `R3 + extended_response + AI_FAST_CONFIRM`。
- 后端验收还需覆盖版本唯一性、跨租户隔离、ready 后不可修改、快照与原题后续修改解耦。
- “进入 grading 的题 100% 有快照”必须由真实数据库 E2E 证明，不能由文档或前端状态代替。

## 当前边界与后续依赖

A01 完成后，A02-A05 才能稳定消费学科与题型事实；A14-A17 再基于快照、parser 质量、模型评测和校准证据决定 AI Eligibility。任何只配置 Profile、但未完成真实 parser/OCR 或模型评测的环境，都必须继续转人工或 abstain。
