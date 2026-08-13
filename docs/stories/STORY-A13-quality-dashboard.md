# STORY-A13 评分质量总览

状态：Implemented（A11/A12 接入后自动补充漂移事件和回标摘要）

## 已实现

- `GET /api/v1/exams/{examId}/quality-dashboard` 返回考试级质量投影：按题目、题型、风险等级、标准卷分档和阅卷员聚合。
- 所有比例均带 `numerator`、`denominator` 与 `sample_size`；分母为零时不伪装成 0%。
- 质量投影只含汇总事实，不返回学生身份、答卷内容、标准卷参考分、标定答案或阅卷私密备注。
- R3 题目存在未解决 critical 质量事件时，质量门禁直接为 `blocked`。
- 每条 warning/blocking 都包含原因、责任对象与处理路径；管理员可从考试工作区的“质量”页展开逐题查看。
- 质量页提供紧凑入口：配置 A10 盲测策略、查看 A12 回标批次；两者均不会直接修改学生最终成绩。

## 接线边界

- A11 以 `DriftReader` 接入，A12 以 `BackmarkReader` 接入，避免质量投影绑定其内部存储或迁移序号。
- 没有接入的质量来源以 `insufficient_data`/空汇总呈现，不会被默认为“通过”。
