# STORY-045 生产化差距清单与上线路线图规格

## 目标

回答“距离生产应用还差什么”，并把答案固化为可跟踪的后续 Story 队列。

## 范围

本轮处理：

- 审阅现有验收、部署、安全和 STORY-044 剩余风险文档。
- 明确当前系统是否可正式生产上线。
- 梳理不可上线阻断项、生产化里程碑和后续 Story 顺序。
- 修正 Web README 中已经过期的 mock 登录说明。
- 明确 `lab/` 是智能体训练实验约束，尚未接入生产链路，不纳入本轮生产能力评估。

本轮不处理：

- 不接入真实 OCR、真实 LLM、Agent worker runtime 或扫描仪驱动。
- 不修改业务 API、数据库迁移或 Web 运行时代码。
- 不把实验性 `lab/` 内容接入生产链路。

## 规格审阅

- 用户关心的是生产应用差距，而不是继续把演示能力当生产能力包装，因此必须清楚区分“可演示”“可试点”“可正式生产”。
- STORY-044 已修掉严重和高风险阻断，但剩余中风险和真实 AI/OCR/Agent 能力仍足以阻断正式上线。
- 由于 `lab/` 尚未开始接入，只应记录为实验边界；不能把它作为当前产品能力或生产风险来源。
- Web README 仍写“当前登录为 mock session”，与 STORY-044 实现不一致，必须修正文档事实。

## 规格修改

审阅后将本 Story 收敛为文档和路线图交付：

- 新增生产化路线图，给出 P0 阻断项、M0/M1/M2 里程碑、后续 Story 队列和上线判断规则。
- 新增 STORY-045 闭环记录和审批记录。
- 更新故事索引。
- 更新 Web README 的登录和 mock 页面说明。

## 实现

新增：

- `docs/deployment/production-readiness-roadmap.md`
- `docs/stories/STORY-045-production-readiness-roadmap.md`
- `docs/stories/STORY-045-approval.md`

修改：

- `docs/stories/README.md`
- `apps/web-admin/README.md`
- `docs/deployment/enterprise-acceptance-checklist.md`

## 实现审阅

逐项检查：

- 已明确当前结论：系统尚不能作为正式生产级智能阅卷应用上线。
- 已明确受控演示、受控试点、正式生产三档标准。
- 已列出真实 OCR、真实主观题 AI、Agent worker runtime、生产认证/会话、多租户数据库级隔离、Web mock 页面、预生产验收、运维 runbook、Windows EXE 真实采集等阻断项。
- 已将后续工作拆为 STORY-046 到 STORY-053，便于继续按单 Story 闭环推进。
- 已明确 `lab/` 不纳入当前生产链路评估。
- 已修正 Web README 中过期的 mock 登录说明。

## 修改实现

实现审阅未发现需要修改业务代码的问题。本轮只补充和修正文档，不改变运行时行为。

## 验收标准

- 生产化路线图存在，且能回答“还差什么”和“下一步做什么”。
- STORY-045 文档包含规格、规格审阅、规格修改、实现、实现审阅、修改实现。
- 审批记录存在并给出明确结论。
- 故事索引包含 STORY-045。
- Web README 不再声称登录是 mock session。

## 剩余风险

- 路线图不是生产能力本身，后续 P0 Story 仍需逐条实现和验收。
- 真实 OCR/LLM/Agent Runtime 仍未接入。
- Web token 仍在 `localStorage`，需 STORY-046 处理。
- 数据库级复合租户约束仍需 STORY-047 处理。
