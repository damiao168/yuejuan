# STORY-A02：Design System 与考试生命周期工作区

状态：Implemented（当前工作树）

## 范围

- 考试工作区按“准备、采集、处理、阅卷、结果”组织阶段，而非暴露内部服务名。
- `packages/design-tokens` 提供颜色、间距、排版、圆角、阴影、动效、断点、层级和评分语义 token；`packages/ui` 提供工作区布局、指标、阶段轨道与通知组件。
- 管理端考试上下文从真实 `workspace` 投影读取当前阶段、下一步和阻断项。

## 接线

- `apps/web-admin/src/features/exams/workspace/ExamWorkspaceLayout.tsx` 组合 `WorkspaceLayout`、`WorkspaceStageRail`、指标和按阻断/警告分组的通知；页面文件仅保留 Query 边界。
- `services/api-gateway/internal/workspace` 提供 `GET /api/v1/exams/{examId}/workspace`。
- 路由仍受既有考试范围与角色授权保护；UI 不替代服务端权限判断。

## 验证

- Workspace 领域定向 Go 测试、Web 类型检查、生产构建和生产路由门禁已执行；浏览器 Mock 流程覆盖五阶段导航、阻断跳转与阅卷员深链越权。

## 外部边界

- 阶段文案和优先级基于现有业务投影；学校实际考试 SOP 与管理员可理解性仍需现场验证。
