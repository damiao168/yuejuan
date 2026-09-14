# STORY-070C：曝光控制与并发预留

状态：Planned。优先级：P0。父 Story：[070](STORY-070-assessment-blueprint-smart-assembly.md)。依赖：070B、069F。

## 问题与范围

仅在 solver 读取 usage 时排除“最近使用”无法保护并发确认。增加明确政策、候选预留与最终确认复验，控制实际曝光与题池复用。

- policy_version 固定时间窗、tenant/school/series/人群范围、最近使用禁期、最大正式使用次数、累计/预计接触人数、重复学生限制与 unknown_usage 处理。
- logical Item 与 ItemVersion 双层统计，family 可扩展；发布 v2 不应使题目“半年未用”限制自动清零。版本级统计不混合与逻辑曝光治理是不同用途。
- 候选生成不记实际曝光。教师选择候选后 acquire reservation，固定 candidate/hash、target exam、scope、额度、expires_at；取消、失效与到期释放，事件和释放回执幂等。
- 确认在事务内按稳定 Item/scope 锁序校验最新 usage+有效预留，消费本预留并 materialize；并发额度不得超限。预留到期或超额返回 exposure_conflict，不静默替题或放宽政策。
- 区分 planned/confirmed/actual exposure。确认额度转为正式使用预留，实际投放按可验证 delivered/exam 使用事实结算；没有可靠实际接触证据时标 unknown/proxy，并说明保守估计来源，不声称每个冻结名单学生都已看到试题。
- 已确认但取消考试的释放/保留取决于是否可能已投放，写原因和批准记录；已实际曝光不能删除 usage 来恢复额度。
- 阈值放宽、必选题豁免等例外要求获授权用户和原因，固定政策版本并审计。

## 数据与 API 规划

新增 exposure_policy/version、reservation 及额度/结算事实表，扩展 usage 来源与 run 输入政策快照；唯一业务键防重，额度检查与消费同事务。API 提供 policy preview、candidate reservation/acquire/release、confirm 的 exposure diagnostics；后台 sweep 到期预留，不依赖用户浏览器常在线。

## 非范围

不只在前端禁用按钮、不把候选浏览当投放、不自动修改曝光政策、不删除真实使用历史、不保证来源系统没有记录的线下曝光被精确识别。

## 预计修改文件

扩展 `internal/questionbank/` usage/policy、`internal/testassembly/` reservation/confirm 与迁移；修改事务锁与命令恢复、后台 sweep、管理端策略/阻断/例外记录、OpenAPI/SDK。071 后续提供题池健康总览。

## 测试方式

Go 固定时间夹具验证窗口与过期；真实 PostgreSQL 两个并发 confirm 争同额度、acquire/release/confirm 重放、取消与实际使用结算、租户/范围隔离与数据库崩溃恢复。硬约束 evaluator 在政策变化后重新验证，UI 显示冲突与可解释放宽路径。

## 验收标准

1. 最新实际/预留使用在确认时复核，同一额度两个并发请求至多一个成功，事务不留下半套题目。
2. 候选生成不增加实际 usage，取消/到期预留可释放且幂等，已投放历史不能删除。
3. Item 新版本不绕过逻辑曝光政策，版本统计仍独立；unknown 使用来源明确且遵守政策。
4. 政策窗口/人群/计量单位与命令回执可查；撤权或策略失效使旧预留拒绝，例外需原因和动作权限。
5. 同确认命令重放不重复消费额度或创建题目，过期返回可解释冲突；实际曝光结算不重复记人数。
6. 第一阶段串通发布资产→蓝图→候选→预留确认→ready→发布统计回流，软件与现场证据分层记录。

## 规划审阅与实施记录

规划已补齐并发、版本绕过、实际曝光与代理估计边界。实现、实现审阅、修正与 Approval 待证据；本切片验收后第一阶段组卷软件能力才可宣布完成。
