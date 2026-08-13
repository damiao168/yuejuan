# STORY-A07 相似答案分组与 Human-amplification Grading

状态：Implemented

## 已实现

- 仅对可可靠文本化的 `exact_text` 与 `short_constructed` 作答构建保守分组；算法与表征版本、冻结题目快照和成员相似度均可追溯。
- 教师在独立分组抽屉查看代表、边界和异常样本，并完成接受/拒绝抽检；异常样本未检查或最低抽检量不足时，前后端都拒绝确认。
- 组决策采用乐观修订；确认只生成逐成员 automation candidate，不直接写最终成绩；支持按 rollback reference 带原因回滚。
- 质量页展示 Group Homogeneity、Batch Override Rate、Human Actions Saved 与事后审计证据状态。
- Teacher Reference Cases 仅来自活动且人工批准的 Gold version，不把原始学生答案相似检索伪装成教师参考案例。
- OpenAPI 和生成 SDK 覆盖构建、列表、详情、抽检、候选保存、确认、回滚与指标查询。

## 边界

- 数学表达式在结构化 representation 稳定前不参与；作文和长论述默认不开放分组。
- 分组的目标是减少重复人工动作，不是自动定分。任何组候选仍受后续评分与质量门禁约束。
