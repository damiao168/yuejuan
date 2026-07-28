# STORY-060 实现事实审计（2026-07-28）

## 结论

STORY-060 当前应标记为 **In Progress**，不能批准完成，也不能把 STORY-061 的实现置于它之前。

已形成生产价值的主要能力是：

- 学生身份条码 v2 的签名、验证和基础归属候选。
- 花名册对账、显式缺考、身份未决/缺页状态和发布硬门禁。
- `human_grade.ai_grade_id` 的服务端关联。
- 删除没有生产者的 `score_anomaly_unconfirmed` 假门禁。
- `grading_mode` 已被双评默认策略消费，不再只是数据库装饰字段。

仍未完成的核心能力是：

- 学生条码跨答卷序列防重、考试 roster 约束、打印包和真实打印回扫。
- OMR 校准从题目级提升到模板 profile 级，并采用无偏分层样本。
- 失败采集文件批内重跑、失败/废弃文件同 hash 重传和质量人工放行。
- 卡纸/插页场景的完整归属 E2E。

## 审计方法

本审计以当前 `main` 代码事实为准，逐项对照：

- `docs/stories/STORY-060-objective-automation-integrity.md`
- 迁移 000051～000053
- capture、grading、review、score 的 memory/PostgreSQL 实现
- Web 管理端相关页面
- STORY-060 PostgreSQL E2E 和现有单元测试

不把文档计划、字段存在、路由存在或 mock 测试等同于业务能力完成。

## 范围逐项审计

| 范围 | 状态 | 已有证据 | 未完成事实 |
| --- | --- | --- | --- |
| 1. 条码承载学生身份 | 部分完成 | `BarcodeClaims` v2 包含 `student_id`、`sheet_serial`；旧 v1 保持验证；有签名/篡改单测；有签发路由和基础归属候选 | 签发仅验证租户内学生存在，没有限制为当前考试 roster；`sheet_serial` 未作为持久化唯一事实，无法可靠发现跨文件/跨 submission 重复；没有打印包 UI/PDF；没有插页 E2E 和 30 份实体回扫报告 |
| 2. 花名册对账与缺考 | 软件范围完成 | migration 000052/000053；管理员 roster API/UI；缺考原因和审计；缺考后不再触发 missing submission；未识别、缺页、重复答卷和缺少答卷进入发布门禁；500 人 PostgreSQL E2E | 历史已发布考试不会被追溯重开；这属于迁移/运营边界，不阻断本项 |
| 3. OMR 校准范式修正 | 未完成 | 原有题目级校准、人工标注、双人审批/撤销和前端标注抽屉仍可用 | `CreateOMRCalibrationInput` 仍要求 `question_id`；session、scope、批准查询仍绑定 question；样本 SQL 仍要求 `decision='selected' AND confidence >= minimum`，排除了 ambiguous/blank/低置信样本；没有模板级校准场次和克隆语义 |
| 4. 采集链路自救 | 未完成 | 页面配准任务已有 retry；Worker Runtime 有通用重试/重排 | `QueueBatch` 只查询 `status='uploaded'`；失败 capture file 不能通过批次重新排队；同 batch 同 sha256 无条件标 duplicate；没有修改 `submission_page.quality_override` 的业务 API/UI，现有字段仍只是空对象/读取能力；旧 submission 直传管线仍存在 |
| 5. 门禁与数据诚实化 | 大部分完成 | 假门禁已删除；AI 建议关联由服务端取值；界面不再把 0 置信显示成真实校准结果；考试双评模式已有后端 fallback policy | 仍需在最终 STORY-060 验收时复核全部公式/编程题开考门禁和生产页面文案，不能仅依据静态搜索批准 |

## 原验收标准审计

| 验收项 | 状态 | 结论 |
| --- | --- | --- |
| 1. 500 人 roster、抽走 3 份、缺考后发布 | 通过 | `TestStory060FiveHundredStudentRosterScaleE2EWithPostgresTestDatabase` 覆盖 497 份答卷、3 人缺考和发布门禁；HTTP 对账已在本地真实 PostgreSQL 执行 |
| 2. 扫描仪卡纸/中间多一页且后续不串位 | 未通过 | 没有对应 E2E；当前每个 capture file 先创建一个 submission，条码候选尚未形成跨页、跨文件的 sheet serial 持久化约束 |
| 3. 30 份试印回扫及旧条码兼容 | 部分通过 | 旧 v1 与新 v2 的软件兼容单测通过；没有打印包和实体扫描报告，因此不能批准物理链路 |
| 4. 20 题模板 ≤200 次标注完成全模板校准 | 未通过 | 当前仍是每题独立校准，且存在高置信 selected 样本偏置 |
| 5. 失败文件重跑、同文件重传、质量 override | 未通过 | 三条产品路径均未落地；页面配准 retry 不能替代 capture file 重跑和质量放行 |
| 6. 无假门禁、AI 建议正确关联 | 通过 | 假门禁已移除；`human_grade.ai_grade_id` 迁移、memory/PostgreSQL 和测试均存在 |
| 7. 全量工程门禁 | 持续验证 | 每次后续切片仍需重新运行，不能用历史通过结果替代最终验收 |

## 发现的产品与安全缺口

### 条码签发范围不足

当前学生条码签发检查的是“学生属于同租户”，不是“学生属于该模板考试关联的应考班级”。这会允许管理员为同租户但不参加本场考试的学生签发有效条码。

收口要求：

- 签发只接受当前考试 roster 中的 active student。
- 签发结果保存不可变的 sheet serial 事实，而不是只返回给调用方。
- 同一 sheet serial 再次出现在不同 capture file/submission 时必须进入冲突队列，不能覆盖归属。

### 条码不是打印能力

返回字符串数组不等于“可打印答题卡”。打印包必须锁定：

- 模板 content hash、学生、sheet serial、页码和 key id。
- 条码尺寸、纠错级别、静区、位置和打印缩放。
- 打印批次、作废/重印原因和审计。
- 真实打印机与扫描仪回扫的解码率和配准偏差。

在实体报告通过前，产品只能称为“学生条码签发基础”，不能称为“学生答题卡打印闭环”。

### OMR 评测存在选择偏置

当前样本只从模型已经输出 `selected` 且达到高置信阈值的结果中抽取。该集合不能估计 blank、ambiguous 和低置信错误，因此即使审批通过，也不能证明全量自动确认安全。

收口要求：

- session 作用域提升至 template content hash + profile hash + reference。
- case 保存原 question id，但批准事实不再按 question 分裂。
- 对 selected、blank、ambiguous 和置信区间分层抽样。
- 每个选项位达到最小覆盖，不能只统计模型选择结果。
- 模板克隆必须显式判断 content/profile/reference 是否完全相同。

### 采集“重试”概念混淆

系统已有 page registration retry 和 Worker Runtime retry，但 STORY-060 要求的是用户可以在采集批次中恢复失败文件、重新上传失败文件和人工放行质量误判。这三项当前仍缺失，不能用底层 Worker 重试替代。

## 收口切片

### STORY-060A：学生条码归属闭环

- roster 限制和批量签发幂等。
- 持久化打印批次、sheet serial 和作废/重印。
- capture page/submission 绑定 serial 并检测跨文件冲突。
- 插页/缺页/重复页 E2E。
- 打印包 UI 与可下载文件。
- 30 份实体回扫作为外部验收证据。

### STORY-060B：模板级 OMR 校准

- 新迁移与向后兼容读取。
- profile 级 session 和批准查询。
- 无偏分层抽样与覆盖统计。
- 校准场次、标注 UI、克隆和撤销。
- 20 题模板 ≤200 次标注的真实 PostgreSQL 验收。

### STORY-060C：采集自救

- failed capture file 批内重排，创建或恢复真实 Worker task。
- 失败/作废源文件允许同 hash 重传；正常完成文件仍防重复。
- 质量人工放行 API、原因、操作者、时间、原问题和审计。
- 放行后恢复配准链路，不能只改展示状态。
- 关闭或明确废弃旧 submission 直传通道。

### STORY-060D：最终回归与实体证据

- 运行全部工程门禁和 PostgreSQL E2E。
- 实体打印/扫描报告。
- 500 人考试、卡纸、漏扫、缺考、重复答卷和质量误判联合演练。
- 更新 Story Implementation、Review、Fixes 和 Approval。

## 顺序决定

实施顺序保持正确性优先：

1. 060A 的 roster 限制、serial 持久化和冲突检测。
2. 060B 的 OMR 无偏模板级校准。
3. 060C 的采集自救。
4. 060A 打印包与 060D 实体回扫/联合验收。
5. STORY-060 批准后，才开始 STORY-061A 实现。

实体打印机/扫描仪不影响软件切片继续开发，但会阻止 STORY-060 最终批准。
