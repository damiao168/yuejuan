# STORY-MATH-13：Grading Agent v2 数学生产接线

## 状态

Implemented / Feature Flag Off / Real-model Shadow and Teacher UI Pending

## 实现

- 数学新链路只在三个条件同时成立时启用：`EDUGRADE_MATH_GRADING_V2_ENABLED=true`、Assessment Snapshot 学科为 mathematics、当前存在与 segment/snapshot 精确绑定的 verified effective math artifact。开关默认关闭；非数学和不满足条件的请求保持既有 v1 行为，生产 AI eligibility 会因缺少数学 ParserQuality 而保守拒绝缺证据的数学请求。
- `MathEvidenceSource` 统一消费 MATH-08A effective projection，并发送最小化模型上下文：step 文本/公式引用、canonical LaTeX、verification status/reason、bbox、rubric evidence 与分维度质量。内部 AST、symbol/relation、verification details 和存储引用不进入模型请求。
- 数学质量拆为 recognition、formula、structure、verification、rubric mapping；`critical` 固定为五项最小值，直接作为 AI eligibility 的数学 ParserQuality。未验证工件、识别/结构冲突、required rubric 未决、diagram、替代解候选或低于门槛均路由人工。
- 生产 Go adapter 调用 `/grading/grade-v2`，只接受 score-free criterion candidates。Python 生产入口使用结构化输出 schema，模型只能返回 rubric point、evidence IDs、semantic status/confidence/reason 与 alternative-solution 信号；响应若夹带 score/total、虚构 ID、重复 point、非法风险或版本/telemetry 不匹配会失败关闭。
- 候选返回后由 API Gateway 重新执行 MATH-12 `ScoreFrozenRubric`。模型候选不能授分或扣分；成功建议的 `suggested_score / matched_points / missing_points / evidence` 全部来自服务器冻结 Rubric 和 canonical math facts。任何 unresolved point 返回 `suggested_score=null` 与区间并转人工，不持久化成功 `ai_grade`。
- 推理前后的 artifact ID/version/effective correction lineage 会再次读取比对。教师 correction 或新 artifact 在推理期间到达时，run 进入 conflict，不允许旧证据建议落库。
- 启用 v2 后必须获取当前 crop metadata，并验证其 SHA256 与 artifact 的输入 hash 相同；新 crop 应用而数学 worker 尚未产出新工件时，旧工件和旧幂等建议不能继续消费。已解析 crop 在模型调用前再次核对，结算时重读 crop metadata；图片漂移返回 conflict，crop 不可用转人工，并终结相应 worker task。输入 hash 只用于内部校验，不增加模型上下文字段。
- direct、batch enqueue、worker execute/complete 共用相同版本绑定和服务端重算。Worker execute 为兼容内部结果信封返回服务端结算分；complete 只消费无分值候选，丢弃信封中的分数、评分点、证据与 raw output，再次重算。伪造 `AdapterOutput.SuggestedScore=999` 不能改变评分；Mock/非 teacher_suggestion 模式、非法候选会失败关闭，替代解风险转人工。v1 worker/result schema 保持兼容。

## 版本、持久化与契约

- `subjective_grading_run` 和 `ai_grade` 增加 `math_artifact_id / math_artifact_version / math_correction_revision / math_scoring_version`。复合外键检查 tenant、artifact、version、answer_segment 的精确组合，check constraint 保证绑定同时为空或同时有效。数学迁移为 `000148_story_math13_grading_bindings.sql`；工作树另有 auth `000149`，公共 schema metadata 已随最新迁移同步。
- 建议绑定中的 correction revision 是 verified artifact 已吸收的父修订与该 artifact 新增 effective correction revision 之和；MATH-12 评分响应仍分别保留 verified lineage 和本工件局部 revision，避免重验证后把已吸收修订误记为零。
- 幂等身份包含 answer/rubric/model/prompt 版本以及四项数学绑定。任一 artifact、correction 或 scorer 版本变化都不是同一次建议。
- 公共 `POST /api/v1/answer-segments/{id}/subjective-ai-grade` 已进入 OpenAPI，不再依赖 legacy route exception；Generated SDK 包含 `SubjectiveAIGrade`、`SubjectiveGradingRun`、数学质量、score-free candidate 与 unresolved review 响应。
- grading-agent v2 request/response JSON Schema 与 fixtures 已升级为数学结构证据和 candidate-only response。v1 contract、route 和非数学 adapter 未改变。
- 所有 AI 结果固定为 `teacher_suggestion` 且 `needs_human_review=true`。本 Story 不写 `human_grade` 或 `final_grade`，不放宽双评、仲裁和成绩发布权限。

## 验证

- Go：最终 `go test -p 2 ./...` 与 `go vet -p 2 ./...` 全包通过，包含 subjective v2 builder/contract/HTTP adapter、数学 evidence/settlement/幂等/version drift、config、OpenAPI 回归。入口测试覆盖 verified crop + math evidence、精确版本幂等重放、缺 crop 不推理、未决项无 grade、worker execute/complete 重算、非法候选/Mock/模式/替代解和版本冲突后终结 task；crop 变更后拒绝旧缓存重放、推理期间 crop 漂移、解析图片与已准备工件不一致及 worker crop 失效清理均有回归。strict decoder 拒绝注入总分，必填字段缺失/null、重复风险也不能借 Go 零值绕过契约。全包命令未配置数据库 E2E 环境，数据库验证范围以下述独立运行结果为准。
- Python：v2 contract、离线 seam、生产 application 和真实 HTTP `/grading/grade-v2` 路由定向 `17 passed, 15 subtests passed`；AI services 全套 `125 passed, 45 subtests passed`。定向 Ruff 通过；全套 Ruff 仍有其他既有测试文件的 10 条 import-sort 诊断，未批量改写。测试模型是结构化 Fake，不代表真实 VLM 质量。
- PostgreSQL 18：先前隔离数据库执行全历史迁移至 `000148` 和生产 Router/Store 核心工作流通过；本轮另一隔离库执行至工作树最新 `000149`，生产 Store 的 run/grade 四项绑定回读、batch 冻结计划恢复，以及错 version/segment、缺 artifact、不完整绑定四组约束断言全部通过，明确核对实际 FK/check 名称。测试后自动删除隔离库。绑定测试复用冻结物理考试合成夹具，仅验证存储边界，不代表数学 eligibility、真实模型或教师提交完整 E2E，也不代表学校预生产升级或容量验收。
- 全工作区 TypeScript typecheck 通过；与工作树新增 auth 契约同步后的 OpenAPI/SDK、route coverage、breaking-change、contract gates 与 schema version 门禁再次通过，当前 schema metadata 为 `000149`。

## 未覆盖边界

- 功能旗标仍关闭，未进行真实本地/第三方多模态模型 shadow、受治理数据出境审批、延迟/吞吐、成本、断网或 provider SLA 验证。
- 没有通过本 Story 宣布数学自动评分准确率、自动 final score 或生产晋升；MATH-15 的真实错答/替代解/风险召回 benchmark 与 promotion gate 仍是前置外部证据。
- MATH-14 仍需把 server rubric decisions、建议区间、step/bbox/verification 和 stale 版本/crop 状态整合到教师阅卷工作台，并覆盖 correction → reverification → new suggestion 的浏览器闭环；本轮 crop 守卫只覆盖 AI grading 入口，不能把独立 MATH-12 只读预览误当成已完成同样的工作台 stale 防护。
- follow-through、自我纠正、复杂定义域、单位/substitution、几何图形和未知替代解仍保持人工决策，不由 LLM 候选静默结算。
