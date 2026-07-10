# STORY-023 自审审批记录

## Story

STORY-023 Web 管理后台基础框架

## 审批结论

Approved

## 验收项

| 验收项 | 证据 | 结论 |
| --- | --- | --- |
| React + TypeScript + Vite 可构建 | `npm.cmd run typecheck`、`npm.cmd run build` | 通过 |
| 登录页存在 | Playwright 登录页快照 | 通过 |
| 主布局完整 | 左侧导航、顶部栏、面包屑、通知、任务队列、用户菜单 | 通过 |
| 路由系统存在 | hash routes + navigation | 通过 |
| 权限路由存在 | route permission metadata + 403 状态 | 通过 |
| API Client 存在 | `apps/web-admin/src/api/client.ts` | 通过 |
| 统一错误处理存在 | `ApiError`、`parseApiError`、Error state | 通过 |
| Loading/Empty/Error 状态存在 | `PageState.tsx` | 通过 |
| 表格组件封装存在 | `DataTable.tsx` | 通过 |
| 表单组件封装存在 | `FormShell.tsx` | 通过 |
| 设计 token 存在 | CSS variables + Ant Design token | 通过 |
| 菜单覆盖 14 个指定模块 | `routes.tsx`、Playwright DOM 快照 | 通过 |
| mock 页面明确标注 | `MockBadge`、mock session 文案 | 通过 |
| 不冒充真实业务能力 | 仅框架页和 mock 占位页 | 通过 |
| 桌面/移动视口无明显布局重叠 | Playwright screenshots | 通过 |

## 运行命令与结果

```powershell
npm.cmd install --workspace apps/web-admin
npm.cmd run typecheck
npm.cmd run build
npm.cmd run dev -- --host 127.0.0.1 --port 5173
```

```text
npm install --workspace apps/web-admin -> passed, 0 vulnerabilities
npm run typecheck -> passed
npm run build -> passed
Playwright desktop 1440x900 -> passed
Playwright mobile 390x844 -> passed
Console after fixes -> 0 errors, 0 warnings
```

## 修复记录

- 补齐 React 类型依赖。
- 补齐 favicon，消除浏览器 404。
- 修正登录输入 autocomplete。
- 修正 AntD Table rowKey 废弃参数警告。
- 修正移动端 collapsed 侧栏品牌文字越界。
- 修正移动端表格撑开整页的问题，改为表格内部横向滚动。

## 剩余风险

- 当前登录和模块页仍是明确标注的 mock，仅用于框架阶段。
- 真实考试管理、试卷管理、答卷采集等页面需要后续 Story 对接真实 API。
- Vite 构建提示首包较大，后续页面增多后需要按路由拆包。

## 下一步

进入 `STORY-024 Web 考试管理页面`。
