# STORY-MATH-06：Rubric 数学证据

状态：Implemented。

- 扩展现有 `paper.RubricPoint`，增加版本化 `evidence_requirements`，没有另建第二套 Rubric。
- 支持 all_of、any_of、at_least、合法变换、最终结果、概念、单位和定义域。
- Matcher 只生成 supported/unsupported/uncertain 证据及来源，不计算、不覆盖、不发布得分。
- 非法深度、空组合和未知证据类型在 Rubric 保存时被拒绝。

边界：未配置数学证据要求的评分点继续使用原人工评分流程。
