# STORY-024 Web 考试管理页面

## 状态

Approved

## 目标

实现 Web 管理后台的考试管理页面，对接现有真实后端 API：

- `GET /api/v1/exams`
- `GET /api/v1/exams/{id}`
- `POST /api/v1/exams`
- `PATCH /api/v1/exams/{id}`
- `POST /api/v1/exams/{id}/status`
- `POST /api/v1/exams/{id}/archive`
- `GET /api/v1/schools`
- `GET /api/v1/grades`
- `GET /api/v1/classes`

页面包括考试列表、搜索筛选、新建/编辑考试表单、考试详情视图、状态流转和归档操作。所有数据必须来自真实 API 响应；后端尚未返回的字段不得伪造。

## Plan

- API 层：
  - 新增 `apps/web-admin/src/api/exams.ts`，封装考试列表、详情、创建、更新、状态流转和归档。
  - 新增 `apps/web-admin/src/api/org.ts`，封装学校、年级、班级列表。
  - 类型定义与后端 `Exam`、`School`、`Grade`、`Class` 字段保持一致。
- 路由：
  - `/exams` 从 mock 占位页切换为真实考试管理页面。
  - `routes.tsx` 中考试管理路由标记为非 mock。
- 列表页：
  - 搜索：按考试名称、学科、创建人本地过滤。
  - 筛选：学科、年级、状态、考试类型。
  - 状态筛选和学校筛选使用后端查询参数；学科、年级、考试类型基于真实返回数据做本地过滤。
  - 表格字段覆盖：考试名称、学科、年级、班级数量、总分、状态、阅卷模式、创建人、创建时间、操作。
  - `年级` 通过 `class_ids -> class.grade_id -> grade.name` 从真实组织数据推导。
  - `班级数量` 使用真实 `class_ids.length`。
  - `创建时间` 当前考试 API 未返回，显示为“后端未返回”，不伪造时间。
- 表单：
  - 使用 Ant Design Drawer/Form 实现新建/编辑。
  - 字段：基础信息、学校、班级选择、阅卷模式、是否允许申诉、成绩发布策略。
  - 班级选择来自真实 `GET /api/v1/classes`，按年级分组展示。
  - 完整校验：名称、学校、学科、考试类型、总分、阅卷模式、发布策略、至少一个班级。
  - 编辑 published/archived 状态考试时禁用保存。
- 详情：
  - 显示基础信息。
  - 显示试卷配置状态、答卷采集状态、阅卷进度、成绩发布状态为“待后续 API 支持”，明确不伪造。
  - 提供审计记录入口到 `/audit`，不实现审计查询。
- 状态和权限：
  - loading、error、empty 状态完整。
  - 操作按钮根据 `exam:manage` 权限、考试状态和状态机禁用或隐藏。
  - 状态标签颜色复用 `StatusTag`。
- 验证：
  - `npm.cmd run typecheck`
  - `npm.cmd run build`
  - 浏览器检查 `/exams` 在无后端/无 token 时显示真实错误状态，不显示 mock 数据。
  - 浏览器检查桌面/移动视口无明显布局重叠和整页横向滚动。

## Plan Review

- 不越界：
  - 不实现后端未提供的创建时间、试卷配置进度、答卷采集状态、阅卷进度和成绩发布详情查询。
  - 不实现考试相关图表，不实现批量导入，不实现审计日志查询页面。
  - 不改后端考试 API 合约，除非实现中发现前端无法满足真实对接的阻塞问题。
- 不造假：
  - 不再使用 `src/data.ts` 的 mock 考试数据作为考试管理页数据源。
  - 缺失字段以明确的“后端未返回/待后续 API 支持”展示。
  - API 错误、401/403、网络不可用都显示错误状态，不回退到 mock 数据。
- 与当前项目一致：
  - 沿用 `ApiClient`、`AppLayout`、`StatusTag`、`PageState`、设计 token 和 Ant Design。
  - 保持 hash 路由，不引入新路由库。
  - 使用现有后端真实 API 路径和权限模型。
- 风险：
  - 当前登录仍是 STORY-023 的 mock session，不会生成真实后端 token；因此本页面在未配置 `edugrade.access_token` 时会显示真实认证/连接错误。
  - 考试 API 不支持 subject、exam_type、grade 服务端过滤，第一版使用真实列表数据本地过滤，并在文档中记录限制。

## 非范围

- 真实登录/OIDC/SAML。
- 考试创建时间后端字段改造。
- 试卷配置页面。
- 答卷采集页面。
- 阅卷进度实时查询。
- 成绩发布详情。
- 审计日志查询。
- E2E 测试文件。

## 验收标准

- `/exams` 使用真实 API Client 调用考试和组织接口。
- 页面包含考试列表、搜索、学科/年级/状态/考试类型筛选和新建考试按钮。
- 表格包含要求字段，并对后端未返回字段明确标注。
- 新建/编辑表单包含基础信息、班级选择、阅卷模式、申诉开关和成绩发布策略。
- 表单校验完整。
- 考试详情显示基础信息、配置状态占位、采集状态占位、阅卷进度占位、成绩发布状态占位和审计入口。
- 有 loading、error、empty 状态。
- 状态标签颜色统一。
- 无权限或状态不允许的操作隐藏或禁用。
- `npm.cmd run typecheck` 通过。
- `npm.cmd run build` 通过。
- 桌面/移动视口无明显布局重叠和整页横向滚动。

## Implementation

新增真实 API 封装：

- `apps/web-admin/src/api/exams.ts`
  - `listExams`
  - `getExam`
  - `createExam`
  - `updateExam`
  - `updateExamStatus`
  - `archiveExam`
  - 与后端 `Exam`、`CreateInput`、`UpdateInput` 字段保持一致。
- `apps/web-admin/src/api/org.ts`
  - `listSchools`
  - `listGrades`
  - `listClasses`
  - 用于学校、年级和班级选择，不伪造组织数据。

新增考试管理页面：

- `apps/web-admin/src/pages/ExamManagementPage.tsx`
  - 考试列表和筛选。
  - 新建/编辑 Drawer 表单。
  - 详情 Drawer。
  - 状态推进和归档操作。
  - 无 token 时停止请求并显示明确错误，不回退 mock。
  - 有 token 时只调用真实 API。
  - 创建时间、试卷配置状态、答卷采集状态、阅卷进度和成绩发布状态在后端缺字段/缺接口时明确显示为后端未返回或待后续 API 支持。

路由和样式：

- `/exams` 从 mock 占位页切换为真实考试管理页。
- `routes.tsx` 中考试管理 `mock=false`。
- `styles.css` 增加筛选栏、详情状态格、Drawer 提示和响应式样式。

## Implementation Review

验收检查：

| 验收项 | 证据 | 结论 |
| --- | --- | --- |
| `/exams` 使用真实 API Client | `api/exams.ts`、`api/org.ts`、`ExamManagementPage.tsx` | 通过 |
| 考试列表、搜索、筛选、新建按钮 | 页面快照和代码 | 通过 |
| 表格字段覆盖要求 | columns 包含名称、学科、年级、班级数量、总分、状态、模式、创建人、创建时间、操作 | 通过 |
| 后端未返回字段明确标注 | 创建时间显示“后端未返回” | 通过 |
| 表单字段覆盖要求 | Drawer Form 包含基础信息、班级、阅卷模式、申诉、发布策略 | 通过 |
| 表单校验完整 | required + class_ids array min 校验 | 通过 |
| 详情页信息覆盖要求 | 基础信息、配置/采集/阅卷/发布占位、审计入口 | 通过 |
| loading/error/empty 状态 | LoadingState、ErrorState、Table emptyText | 通过 |
| 状态标签颜色统一 | `StatusTag` + `statusTone` | 通过 |
| 无权限/无 token 操作禁用 | `canWrite = canManage && hasToken` | 通过 |
| 不使用 mock 考试数据 | 页面不导入 `src/data.ts` | 通过 |
| typecheck/build 通过 | `npm.cmd run typecheck`、`npm.cmd run build` | 通过 |
| 桌面/移动视口稳定 | Playwright scrollWidth 检查和截图 | 通过 |

审阅发现的问题：

- 无 token 时仍发起真实请求，导致本地无后端时控制台出现连接失败噪声。
- 无 token 时“新建考试”仍可点击。
- 发布状态下“推进到已归档”和“归档”操作可能重复。
- 班级多选需要明确校验至少 1 个班级。

## Fixes

- 增加 token 前置检查：未配置 `edugrade.access_token` 时不请求后端，直接显示错误状态。
- 无 token 时禁用新建、编辑、状态推进和归档等写操作。
- 隐藏 `nextStatus=archived` 的推进按钮，只保留归档操作。
- 班级选择增加 `type=array` 和 `min=1` 校验。

## Verification

运行命令：

```powershell
npm.cmd run typecheck
npm.cmd run build
```

结果：

```text
npm run typecheck -> passed
npm run build -> passed
```

浏览器验收：

```text
Playwright /#/exams after mock shell login -> passed
No real token state -> shows explicit error and disabled write actions
No mock exam data rendered -> passed
Desktop console -> 0 errors, 0 warnings
Mobile 390x844 scrollWidth -> document 390px at 390px viewport
Mobile screenshot -> no obvious overlap
```

限制记录：

- 当前 Story 不实现真实登录；真实后端 token 需由后续认证页面或人工写入 `localStorage.edugrade.access_token`。
- 后端考试 API 暂不返回 `created_at` 和年级字段；页面只显示真实字段或明确缺失状态。
- 试卷配置、答卷采集、阅卷进度和成绩发布详情留给后续 Story/API 页面。
- Vite 仍提示首包超过 500 kB，属于基础依赖体积问题，后续适合做路由级拆包。

## Approval

Approved。

本 Story 满足验收标准，可以进入 `STORY-025 Web 试卷与 Rubric 配置页面`。
