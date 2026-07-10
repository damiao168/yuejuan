# STORY-048 Web mock 页面退场与生产菜单收敛

## Plan

目标：关闭生产化路线图中的 “Web mock 页面退场” P0 风险。生产 Web Admin 默认只展示真实 API 支撑的页面；未生产化页面保留给开发或演示环境，但必须通过显式开关暴露，并保留 Mock 标识。

本 Story 处理：

- 增加 Web 生产路由可见性模型，默认生产隐藏 mock route。
- 菜单按“当前环境可见 + 当前用户有权限”过滤。
- `routeFromPath()` 对生产隐藏 route 返回 `notFoundRoute`，防止 hash 直达。
- `DashboardPage` 从静态 `data.ts` 退场，改为调用真实 API：
  - `listExams()`
  - `listAuditLogs({ limit: 5 })`
  - `getSystemStatus()`
  - 最近考试的 `listSubmissions(exam.id)`
- 生产默认隐藏 `/review`、`/quality`、`/permissions`、`/settings`。
- 开发/demo 或 `VITE_ENABLE_MOCK_ROUTES=true` 时仍可查看 mock route。
- 更新 story 状态、路线图状态和批准记录。

本 Story 不处理：

- 不实现真实 Quality API，后续 STORY-054 处理。
- 不实现完整用户权限管理，后续 STORY-056 处理。
- 不实现系统设置 API。
- 不新增后端 Dashboard 聚合接口，先复用现有 API。
- 不接入真实 OCR、主观题 AI、Agent worker。
- 不接入 `lab/`；`lab/` 仍是智能体训练实验，不作为生产闭环证据。

验收标准：

- `DashboardPage` 不再导入 `apps/web-admin/src/data.ts`。
- 生产可见菜单不包含 `mock: true` route。
- `/review`、`/quality`、`/permissions`、`/settings` 在生产环境不可从菜单或 hash 直接访问。
- 开发/demo 环境可以通过显式开关查看 mock route，且仍有 Mock 标识。
- 静态检查通过，证明生产路由没有未收敛 mock 页面。
- Web TypeScript typecheck 通过。
- 生产化路线图和 Story 索引更新。

## Plan Review

审阅结论：

- 直接删除 `ModulePage` 或所有 mock route 会增加后续开发摩擦，不是最小生产化闭环。
- 只保留 MockBadge 而不隐藏生产菜单，仍会让验收人员看到未实现页面，不能关闭 P0 风险。
- 为 Dashboard 新增后端聚合接口会扩大后端范围；现有 `listExams`、`listAuditLogs`、`getSystemStatus` 足以构建轻量真实概览。
- legacy `/review` 需要谨慎处理：真实阅卷工作台已经集中在 `/grading`，所以 `/review` 应从生产隐藏并记录替代路径。
- `quality`、`permissions`、`settings` 对应后续 Story，不能在本轮伪实现。

Plan 修改：

- 采用 feature flag 隐藏 mock route，而不是删除页面。
- Dashboard 必须接真实 API，不再允许静态演示数据。
- 生产环境路由过滤必须同时作用于菜单和 `routeFromPath()`。
- 不新增后端 Dashboard 聚合接口。
- 不把 Quality、Permissions、Settings 做成假页面；生产隐藏，后续接真 API 后再开放。

## Implementation

按 TDD 推进，先新增静态检查脚本并确认 RED，再实现功能。

新增：

- `apps/web-admin/scripts/check-production-routes.mjs`
  - 检查 `DashboardPage` 不再导入 `../data`。
  - 检查 Dashboard 使用考试、审计、系统状态 API。
  - 检查路由具备 `productionReady`、`mockRoutesEnabled`、`visibleRoutes`。
  - 检查 `/review`、`/quality`、`/permissions`、`/settings` 显式 `productionReady: false`。
  - 检查 `routeFromPath()` 只从 `visibleRoutes()` 解析。
  - 检查 `AppLayout` 菜单同时使用 `visibleRoutes()` 和 `hasEveryPermission()`。
  - 检查顶栏 MockBadge 只在当前 mock route 显示。

修改：

- `apps/web-admin/package.json`
  - 增加 `check:production-routes`。
- `apps/web-admin/src/router/routes.tsx`
  - `AppRoute` 增加 `productionReady`、`replacementPath`。
  - 新增 `mockRoutesEnabled()`、`isRouteVisible()`、`visibleRoutes()`。
  - `routeFromPath()` 改为只解析当前环境可见 route。
  - `/dashboard` 改为真实生产 route：`mock: false`、`productionReady: true`。
  - `/review`、`/quality`、`/permissions`、`/settings` 标记为 `mock: true`、`productionReady: false`。
  - `/review` 增加 `replacementPath: "/grading"`。
- `apps/web-admin/src/components/AppLayout.tsx`
  - 菜单来源改为 `visibleRoutes()`。
  - 菜单项再按 `hasEveryPermission(user, route.permissions)` 过滤。
  - 顶栏 `MockBadge` 只在当前 route 为 mock 时展示。
- `apps/web-admin/src/pages/DashboardPage.tsx`
  - 删除 `../data`、`MockBadge`、fake charts 的依赖。
  - 使用真实考试、答卷、审计、系统状态 API 聚合首页数据。
  - 考试列表失败时显示真实错误。
  - 审计、系统状态、最近答卷失败时显示真实告警，不回退假数据。
  - 空数据展示真实空状态。
- `apps/web-admin/src/vite-env.d.ts`
  - 增加 `VITE_ENABLE_MOCK_ROUTES` 类型声明。

## Implementation Review

对照验收标准逐项审阅：

- `DashboardPage` 不再导入 `../data`，也不再使用静态 `metrics`、`trend`、`pipeline`、`agentStages`。
- 生产默认路径下，mock route 必须满足 `productionReady: false`，且 `routeFromPath()` 只从 `visibleRoutes()` 中解析。
- 菜单不会展示生产隐藏 route，也不会展示当前用户无权限访问的 route。
- 开发/demo 环境或显式设置 `VITE_ENABLE_MOCK_ROUTES=true` 时，mock route 仍可见，并且当前 mock route 会显示 `MockBadge`。
- `ModulePage` 未删除，但生产默认不可通过隐藏 route 到达，保留给演示/开发环境。
- `lab/` 未接入生产链路，本 Story 不以 `lab/` 作为验收证据。

审阅发现和处理：

- 需要防止只验证菜单、不验证 hash 直达；已通过 `routeFromPath()` 使用 `visibleRoutes()` 关闭。
- Dashboard 同时调用多个 API 时，非核心接口失败不应回退假数据；已改为真实告警和空状态。
- 顶栏全局 `MockBadge` 会误导生产用户；已改为仅当前 mock route 展示。

## Fixes

- 静态检查脚本新增对 `routeFromPath()`、`visibleRoutes()`、权限过滤和条件 `MockBadge` 的约束。
- Dashboard 保留真实空状态和告警，不提供任何 hard-coded 运营数值。
- 路由显式记录 legacy `/review` 的替代路径 `/grading`。

## Verification

RED：

```powershell
npm.cmd --workspace apps/web-admin run check:production-routes
```

结果：失败，失败项覆盖 Dashboard mock 数据、路由生产可见性、隐藏 route、菜单权限过滤和 MockBadge 条件展示。

GREEN：

```powershell
npm.cmd --workspace apps/web-admin run check:production-routes
```

结果：通过，输出 `Production route checks passed.`。

TypeScript：

```powershell
npm.cmd --workspace apps/web-admin run typecheck
```

结果：通过，`tsc --noEmit` 退出码为 0。

Workspace TypeScript：

```powershell
npm.cmd run typecheck
```

结果：通过，desktop-client 和 web-admin workspace 均通过 `tsc --noEmit`。

Production build：

```powershell
npm.cmd --workspace apps/web-admin run build
```

结果：通过，Vite production build 完成。构建输出提示主 JS chunk 超过 500 kB，这是后续性能优化风险，不阻塞本 Story 的生产菜单收敛目标。

备注：npm 输出 `Unknown env config "store-dir"` 警告，这是本机 npm 配置兼容性提示，不影响本 Story 的 Web 检查和类型编译结果。

## Approval

STORY-048 批准通过。生产默认 Web 菜单已退场 mock 页面，Dashboard 已切换为真实 API 聚合页，演示/开发 mock route 仍保留显式开关与标识。

后续进入 STORY-049 前，仍需明确：OCR、主观题 AI、Agent worker、Quality、Permissions、Settings 尚未因本 Story 获得生产能力。
