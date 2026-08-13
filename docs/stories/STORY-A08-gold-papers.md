# STORY-A08 Gold Papers / 标准卷体系

状态：Implemented

## 已实现

- 主阅可在真实答卷提交人工评分时直接提名标准卷，提名引用已落库的评分事实，而不是重新抄写一个孤立总分。
- 每个版本冻结 Assessment Snapshot、Rubric、拟定分、评分解释、Rubric 采分点证据、典型错误标签和来源评分 ID。
- 标准卷版本须审批后才能成为活动版本；已批准版本不可原地修改，只能新建版本；支持带原因退役。
- 管理抽屉展示答题图像、冻结 Rubric、采分点证据、拟定分、解释、版本历史和覆盖缺口。
- 复核与异常页面按考试展示真实 Gold coverage 缺口；高风险校准/Seed 只读取 active approved Gold Set。
- OpenAPI 与生成 SDK 覆盖提名、列表、详情、新版本、审批、退役和 coverage 查询。

## 边界

- 标准卷不包含学生姓名、准考证号或阅卷私密备注；答题图像继续通过既有受权图像接口读取。
- Gold 不是“总分样例库”。系统同时保留 Rubric point / trait / error evidence；学科 coverage 仍以实际题型和学科规则计算。
