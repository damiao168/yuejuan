# STORY-023 Web 管理后台基础框架

## 状态

Approved

## 视觉与交互方向

- Visual thesis：克制、密集、可信的教育评测运营工作台，使用清晰信息层级、低噪声色彩和稳定布局，适合长时间阅卷/教务使用。
- Content plan：登录页、主工作台、侧边导航、顶部上下文、状态/表格/表单基础组件、模块占位页。
- Interaction thesis：侧边导航切换保持空间稳定；工作区内容轻量进入；按钮、菜单、表格行用小幅反馈强化可操作性。

## 目标

实现 EduGrade Enterprise Web 管理后台基础框架：React + TypeScript + Vite 的可构建前端入口、登录页、主布局、路由系统、权限路由、API Client、统一错误处理、Loading/Empty 状态、表格组件封装、表单组件封装、设计 token，以及覆盖后续模块的导航骨架。

本 Story 允许使用明显标注的 mock 页面和 mock session 来搭建框架，但不实现复杂业务页面，不冒充真实后端数据；后续具体页面按 `STORY-024` 之后逐步对接真实 API。

## Plan

- 修复当前 Web 工程入口：
  - 新增 `src/App.tsx`。
  - 新增 `src/styles.css`。
  - 保持 `main.tsx` 入口可构建。
- 建立基础前端架构：
  - `api/client.ts`：统一 API Client，支持 base URL、token、JSON 错误结构。
  - `auth/session.ts`：mock session 明确标记，用于框架阶段权限路由。
  - `router/routes.ts`：路由定义、菜单定义、权限元数据。
  - `components/`：基础布局、状态组件、数据表格、表单控件、状态标签。
  - `pages/`：登录页、Dashboard、模块占位页、无权限页、404。
- 登录页：
  - 提供租户、用户名、密码输入。
  - 显示“框架阶段 mock session”标识。
  - 不声称已经接通真实登录；保留 API Client 对接入口。
- 主布局：
  - 左侧导航。
  - 顶部栏：租户/学校、当前考试、全局搜索、通知入口、任务队列状态、用户菜单。
  - 面包屑。
  - 稳定响应式布局。
- 菜单覆盖：
  - 首页。
  - 考试管理。
  - 试卷管理。
  - 答卷采集。
  - 智能阅卷。
  - 人工复核。
  - 双评仲裁。
  - 成绩管理。
  - 学情报告。
  - 申诉中心。
  - 质量控制。
  - 用户权限。
  - 系统设置。
  - 审计日志。
- 权限路由：
  - 路由配置声明所需权限。
  - 当前 mock session 中没有权限时显示无权限页。
  - 真实权限数据接入留给后续真实登录/用户权限页面。
- 状态与组件：
  - Loading 状态。
  - Empty 状态。
  - Error 状态。
  - 表格基础封装：搜索、筛选、密度、列说明、空状态。
  - 表单基础封装：字段、校验提示、提交/重置。
  - 统一状态标签颜色。
- 设计 token：
  - 色彩、字体、间距、圆角、阴影、表格密度和状态色。
  - 避免一色系和营销式 hero。
- 文档：
  - 更新 README 和 Web Admin README。
  - 更新 Story 进度。
- 验证：
  - 安装/确认前端依赖。
  - `npm.cmd run typecheck`。
  - `npm.cmd run build`。
  - 启动本地 dev server。
  - 使用浏览器截图/页面检查确认登录页和主框架非空、无明显重叠。

## Plan Review

- 不越界：不实现考试管理真实 CRUD、不实现试卷配置、不实现阅卷工作台细节、不实现 Web 报告图表、不实现 EXE。
- mock 合规：本 Story 明确允许框架阶段 mock session 和 mock 页面，但必须在 UI 中标注为 mock，不冒充真实后端数据。
- 与当前仓库实际一致：当前 `App.tsx` 和 `styles.css` 缺失，优先补齐可构建入口；已有 `data.ts` 可用于 mock 框架页面展示。
- 与后续 Story 衔接：`STORY-024` 起逐个页面对接真实 API；本 Story 只提供路由、布局、组件和 API Client 基础。
- 设计符合产品：后台是运营工作台，不做营销 landing hero；采用高信息密度、稳定导航和克制视觉。
- 风险控制：如果依赖未安装，先安装前端依赖；如果浏览器工具不可用，至少完成 typecheck/build 并记录限制。

## 非范围

- 真实登录流程。
- 真实权限管理页面。
- 真实考试 CRUD。
- 真实文件上传 UI。
- 真实阅卷工作台。
- Web 报告图表页。
- Web 申诉中心细节。
- 审计日志真实查询页。
- E2E 测试。

## 验收标准

- Web Admin 可以 typecheck。
- Web Admin 可以 production build。
- `App.tsx` 和 `styles.css` 存在并可运行。
- 有登录页。
- 有主布局：左侧导航、顶部栏、面包屑、用户菜单、通知入口。
- 有路由系统。
- 有权限路由和无权限页。
- 有 API Client。
- 有统一错误展示。
- 有 Loading、Empty、Error 状态组件。
- 有表格组件封装。
- 有表单组件封装。
- 有设计 token。
- 菜单覆盖 14 个指定模块。
- 所有 mock 页面明显标注 mock。
- UI 在桌面和移动视口无明显文字溢出或重叠。
- 不实现复杂业务，不假装真实数据。

## Implementation

新增和完善 Web Admin 基础框架：

- `apps/web-admin/src/App.tsx`：应用状态、hash 路由、登录态切换、权限路由和 404/403 分支。
- `apps/web-admin/src/components/AppLayout.tsx`：Ant Design 主布局，包含左侧导航、顶部栏、面包屑、通知入口、任务队列入口和用户菜单。
- `apps/web-admin/src/router/routes.tsx`：14 个指定后台模块的路由、菜单分组、图标和权限元数据。
- `apps/web-admin/src/auth/session.ts`：明确标注为框架阶段 mock session，不冒充真实认证。
- `apps/web-admin/src/api/client.ts`：统一 API Client、base URL、token 注入、JSON 错误解析和统一异常结构。
- `apps/web-admin/src/components/PageState.tsx`：Loading、Empty、Error、Forbidden、NotFound 状态。
- `apps/web-admin/src/components/DataTable.tsx`：表格封装，包含搜索、筛选、空状态、稳定 row key 和窄屏内部横向滚动。
- `apps/web-admin/src/components/FormShell.tsx`：表单封装，提供统一标题、说明、提交和重置动作。
- `apps/web-admin/src/pages/LoginPage.tsx`：登录页，清晰展示“框架阶段 MOCK”，并补充浏览器 autocomplete。
- `apps/web-admin/src/pages/DashboardPage.tsx`：后台首页工作台，使用 mock 数据展示队列、质量趋势、Agent 状态和表格。
- `apps/web-admin/src/pages/ModulePage.tsx`：后续模块占位页，全部标注 mock，不实现复杂业务。
- `apps/web-admin/src/styles.css`：设计 token、后台工作台视觉系统、桌面/移动响应式布局和表格移动端约束。
- `apps/web-admin/public/favicon.svg` 与 `index.html`：补齐应用 favicon，避免浏览器 404 噪声。

依赖更新：

- `apps/web-admin/package.json` 增加 `antd`、`@types/react`、`@types/react-dom`。
- `package-lock.json` 更新依赖锁定。

文档更新：

- 根 `README.md` 更新当前 Web Admin 状态和构建命令。
- `apps/web-admin/README.md` 新增 Web Admin 开发说明。
- `docs/stories/README.md` 更新 Story 状态。

## Implementation Review

验收检查：

| 验收项 | 证据 | 结论 |
| --- | --- | --- |
| Web Admin 可以 typecheck | `npm.cmd run typecheck` | 通过 |
| Web Admin 可以 production build | `npm.cmd run build` | 通过 |
| 入口文件存在并可运行 | `src/App.tsx`、`src/styles.css`、Vite dev server | 通过 |
| 有登录页 | Playwright 登录页快照 | 通过 |
| 有主布局 | 左侧导航、顶部栏、面包屑、通知入口、用户菜单 | 通过 |
| 有路由系统 | hash route 配置和导航点击 | 通过 |
| 有权限路由和无权限页 | route permission + mock session check | 通过 |
| 有 API Client | `src/api/client.ts` | 通过 |
| 有统一错误结构 | `ApiError`、`parseApiError` | 通过 |
| 有 Loading/Empty/Error 状态 | `PageState.tsx` | 通过 |
| 有表格组件封装 | `DataTable.tsx` | 通过 |
| 有表单组件封装 | `FormShell.tsx` | 通过 |
| 有设计 token | CSS variables + Ant Design token | 通过 |
| 菜单覆盖 14 个指定模块 | `routes.tsx` + Playwright 快照 | 通过 |
| mock 页面显式标注 | `MockBadge` 和页面文案 | 通过 |
| 不实现复杂业务 | 仅基础框架和 mock 占位页 | 通过 |
| 桌面/移动无明显重叠或整页横向滚动 | Playwright screenshot + scrollWidth 检查 | 通过 |

审阅发现的问题：

- 初始依赖缺少 React 类型声明，导致 TypeScript 无法完整检查。
- 浏览器请求缺失 favicon，控制台出现 404。
- 登录密码输入缺少 autocomplete。
- `DataTable` 最初使用 AntD 已废弃的 `rowKey(record, index)` 参数。
- 移动端侧栏 collapsed 后品牌文字露出，影响窄屏布局。
- 移动端表格内容撑开整页，导致文档横向滚动。

## Fixes

- 增加 `@types/react` 和 `@types/react-dom`。
- 增加 `public/favicon.svg` 并在 `index.html` 声明。
- 为登录账号和密码输入补充 autocomplete。
- `DataTable` 改为稳定 row key，不再依赖行号。
- collapsed 侧栏隐藏品牌文案，只保留品牌标识。
- 表格启用内部横向滚动，布局容器补充 `min-width: 0`，确保移动端整页宽度不被表格撑开。

## Verification

运行命令：

```powershell
npm.cmd install --workspace apps/web-admin
npm.cmd run typecheck
npm.cmd run build
npm.cmd run dev -- --host 127.0.0.1 --port 5173
```

结果：

```text
npm install --workspace apps/web-admin -> passed, 0 vulnerabilities
npm run typecheck -> passed
npm run build -> passed
Vite dev server -> http://127.0.0.1:5173/
```

浏览器验收：

```text
Playwright login page snapshot -> passed
Playwright click login to /#/dashboard -> passed
Desktop 1440x900 screenshot -> passed
Mobile 390x844 screenshot -> passed
Console errors/warnings after fixes -> 0 errors, 0 warnings
Mobile scrollWidth check -> document 390px at 390px viewport, tables scroll internally
```

构建备注：

- Vite 提示首包超过 500 kB，主要来自 Ant Design/Recharts 等基础依赖。当前 Story 只要求基础框架可构建；路由级 code splitting 留给后续页面增长时处理。

## Approval

Approved。

本 Story 满足验收标准，可以进入 `STORY-024 Web 考试管理页面`。
