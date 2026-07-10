# STORY-004 自审审批记录

## Story

STORY-004 数据库模型设计

## 审批结论

Approved

## 验收项

| 验收项 | 证据 | 结论 |
| --- | --- | --- |
| 35 个核心实体全部覆盖 | 检查命令确认 35 个核心表均出现 `CREATE TABLE` | 通过 |
| 所有核心表支持 tenant_id | 设计文档通用原则要求核心业务表支持 `tenant_id`，每个核心表 DDL 均包含该字段或解释例外 | 通过 |
| 关键表包含 created_at/updated_at | 设计文档通用字段和各表 DDL 覆盖时间字段；`audit_log` 明确为不可变例外 | 通过 |
| 分数不用 float | 分数均使用 `NUMERIC(8,2)`，并补充分数范围 check constraint | 通过 |
| AI 阅卷结果保留版本 | `ai_grade` 包含 `model_version_id`、`prompt_version_id`、`rubric_version_id` | 通过 |
| 改分可追溯 | 第 14 节定义 human_grade 追加记录、final_grade 来源、audit before/after | 通过 |
| 审计日志字段完整 | `audit_log` 包含 actor、action、target、before、after、reason、ip、user_agent、request_id | 通过 |
| 支持软删除 | 通用字段和各关键表包含 `deleted_at`，审计日志解释为不可变例外 | 通过 |
| 支持成绩状态流转、双评、仲裁、申诉 | 第 15-17 节分别说明发布状态、双评仲裁和申诉 | 通过 |
| 支持文件对象存储路径 | `file_asset` 包含 bucket/key/hash/owner 字段 | 通过 |
| 关键索引和数据隔离说明 | 第 18-19 节覆盖索引和租户隔离规则 | 通过 |
| migration 决策明确 | 检查命令确认迁移目录为 0，文档第 20 节说明不生成 migration 的原因 | 通过 |

## 运行命令

```powershell
$core = @(...)
$requiredPhrases = @(...)
```

结果：

```text
STORY-004 schema check passed
Core tables: 35
Migration dirs found: 0
Lines: 1027
```

## 剩余风险

- 当前是数据库设计文档，尚未落地 migration。
- 真实外键、枚举、RLS 和复合 tenant 外键需要在后续后端/迁移 Story 中实现和测试。
- `tenant.tenant_id = tenant.id` 需要 migration 阶段用 check constraint 保证。

## 下一步

进入 `STORY-005 后端基础服务骨架`，实现 Go 后端基础能力、健康检查、依赖连接抽象、Docker Compose 校验和基础测试。
