# STORY-042 企业级验收文档

## 状态

Approved

## Plan

### 目标

编写可交付给真实客户、学校或机构的 EduGrade Enterprise 企业级验收文档，明确功能、流程、安全、运维、部署和不可上线阻断项的验收方法。

文档保存到：

```text
docs/deployment/enterprise-acceptance-checklist.md
```

### 范围

1. 编写企业级验收清单，覆盖用户要求的 22 类验收主题：
   - 产品功能验收
   - 考试流程验收
   - 阅卷流程验收
   - AI 阅卷验收
   - OCR 验收
   - 人工复核验收
   - 双评仲裁验收
   - 成绩发布验收
   - 申诉验收
   - 学情报告验收
   - 权限验收
   - 多租户隔离验收
   - 审计验收
   - 文件安全验收
   - EXE 客户端验收
   - 离线阅卷验收
   - 私有化部署验收
   - 性能验收
   - 安全验收
   - 数据备份恢复验收
   - 运维监控验收
   - 不可上线阻断项
2. 每个验收项包含：
   - 验收目标
   - 操作步骤
   - 预期结果
   - 是否阻断上线
3. 文档必须区分：
   - 已实现能力。
   - mock/placeholder 能力。
   - 未实现或待企业化补齐能力。
   - 必须阻断上线的问题。
4. 文档需要可被客户验收小组直接执行，包含验收记录字段、证据要求和签署建议。

### 非范围

1. 不实现新功能。
2. 不修改后端、前端或 EXE 业务代码。
3. 不伪造性能、安全、OCR、AI 或部署验证结果。
4. 不提前进入 STORY-043 上线前代码审查。

### 预计新增文件

1. `docs/deployment/enterprise-acceptance-checklist.md`
2. `docs/stories/STORY-042-enterprise-acceptance-checklist.md`
3. `docs/stories/STORY-042-approval.md`

### 预计修改文件

1. `docs/stories/README.md`

### 验收标准

1. `docs/deployment/enterprise-acceptance-checklist.md` 存在。
2. 文档覆盖 22 类验收主题。
3. 每类验收均包含验收目标、操作步骤、预期结果、是否阻断上线。
4. 文档明确指出 mock/placeholder/未实现能力不得作为正式上线通过依据。
5. 文档包含客户可执行的验收记录字段和证据要求。
6. Story 文档和审批记录完整，索引更新。

## Plan Review

1. 范围符合 STORY-042：只交付企业级验收文档，不修改业务实现，不进入上线前代码审查。
2. 用户显式要求全部纳入：保存路径、22 类验收主题、每项四类字段和真实客户交付用途均已覆盖。
3. 与当前仓库实际匹配：仓库已有 PRD、架构、API、部署、安全、UI 和 Story 审批记录；验收文档可引用当前实现状态，并对 mock/placeholder 能力作阻断说明。
4. 风险控制：性能、OCR、真实 AI、真实 Agent worker、生产级备份恢复等尚未完全验证的能力必须标记为上线阻断或需补充验证，不写成已通过。
5. 审阅结论：计划可执行，进入 Implementation。

## Implementation

- 新增 `docs/deployment/enterprise-acceptance-checklist.md`：
  - 增加验收记录表。
  - 增加证据要求。
  - 增加当前实现边界说明。
  - 覆盖 22 类企业级验收主题。
  - 每类验收均按“验收目标 / 操作步骤 / 预期结果 / 是否阻断上线”组织。
  - 明确 mock、placeholder、stub、not implemented 能力不能作为正式上线能力通过依据。
- 更新 `docs/stories/README.md`：
  - 将 `STORY-042` 标记为 `Approved`。
- 新增 `docs/stories/STORY-042-approval.md`。

## Implementation Review

逐项检查结果：

- 目标文件存在：`docs/deployment/enterprise-acceptance-checklist.md` 已创建。
- 22 类主题覆盖：文档包含产品功能、考试流程、阅卷流程、AI、OCR、人工复核、双评仲裁、成绩发布、申诉、学情报告、权限、多租户、审计、文件安全、EXE、离线阅卷、私有化部署、性能、安全、备份恢复、运维监控、不可上线阻断项。
- 每类验收字段：每类均使用表格记录验收目标、操作步骤、预期结果、是否阻断上线。
- mock/placeholder 边界：文档在证据要求、当前实现边界和不可上线阻断项中明确声明。
- 客户可执行性：包含验收记录、证据要求、签署建议和阻断项。
- 不越界：未修改业务代码，未进入 STORY-043 上线前代码审查。

## Fixes

- 将“当前实现边界”独立成表，避免客户把真实 OCR、真实 AI、真实 Agent worker、语义/视觉证据核验误认为已可上线能力。
- 在“不可上线阻断项”中增加假功能、权限、成绩发布、审计、备份恢复和真实 AI/OCR 宣称阻断项。

## Verification

运行命令：

```powershell
go test ./...
npm.cmd run typecheck
docker compose --env-file .env.example -f infra\docker-compose\docker-compose.yml config
```

结果：

```text
go test ./... -> passed
npm.cmd run typecheck -> passed
docker compose config -> passed with Grafana env warnings
```

文档检查：

```text
docs/deployment/enterprise-acceptance-checklist.md -> exists
docs/stories/STORY-042-approval.md -> exists
docs/stories/README.md -> STORY-042 Approved
```

## Approval

Approved。

本 Story 满足验收标准，可以进入 `STORY-043 上线前代码审查`。
