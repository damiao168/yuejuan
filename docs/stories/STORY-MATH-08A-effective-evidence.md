# STORY-MATH-08A：Effective Evidence / Correction Projection

状态：Implemented（真实学校试点与自动重验证不在本阶段）。

## 编号说明

新数学闭环路线中的 MATH-08 对应本 Story。仓库已有 `STORY-MATH-08-pilot-gates.md`，因此使用 08A 保留既有 Story 与验收历史，不重命名或覆盖已完成项目。

## 实现

- `ResolveEffectiveArtifact` 从当前 immutable artifact 与该 artifact 的最新 correction 得到统一投影；无校正时使用 base contract，非空校正读取失败时不静默回退旧识别结果。
- PostgreSQL 独立使用 `ORDER BY revision DESC LIMIT 1`，不从有 500 条上限的历史列表推断最新版本；沿用 `(tenant_id, artifact_id, revision)` 唯一索引，无需新 migration。
- GET 保留原始 `artifact` 和审计 `corrections`，新增 `effective_artifact`、`correction_revision` 和 `corrected`；有效 contract 与 base 身份元数据分开。
- 校正禁止更换 segment、snapshot、subject、input hash 或 engine version，已被新 artifact 取代的旧 artifact 不接受新校正。
- 可选 `expected_correction_revision` 提供并发保护。新工作台提交此字段，旧客户端省略仍可使用原契约；PostgreSQL 在 artifact 行锁获取后读取最新 revision。
- MemoryStore 深拷贝补齐嵌入 contract 被身份字段遮蔽的值；校正输入、输出与历史读取都深拷贝，防止通过引用修改 append-only 审计数据。
- 工作台的公式、块、步骤、关系与 Rubric 证据全部从 effective contract 加载；保存基于有效内容，但使用 base artifact ID/version 定位。异步读取过期响应不能覆盖已切换的答题区。
- 客户端选择函数位于独立纯模块，不从 React 组件导出，避免 Vite 热更新因组件/函数混合导出而重挂载工作台。
- OpenAPI 正式定义有效证据与 revision 字段，SDK 从现有工作区契约再生成。

## 验收

Go 测试覆盖无校正 contract 有效性、多次校正的完整投影、当前 artifact 切换、租户隔离、最新查询失败、revision 501、并发冲突、不可变 source binding 和审计引用隔离。HTTP 测试覆盖 GET → POST correction → GET effective contract、409、403 与跨租户 404。

前端 9 个定向回归验证 quantitative subject 边界、effective evidence 优先选择、raw artifact 保持不变与旧服务端响应兼容。Playwright CLI 驱动真实 Chromium 与本地 Vite 工作台，使用 stateful API 夹具验证：原识别 `x=2` / 已有校正 `x=3` → 修改为 `x=5` 并调整步骤顺序 → 第二次校正 `y=6` → 刷新后仍保持 `x=5 / y=6`、新步骤顺序与 revision #3；两次保存携带 revision 1 / 2，human grade 提交数始终为零。这不是真实学校答卷或端到端 PostgreSQL API 测试。

真实 PostgreSQL 查询/锁回归 `TestPostgresMathEffectiveEvidenceProjection` 使用独立临时数据库和最小测试表，直接调用生产 artifact / correction stores，覆盖并发行锁、revision 冲突、最新 revision 503 不受历史 500 条上限影响、tenant 隔离和 obsolete artifact 拒绝。它不替代全量历史迁移、RLS 权限或最终成绩流程验证。

2026-09-13 定向验证：Go mathunderstanding / apicontract、真实 PostgreSQL projection 回归、前端 9 个 Vitest、Web / SDK 类型检查、定向 Biome lint、生成契约一致性、OpenAPI breaking gate 和 `git diff --check` 通过。浏览器截图与夹具位于本地忽略目录 `output/playwright/math08-effective/`。PostgreSQL 临时库在测试 cleanup 中清理；未改动现有业务库。

Windows 环境未启用 CGO，`go test -race` 无法运行；并发行为由上述同步竞争测试单独覆盖，不宣称通过 race detector。

## 剩余边界

本阶段不调用 SymPy、不生成新的 AI 建议、不改变人工成绩，也不声称教师修改后的 LaTeX 已重新通过原 AST/verification。后续 Symbolic Verification Runtime 必须重新产生验证并冻结 correction revision，然后评分引擎才可消费该版本；既有验证在校正之后不能作为新内容已验证的依据。
