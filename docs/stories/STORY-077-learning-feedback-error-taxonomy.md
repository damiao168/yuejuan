# STORY-077：学习反馈与错因体系

状态：Planned。优先级：P2。依赖：069、075、已发布题目与冻结评分事实、模型治理。

## 问题与范围

学习反馈应解释可证据支持的错误与复习方向。以 published release、冻结题目/Rubric、人工确认答案证据和长期表现作为上下文，生成候选，由教师确认后向学生展示。

- 首切片先做版本化 error taxonomy 与人工标签：concept_gap、calculation_error、misread_question、missing_step、unit_error、formula_error、reasoning_gap、incomplete_answer、expression_problem，并保留 unknown 与多错因。
- 后续 AI 建议记录 provider/model/prompt/policy 版本、输入事实 hash、引用的题目/评分点/证据 ID 和候选修订。无授权原图或证据不完整时给出不足，不能推断学生心理原因。
- 失分归因只映射到冻结 Rubric 的明确扣分/评分点证据；多标签不得重复加总失分。已确认反馈记录 teacher、revision、release_id，发布修订使相关反馈进入复查流程。
- 推荐复习题仅来自允许 practice 用途的已发布版本，检查版权、ACL、曝光与公开范围；不把保密题干/答案泄露给学生。
- 学生看见证据对应的错误、建议、知识点和允许的练习入口；未确认 AI 建议不影响成绩或正式报告事实。

## 非范围

不将学生答案直接拼入无治理 LLM，不自动给正式错因结论或改分，不生成无法引用证据的失分分解，不从未校准稀疏考试预测真实掌握概率。

## 预计修改文件

拟新增 `internal/learningfeedback/`、taxonomy/候选/确认迁移、管理端反馈复核和学生端反馈页；修改受治理模型请求、questionbank practice eligibility、OpenAPI/SDK。沿用既有教师/AI feedback 的入口，实施前核对避免重复实体。

## 测试方式与验收标准

Go/真实 PostgreSQL 验证 taxonomy、证据权限、失分防重、候选到教师确认、重发布复查及模型失败；UI 验证未确认不可见。模拟模型只证明契约；真实反馈质量需要脱敏授权样本与教师评审。

验收要求每个正式错因可追溯到 release 和冻结证据、无证据显示 unknown、多标签失分总和不超实际失分；模型故障保留人工流程；保密题不推荐；AI 不写最终分。首切片人工错因先验收，再实施 AI 建议。

## 规划审阅与实施记录

规划已将事实、候选与教师确认分开，并区分质量未知与自动结论。当前实现、实现审阅、修正与审批待留证。
