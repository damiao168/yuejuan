# STORY-070B：确定性组卷与教师确认

状态：Planned。优先级：P0。父 Story：[070](STORY-070-assessment-blueprint-smart-assembly.md)。依赖：070A、069B/C/F。

## 问题与范围

初版用 Go Filter → Greedy → Local Search → Constraint Repair 生成可解释候选，不让 LLM 决定选题。允许可验证的小池穷举作为测试 oracle，不把启发式结果宣称全局最优。

- 固定输入：blueprint_version/hash、可见且 active/published 的 item_version 清单、完整 bundle hash、metadata/stat snapshot、policy、seed、solver_version 和操作次数预算。候选池建于一致数据库读视图并持久化摘要/必要输入。
- 稳定按 Item/Version 排序，seed 驱动可复现伪随机过程；不遍历无序 map 决定选题。墙钟 deadline 只作中止保护，interrupted 不算正常确定性结果。
- 同卷只选一个 logical Item，必选/禁用与 family 约束先执行；题目分值固定，不隐式缩放 Rubric。
- 独立 hard constraint evaluator 在候选保存与确认前复算全部约束；soft 分别报告偏差、权重与优化值，缺质量指标有可见处理策略。
- constraint result 保存期望、实测、pass/violation、诊断与来源；候选支持预览、固定题号/顺序与教师确认。编辑题目选择创建新 candidate revision 并重新校验，不原地伪装原 solver 输出。

## 状态与失败语义

run 为 queued/running/completed/failed/interrupted；结果分类：feasible、capacity_insufficient、search_exhausted、invalid_input。capacity_insufficient 给出必要条件证据（例如符合条件的题目仅 4 道而要求 5）；一般复杂冲突未证明则不能写 mathematically_infeasible。

诊断展示过滤前后授权池数量、约束缺口与建议。建议不自动放宽约束；教师编辑后生成新 BlueprintVersion/Run。记录真实 elapsed/search operations，后续比较 solver 时不挑选性隐藏失败。

## 数据与 API 规划

新增 test_assembly_run、candidate、candidate_item、constraint_result；候选 item 固定 source Version/hash/score/sort 与 run 输入。拟定 create run、read result、candidate preview、revise、confirm API。

确认固定 candidate revision、target exam revision 与 command ID，复用 069B materialize；复验当前权限、Item 生命周期/用途、bundle hash 与目标 draft 状态。来源或目标变更返回 stale_candidate/target_revision_conflict，先重新预览，不静默挑替代题。070C 后加入曝光预留校验。

## 非范围

不做外部求解器、平行卷、IRT 目标或 LLM 最终选题，不保证任意复杂蓝图必有解，不把搜索耗尽当数学无解，不直接把候选写为 ready。

## 预计修改文件

扩展 `internal/testassembly/` pool snapshot/solver/evaluator/run store 与迁移；修改 questionbank materialize、服务命令恢复与路由、管理端方案/候选/约束报告、OpenAPI/SDK。

## 测试方式

Go 用小池穷举 oracle 验证 hard evaluator 与已知可行案例，固定池/seed 重放至少两次；针对容量不足/搜索耗尽/中止分别验证。真实 PostgreSQL 验证 run 输入冻结、候选版本、确认复验、事务回滚与命令恢复。UI 只串通方案→候选→约束→确认；继承 paper/assessment ready 定向回归。

## 验收标准

1. 相同固定输入生成相同候选 ID 清单/顺序与硬约束结果；无序 map 或执行速度不改变正常结果。
2. 总分/题量/知识点/难度/必选禁用逐条可核算，违规候选不能确认，未知指标不作为零优化。
3. capacity_insufficient 有池容量证据，search_exhausted 明确“未找到”，超时标 interrupted；未获证明不声称无解或最优。
4. 确认重试返回相同考试题目，半套复制失败回滚；撤权、退役、目标 ready/revision 变化都拒绝旧候选。
5. 候选编辑产生 revision 并重验，原 run 可重放；教师不接受建议时蓝图不被自动改变。
6. 指定版本 materialize 后继续既有 ready 门禁，阅卷不动态读题库；全部命令有持久化回执和来源记录。

## 规划审阅与实施记录

规划已拆开生成、独立验证与确认事务，避免贪心算法给出虚假可行性承诺。实现、实现审阅、修正与 Approval 待证据，验收后进入 070C。
