# STORY-MATH-12：冻结 Rubric 的确定性数学评分预览

## 状态

Implemented Baseline / Semantic Mapping, Follow-through and Real-answer Benchmark Pending

## 实现

- 复用既有 `paper.RubricPoint / EvidenceRequirement` DSL，不新增数学评分标准。一个点的多个要求按 `all_of` 聚合；支持 `all_of / any_of / at_least`，成功的替代路径只引用证明该路径的证据。
- 原子 CriterionDecision 保存状态、冻结分值、可空 awarded_score、step/formula evidence IDs、独立 verification IDs、来源、原因和人工复核信号。
- `supported` 获得该点冻结分值，明确绑定的 `contradicted` 为零；`uncertain / not_applicable / legacy unsupported` 不因候选或缺失证据成为零分。未解决项不进入旧 `missing_points`。
- `valid_transformation` 要求冻结 Target 精确匹配目标 canonical LaTeX，且 SymPy 等价检查绑定目标所属 step/formula、源 step/formula 和当前 `derives` 边。普通阅读顺序、任意其他等价式不能证明这个点。
- `final_result` 基线仅支持高置信、已通过 syntax 验证的 conclusion formula 与冻结 Target 的精确 canonical LaTeX 相等。不把 `solution_set_computed` 当作最终答案正确。非相等、缺失、等价但不同写法等情况均 uncertain，不静默判错。
- 涂除/草稿/不确定块、低置信、未解析公式、syntax 不确定、源步骤不确定和同目标冲突事实均不能确定授分或扣分。概念、单位、定义域的无来源字符串标签不再是评分权威。
- 局部等价变换的源步骤有未验证/矛盾/不确定数学依赖或自我纠正边时，返回 `dependent_step_requires_review`，不自动推断 follow-through 政策。非基线实数域与显式约束也不被静默忽略。
- 识别阶段或当前工件有尚待重验的教师修订时，所有点待确认；已有冻结 deductions 暂时也要求人工判断，不忽略扣分策略产生假的确定总分。
- 模型候选只含 point ID、status 和 evidence IDs，无 score 字段；候选单独不能结算任何点。拒绝未知点、虚构证据和重复候选。生产模型接线属于 MATH-13。
- 用 1/10000 分整数计算冻结分值之和；验证重复/空 ID、状态、非有限值、负数、精度和总分。返回 verified_score、unresolved_score、score_range；待确认分不为零时 suggested_score 为 null。

## API 与版本绑定

`GET /api/v1/math-answer-segments/{segmentId}/rubric-score`

只读预览，沿用当前租户和阅卷指派权限。生产数据源只读取当前 segment 绑定的 `exam_question_snapshot.rubric_snapshot_json`，不回退到 live question_rubric。历史冻结 JSON 没有 Rubric ID/version 时，使用考试 Snapshot ID 和解析后评分契约的 SHA-256 指纹提供稳定身份。

结果绑定 artifact ID/version、当前工件本地 correction revision、已吸收的 verified correction revision、考试 Snapshot ID 和 rubric scoring hash。计算末尾再次检查当前有效版本，已漂移则返回 409。任何未来持久化的消费方仍必须在提交时再次核验绑定，不把 GET 预览当作事务保证。

所有预览固定 `scope=teacher_suggestion_only / requires_human_review=true`。已解决单点的复核标志与最终教师确认权是不同含义：即使所有点的算术建议已确定，也不能自行提交或发布最终成绩。本 Story 不写 ai_grade / teacher_grade / final_grade，不调用模型，不改变 AI eligibility 或人类成绩链。

## 验证范围与后续

- 数学单测覆盖确认分/区间/null、明确矛盾、无目标/无证据/仅求解、错误边/源归属、低置信、syntax 失败、涂除/草稿、not_applicable、冲突、修订待重验、扣分待确认、复合替代、候选约束、冻结数据校验和小数精确聚合。
- Handler 测试覆盖租户/指派、计算期间 correction 竞争与 409、后续 pending 预览；匹配产生的 evidence 可通过工件引用校验，评分不修改原工件。
- OpenAPI/SDK 明确 nullable 分数、区间、CriterionDecision 和无分值候选；旧理解接口的通用 evidence 契约保持兼容。
- 已核对生产 Assessment/Bank freeze SQL 的 Rubric 字段：ID、version、status、max_score、points、deductions、examples 与当前解析器一致；不把独立 readiness content companion 的简化 Rubric 结构混作实际 Assessment Snapshot。
- 真实 PostgreSQL 测试采用隔离数据库和生产 Source/Handler，验证 live rubric 修改不影响建议与 hash、缺 Snapshot 不回退、跨租户/未指派拒绝、教师修订失效，以及零 ai_grade/final_grade 写入。该测试不是全历史 migration-upgrade 或完整教师 final-submit E2E。
- 本轮数学定向 PostgreSQL 三组回归通过；OpenAPI breaking/生成契约一致性、SDK/Web TypeScript、schema version 000146、数学/契约 Go 单测与 vet 通过。另行启用全部数据库场景的 server 套件触发默认 10 分钟总超时，结束时刚进入旧 STORY-061 的迁移测试（该用例运行 1 秒）；不据此声明全 server 真实数据库套件通过，也不修改该旧 Story。
- 当前只支持显式 Target 的保守代数基线；空 Target 和语义要求需要 MATH-13 的受约束映射与教师确认。依赖错误的 follow-through credit、自我纠正、多解分支、领域语义验证、评分工作台和真实答卷门禁仍待后续，不能由本 Story 的本地回归宣布自动评分准确率或生产批准。
