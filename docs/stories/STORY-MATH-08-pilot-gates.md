# STORY-MATH-08：分层试点门禁

状态：Implemented，未达到真实试点放行条件。

- Formula、AST、Spatial、Solution Graph、CAS Equivalence、Rubric Evidence、Unsafe Suggestion、Risk Recall 分层判断，禁止用最终总分掩盖弱层。
- 评测事实与阈值策略追加写入 PostgreSQL，按租户和科目隔离并审计。
- 即使全部通过，scope 也恒为 `teacher_suggestion_only`，不授予自动最终评分或发布权限。
- MathBench 支持等价精度、不安全建议率和风险召回；文科样本会被公式基准拒绝。

边界：仓库内 synthetic fixture 仅验证可复现，不能用于放行。数学、物理、化学分别需要真实脱敏数据和人工标注门禁证据。
