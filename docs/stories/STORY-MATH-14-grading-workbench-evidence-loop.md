# STORY-MATH-14：数学阅卷工作台证据闭环

## 状态

Implemented / Feature Flag Off / Real Teacher and Real-model Validation Pending

## Plan

把 MATH-11～13 已有的 verified artifact、服务端 Rubric 评分和受治理教师建议接入同一个阅卷工作台。教师必须能看见评分点对应的数学步骤与验证状态，定位原答题区域，区分当前建议与历史建议，并在图片、artifact、校正谱系或评分器变化后阻止旧建议采纳。

本 Story 不改变教师最终评分权，不自动请求模型，不开启 `EDUGRADE_MATH_GRADING_V2_ENABLED`，也不以 UI-only 夹具证明真实模型质量或学校现场可用性。

## Plan Review

- 读取顺序固定为 effective understanding → MATH-12 score；两份响应必须精确绑定同一 segment、Assessment Snapshot、artifact/version、effective correction lineage 和 scorer。
- 待确认分必须显示为区间，不能把 nullable `suggested_score` 转为零分。
- 校正成功与验证任务排队是两个独立事实；排队失败不能回滚已经持久化的教师校正，也不能伪报 queued。
- 模型请求只允许有 `grading:manage` 的管理端通过显式动作发起；普通教师仍可独立阅卷。R3 `HUMAN_PRIMARY` 默认隐藏建议及历史分值，只提供显式第二意见入口。
- 采纳建议前必须重新读取数学证据，避免用户看到建议后发生 crop/artifact/correction 漂移的竞态。

## Implementation

### 服务端证据和历史投影

- MATH-12 只读预览在读取评分前后都核对当前 `answer_segment.crop_sha256` 与 artifact input hash。空 hash、非法 hash、替换后的 crop、读取失败或计算期间漂移均失败关闭，不返回旧分数。
- 教师校正响应新增 `verification_status=queued|not_required|unavailable|failed`，只有真实排队成功时才返回 `verification_task_id`；校正本身独立追加保存。
- review context 投影最近 20 条教师建议，按 `created_at/id` 倒序，并保留 artifact ID/version、effective correction revision 与 scorer version。R3 会递归剥离当前建议和历史记录中的分值字段。
- PostgreSQL 生产投影和约束 E2E 验证同一 segment 上的新旧建议顺序、数学绑定回读，以及错误 version/segment、缺 artifact 和不完整绑定的拒绝。

### 教师工作台

- 新增单一 `useMathWorkbenchEvidence` 状态源，集中处理 loading、ready、pending、unavailable、conflict，任务切换时中止旧请求；后台刷新只读证据，不自动触发模型。
- 新增紧凑、无卡片堆叠的“评分点 × 步骤”矩阵，展示服务端 verified/unresolved/score range、逐点评分状态、步骤、SymPy 验证和最多 20 条建议历史。
- 点击步骤会在原答题图上按归一化 bbox 高亮；step 缺 bbox 时仅合并其 active block bbox，不把像素坐标误作比例坐标。证据失效或校正草稿未保存时立即移除高亮。
- 当前建议必须同时满足 segment、artifact/version、effective correction lineage、scorer、Rubric、非 mock、`teacher_suggestion`、`succeeded`、服务端分数和零 unresolved 精确一致。否则只作为“历史建议 · 不可采纳”显示。
- 采纳按钮执行前再读一次服务器证据，并确认教师草稿未在请求期间变化。校正保存后显示真实排队状态，进入 pending；新 artifact 吸收校正后，只有管理端显式点击“重新生成数学建议”才调用受治理入口。
- 答题图内容 revision 绑定当前 math input hash，避免浏览器预取缓存继续显示旧 crop。R3 的第二意见策略在没有建议对象时也生效。

### 契约

- OpenAPI 和 Generated SDK 增加校正验证调度状态、可选 verification task ID，以及有界的建议历史和数学绑定字段。
- 本 Story 没有新增数据库迁移；最终契约门禁按当前工作树最新 `000150` schema metadata 验证。

## 新增文件

- `apps/web-admin/src/features/grading/workbench/components/MathRubricMatrix.tsx`
- `apps/web-admin/src/features/grading/workbench/hooks/useMathWorkbenchEvidence.ts`
- `apps/web-admin/src/features/grading/workbench/mathWorkbenchEvidence.ts`
- `apps/web-admin/src/features/grading/workbench/mathWorkbenchEvidence.test.ts`
- `services/api-gateway/internal/mathunderstanding/workbench_preview_test.go`
- `docs/stories/STORY-MATH-14-grading-workbench-evidence-loop.md`

## 主要修改文件

- `services/api-gateway/internal/mathunderstanding/handlers.go`
- `services/api-gateway/internal/mathunderstanding/scoring_handler.go`
- `services/api-gateway/internal/review/store_postgres_context.go`
- `services/api-gateway/internal/review/task_context.go`
- `services/api-gateway/internal/server/e2e_math_grading_binding_test.go`
- `services/api-gateway/openapi/edugrade-api.openapi.json`
- `packages/sdk/src/generated/client.ts`、`packages/sdk/src/generated/types.ts`
- `apps/web-admin/src/api/mathUnderstanding.ts`、`apps/web-admin/src/api/review.ts`
- `apps/web-admin/src/features/grading/workbench/GradingWorkbench.tsx`、`MathEvidenceInspector.tsx`、`reviewContext.ts`、`gradingTaskContext.ts`、`grading-workbench.css` 及相关组件、hook 和测试

## Automated Verification

- Go：`go test -p 2 ./...` 与 `go vet -p 2 ./...` 全包通过。
- PostgreSQL 17 隔离集群：`TestPostgresMathGradingBindingsRoundTripAndRejectWrongVersion` 通过，包含新旧建议历史和四组绑定拒绝；测试应用当前全历史迁移，schema metadata 为 `000150`。隔离集群和临时文件在验收后清理。
- Web：工作台 6 个测试文件、49 项测试通过；全工作区 TypeScript typecheck 和 web-admin production build 通过。构建只有既有大 chunk 提示。
- Contracts：Generated SDK、491 条注册路由覆盖、OpenAPI breaking、contract gates 与 schema version 门禁通过。
- 定向 Biome 检查 17 个工作台文件无 error，保留 9 条 hook dependency warning；未应用工具标记为 unsafe 的自动修复。`git diff --check` 无 whitespace error，仅报告工作树既有的换行转换提醒。

## Browser Verification

Playwright CLI 使用 UI-only 合成答卷和协议夹具完成以下有头浏览器检查；没有真实学生数据、真实模型调用或最终成绩写入：

- 当前 v3/#2 建议显示 6/6，可定位 S2；高亮 bbox 为 `left 10% / top 25% / width 70% / height 12%`。
- 未决评分显示已验证 4、待确认 2、建议区间 4–6，旧建议和采纳按钮失效，不把待确认项显示成零分。
- crop drift 返回 409 后矩阵显示证据变化，旧建议标记“已失效 · 图片或证据已变化”，采纳禁用。
- 校正草稿未保存和校正排队 pending 都即时禁用旧建议；排队状态明确显示。
- 管理端模拟新 v4/#3 artifact 已吸收校正后，旧 v3 建议仍不可采纳；仅点击一次“重新生成数学建议”产生一次请求，历史从 2 条增至 3 条，新的精确绑定建议才恢复可采纳。
- R3 默认只显示“本题由教师独立评分”和显式第二意见入口，不预填 AI 分数。
- 390×844 视口下 document/body 无横向溢出，矩阵位于 16～364 px 的可视宽度内。
- 普通稳定态控制台只剩既有 Ant Design `destroyOnClose` 弃用提示；crop drift 场景中的 409 网络错误是本验收主动制造的失败关闭信号。

## Implementation Review and Fixes

- 修复 MATH-12 独立预览原先缺少 crop freshness 守卫的问题，避免 AI 入口已失败关闭但工作台仍展示旧服务端分数。
- 将“校正已保存”和“重新验证已排队”拆成可观察状态，移除静默吞掉排队失败的语义。
- 将建议历史并入 review context，避免前端只能看见最后一条而无法解释 stale lineage。
- 采纳流程增加最后时刻的服务器重读与草稿竞态检查；历史建议不能通过快捷键或旧选中状态绕过禁用。
- 浏览器夹具补齐批注和评语模板只读响应，避免把夹具缺口混作 MATH-14 控制台错误。

## Approval

软件实现、契约、确定性测试、隔离 PostgreSQL 和 UI-only 浏览器验收满足本 Story 的代码级验收标准，批准为 Implemented。功能旗标保持关闭；真实模型 shadow、真实错答与替代解 benchmark、教师现场、预生产升级、容量和 promotion gate 继续由 MATH-15 及外部验收负责。本批准不授予自动最终评分权。
