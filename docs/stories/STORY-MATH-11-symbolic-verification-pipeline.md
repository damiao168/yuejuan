# STORY-MATH-11：两阶段数学符号验证

## 状态

Implemented Baseline / Extended Mathematics and Real-answer Benchmark Pending

## 目标

在既有 Worker Runtime 中串接 `recognition → symbolic verification`。API handler 不调用 SymPy；识别证据、教师修订和验证派生证据均可按固定版本重放，且不拥有最终评分权。

## 实现

- `000146` 增加 `stage / parent_artifact_id / correction_revision / quality_summary_json`，约束验证派生身份，扩展原有不可变触发器。
- recognition artifact 和有公式的教师 correction 插入时，在同一数据库事务创建 `math-verification` 任务；任务只保存 artifact/version/hash/revision 引用，不包含学生答案全文。
- Worker 使用原有领取、租约、心跳与重试机制，通过专用 input/complete/fail adapter 消费精确 correction revision，而不是运行时漂移到最新 revision。
- 验证 `derives` 边的公式变换；普通 `next` 阅读顺序不被擅自解释为等价关系。结果保存源/目标 step、formula、SymPy evidence 和版本。
- 服务端拒绝与 canonical `derives` 边或所属 step/formula 不一致的变换证据；被涂除或未进入当前步骤图的公式不参与第二阶段计算。
- 对末端单变量方程计算受限实数解集，保存为 `solution_set_computed` 约束事实；它不等于学生最终结论完整正确，更不直接获得 Rubric 分值。
- 验证派生工件与 runtime task 完成原子提交。无效/过期租约回滚证据激活；新 crop 或更新的 correction 使旧任务 `superseded`，不覆盖当前有效证据；不同结果的重试返回冲突。
- 第二阶段保留 syntax checks，替换先前 symbolic checks。contradicted/uncertain 强制人工复核；quality summary 保存状态计数及保守 critical confidence。
- SymPy 原始 AST 定义域检查保留约分前的分母排除点和根式实数限制，防止消去分母后错误批准等价变形。未求解的 ConditionSet、复杂域与暂不支持的显式约束返回 uncertain。
- 符号服务暂时不可用时，识别工件仍保存；第二阶段可重新 normalize，服务故障可重试。
- recognition 和 verification 内部路由、阶段元数据、typed MathVerification 已进入 OpenAPI/SDK，并移除已正式契约化路由的历史 gap 豁免。

## 验收与边界

- 定向单测覆盖 immutable derivative、租约事务回滚、exact revision input、教师修订/新 crop supersession、重试冲突、人工复核信号与 HTTP AST 输入。
- 真实 PostgreSQL 测试使用隔离数据库、production stores、实际 Worker Runtime migration 和 `000146`，不改动部署库；不把该测试等同于完整 segment→teacher-submit E2E。
- 支持范围仍是既有受限 AST 和保守单变量实数代数基线。多变量、复杂分类讨论、几何、证明、显式域 DSL、substitution/unit 专项与真实手写答卷门禁尚未完成；未支持的情形不能自动授分。
- 本 Story 不修改 subjective grading、AI eligibility 或 final_grade。下一步 MATH-12 将用这些事实构建 criterion evidence 和服务端确定性建议分。
