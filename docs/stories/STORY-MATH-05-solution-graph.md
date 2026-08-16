# STORY-MATH-05：解题过程图

状态：Implemented。

- 将 AnswerBlock、空间候选边和公式依赖组合为可分支、可汇合的 `SolutionGraph` DAG。
- 图契约强制无环、引用完整、版本可追溯；低置信度图明确要求人工复核。
- 定向测试覆盖左右并行计算后汇合，不退化成错误的全局线性序列。

边界：当前 Builder 是保守基线，教师 MATH-07 校正数据才是后续 Step Grouper/Edge Model 的训练来源。
