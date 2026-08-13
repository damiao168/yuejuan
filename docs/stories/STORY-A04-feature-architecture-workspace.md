# STORY-A04：前端 Feature Architecture 与 Exam Workspace 重构

状态：Implemented（当前工作树）

## 范围

- 管理端使用路由壳、Feature 目录和 Query 层承载考试工作区及核心阅卷状态。
- 路由注册与页面组合分离，避免把实际权限/生产页面判断留在单一巨大 `App.tsx`。
- 考试、阅卷、质量、采集等高频界面优先迁移；不进行无收益的全仓重写。

## 接线

- `apps/web-admin/src/AppShell.tsx`、`router/`、`query/` 与 `features/` 承担生产入口。
- `ExamWorkspacePage` 只消费 Query 投影和加载/错误边界；`features/exams/workspace/ExamWorkspaceLayout.tsx` 承担工作区组合，阅卷工作站由独立 Feature 组合。
- 生产路由检查针对实际 `AppShell` 与权限条件，不把开发 Mock 注册到生产路径。

## 验证

- Web Vitest、TypeScript、生产构建、`check:production-routes` 及 STORY-053 门禁已执行；浏览器覆盖兼容深链、工作区阶段跳转和角色权限差异。

## 外部边界

- 已迁移核心路径不等于所有历史页面均完成同等重构；后续迁移须保持接口和权限边界，不得为目录形式重复开发。
