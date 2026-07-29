# STORY-060 Approval

## Decision

`Software Accepted / Physical Validation Pending`

日期：2026-07-29

STORY-060 的软件实现、PostgreSQL 联合场景、真实浏览器流程和工程门禁已通过，可以开始 STORY-061 的实现。

由于当前没有真实打印机和扫描仪，30 份学生答题卡的实体打印回扫尚未执行。因此本文件是**软件范围的条件批准**，不是 STORY-060 全量 `Approved`，也不是 Production Ready 批准。

## Approved scope

- 学生身份条码 v2、签发台账、旧版兼容和冲突隔离。
- 花名册对账、显式缺考、未决完整性状态与发布硬门禁。
- 模板 profile 级 OMR 校准、无偏分层抽样、双人审批和受控克隆。
- 失败采集文件重跑、失败后同 hash 重传、质量人工放行与审计。
- 旧 submission 直传路径的明确废弃和生产采集入口收敛。
- 门禁事实诚实化和人工评分关联 AI 建议。

## Not approved

- 真实纸张上的条码解码率与配准偏差。
- 具体打印机、驱动、扫描仪、自动进纸器组合的稳定性。
- 正式学校场景的批量印卡和 Production Ready 声明。

## Release gate

STORY-061 可以进入开发和影子环境验证；任何试点或生产发布仍必须等待：

1. 30 份真实打印回扫报告通过。
2. 异常场景由非开发验收人员复核。
3. 本文件的 Decision 更新为 `Approved`。

详细证据见 [STORY-060D 软件联合验收记录](../reviews/STORY-060-software-acceptance-2026-07-29.md)。
