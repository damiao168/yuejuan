# STORY-069F：使用历史与发布版本 CTT 回流

状态：Planned。优先级：P0。父 Story：[069](STORY-069-versioned-question-bank-rubric-library.md)。依赖：069B/C、Score Release；不要求 072 先实现。

## 问题与目标

题库价值需要真实考试反馈，但现有 report 查询读取 live final_grade/question，不能直接作为版本级不可变统计源。增加发布版本输入 adapter 与可重放投影，复用纯统计计算。

## 范围与数据契约

- question_bank_usage 分别记录 materialized、exam_ready、delivered/实际曝光、valid_response 等事实及来源 event/command，避免候选生成当实际曝光。每条绑定 item_version、exam/question、来源 bundle hash 与时间，按明确业务键防重。
- question_bank_stat_snapshot 绑定 tenant、item/version、exam/question、release_id/version、algorithm_version、sample_policy_version、exam_used_content_hash、N、覆盖率、时间、分值/群体与指标有效性。
- 发布后的任务读取 score_release_item/question、相应冻结题目/Rubric，原始响应证据若有则读取与 release 绑定的追加式分析证据。固定一个发布版本，不混入后继发布或正在改分事实。
- materialize 后考试内修改题干/答案/评分保持 provenance 但标记 deviation；与原 bundle 偏离的样本只作实例分析，不进入源版本校准统计。第一阶段不隐式缩放分值。
- 初版指标：有效 N、平均得分/得分率（现有 difficulty 字段含义）、分布、高低组区分度和有证据时的选项分布。定义高低组比例、并列处理、最低 N 与可用状态；复用算法前核对旧 helper 的零样本和 rounding 语义。
- 缺考、无效作答、未知评分证据排除并计数；没有响应或 N 不足则指标为 null + reason，不以 0 表示未知。
- 跨考试当前汇总只对同版本、可比口径汇总 N 与得分/满分贡献；区分度和选项指标默认逐场展示。逻辑 Item 展示各版本概览，不把 v1 参数赋给 v2。

## 重放与重发布

来源快照按唯一键 upsert/追加且内容 hash 验真；同 release 任务重放不增 N。当前聚合每 exam/question 只计 current published release，发布 v2 原子替换该考试 v1 的贡献，v1 snapshot 保留可查。历史不同考试间学生可能重复，汇总 N 标为 response_count，不误称独立学生数。

发布触发复用既有 outbox 语义或可靠的发布后扫描/重放任务，在现有 Publisher 边界接线；不创建第二个抢全局 published 标记的 dispatcher。首次可用管理命令重建指定 release 投影，失败/恢复/重试有耐久记录；072 后续做通知。

若选项响应尚未冻结，首版显示 unavailable。需要补充时增加独立的不可变 analysis evidence，绑定 release 与原响应 hash；旧 release 缺证据保持 unknown，不事后读取可变学生答案填充历史。

## 非范围

不直接读 live final_grade 作正式回流、不跨版本混合校准、不做 IRT 参数、不自动判题质量或删题、不用 synthetic 数字充当真实质量证据。

## 预计修改文件

扩展 `internal/questionbank/` usage/stats/projector 与迁移；新增 `internal/report/` 的 release dataset adapter 或将纯算法抽为共享 helpers，修改 scorerelease 只读查询接口与必要 outbox 接线；管理端统计/使用记录、OpenAPI/SDK。新增发布分析证据时采用新表，不 UPDATE 已发布 release。

## 测试方式

Go 使用人工可核算分布验证 CTT、null/N 与偏离排除；真实 PostgreSQL 验证 release 一致读取、重放、重发布贡献替换、并发投影、任务崩溃恢复和权限。UI 显示版本、N、群体、指标原因与 release 下钻。算法 fixture 证明计算逻辑，真实难度/区分度有效性另需学校数据。

## 验收标准

1. 相同 release 重放后 N/usage 不变，发布 v2 当前贡献只计一次，v1 历史快照保持不变。
2. 发布后 live 分数/题干变化不改变该 release snapshot；缺冻结响应则选项分布明确 unavailable。
3. ItemVersion v2 没样本时不展示 v1 校准值；同 Item 不同版本清楚分列，跨场统计带口径。
4. 低 N、缺考、无效与内容 deviation 有原因与排除计数；得分率越高越容易，未知不显示零。
5. usage 的创建、ready、实际曝光与有效样本单位不同；重复事件防重，任务失败可恢复。
6. 题库 statistics 权限控制所有列表/详情/下钻，不读取未经授权的学生明细；汇总不泄露身份。

## 规划审阅与实施记录

规划已核对 [report 数据查询](../../services/api-gateway/internal/report/store_postgres.go)与 [CTT 计算](../../services/api-gateway/internal/report/stats.go)，区分可复用算法与需替换的事实源。实现、实现审阅、修正和 Approval 待证据；本切片完成后才将观测难度作为 070 可靠输入。
