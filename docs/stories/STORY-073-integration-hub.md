# STORY-073：学校集成中心

状态：Planned。优先级：P0。依赖：既有组织/auth/Score Release；Webhook 依赖 072。

## 问题与范围

学校名单同步与成绩回写需要可预览、可追溯的边界。首版提供 OneRoster 1.2 CSV rostering 子集，按 Parse → Map → Diff → 管理员确认 → Apply 推进。

| 切片 | 交付范围 |
| --- | --- |
| 073 首切片 | connection、sync_run、object_map、conflict；CSV 的 manifest、orgs、academicSessions、users、courses、classes、enrollments 声明子集与依赖验证 |
| 后续 REST | 同一映射/Diff 引擎的拉取、分页、增量游标与重试 |
| 后续 Webhook/API Key | 签名、凭据 scope、撤销、幂等投递及运营；出站从 072 持久化投递读取 |
| 后续 LTI | 注册与 1.3 Launch，再单独交付 NRPS、AGS、Deep Linking；真实 LMS 联调 |

- external_id 只作 `(tenant,connection,object_type,external_id)` 映射键，内部实体继续使用 UUID；不得按姓名静默合并用户。
- Diff 固定源文件 hash、mapping_version、组织目标范围和目标 revision，显示新增/修改/停用/冲突；确认时目标已变则重新 Diff。分批 Apply 保存回执和恢复点，明确事务边界，不承诺跨全量批次一次原子提交。
- bulk/delta 和缺失记录含义遵从声明 profile；不把文件遗漏直接当删除，不改写已冻结考试名单。
- API Key 按租户/connection/scope 管理并只保存可校验摘要；出站密钥受控加密。Webhook 限制目标网络、验证签名与重放窗，失败可重试。
- AGS/Gradebook 回写只读取不可变 published release，保存 release_id、映射与投递结果；后继发布是新投递，不覆盖历史记录。LTI 角色映射不能直接赋予平台管理员或题库内容权限。

## 非范围

首版不宣称完整 OneRoster 三服务或认证，不交付 REST/LTI/Gradebook 全集，不上传即覆盖数据库，不同步改动历史考试快照，不把 LTI 登录等同普通机构 OIDC 登录。

## 预计修改文件

拟新增 `internal/integration/`、连接/同步/映射/冲突迁移、`apps/web-admin/src/features/integrations/`、`docs/api/integrations.md`，修改组织命令接口、auth/路由、OpenAPI 和 SDK；对接服务按切片添加。

## 测试方式与验收标准

授权、无真实学生信息的 CSV 夹具验证依赖、bulk/delta 与编码；Go 与 PostgreSQL 验证 stale Diff、冲突、恢复、重复 Apply、external_id 隔离。UI 验证 Diff 与明确确认步骤。后续协议模拟器和真实 SIS/LMS 联调分别记录，不混用证据。

首切片验收要求同步重试不重复实体、冲突阻断相应变更、目标变更重新预览、停用不破坏历史考试，映射和管理员审批可查。支持 profile 与未支持字段必须列入 API 文档。标准边界依据 [OneRoster 官方概览](https://www.1edtech.org/standards/oneroster)和 [LTI 官方概览](https://www.1edtech.org/standards/lti)。

## 规划审阅与实施记录

规划已明确首版 CSV 与后续协议的独立验收，不把学校接入缺口混回旧 066 编号。当前实现、实现审阅、修正与审批待各切片记录。
