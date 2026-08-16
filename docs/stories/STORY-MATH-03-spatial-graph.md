# STORY-MATH-03：空间关系图

状态：Implemented。

- 从归一化 bbox、连接符和共享数学符号生成局部候选边，不使用全局“从上到下、从左到右”排序。
- 每条边分别记录 geometry、connector、math dependency 分数与证据。
- 并行计算、汇合和划除内容均有定向测试；划除内容保留证据但不进入自动步骤。

边界：当前候选打分为确定性工程基线，真实 Edge Model 仍需人工校正数据训练与评测。
