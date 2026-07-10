# STORY-024 自审审批记录

## Story

STORY-024 Web 考试管理页面

## 审批结论

Approved

## 验收项

| 验收项 | 证据 | 结论 |
| --- | --- | --- |
| 对接真实后端 API | `apps/web-admin/src/api/exams.ts`、`apps/web-admin/src/api/org.ts` | 通过 |
| 考试列表页 | `ExamManagementPage.tsx` | 通过 |
| 搜索与筛选 | 搜索、学校、学科、年级、状态、考试类型控件 | 通过 |
| 表格字段覆盖 | columns 覆盖要求字段 | 通过 |
| 新建/编辑考试表单 | Drawer Form | 通过 |
| 选择班级 | 真实 `GET /api/v1/classes` 数据源 | 通过 |
| 阅卷模式、申诉、发布策略 | 表单字段和校验 | 通过 |
| 考试详情页 | Detail Drawer | 通过 |
| 试卷/采集/阅卷/发布状态 | 明确待后续 API 支持，不伪造 | 通过 |
| loading/error/empty 状态 | PageState + Table empty state | 通过 |
| 状态标签颜色统一 | `StatusTag` | 通过 |
| 无权限或无 token 操作禁用 | `canWrite = canManage && hasToken` | 通过 |
| 不显示 mock 考试数据 | 页面不导入 `src/data.ts` | 通过 |
| 类型检查 | `npm.cmd run typecheck` | 通过 |
| 构建 | `npm.cmd run build` | 通过 |
| 桌面/移动布局 | Playwright screenshot + scrollWidth check | 通过 |

## 运行命令与结果

```powershell
npm.cmd run typecheck
npm.cmd run build
```

```text
npm run typecheck -> passed
npm run build -> passed
```

## 浏览器检查

```text
/#/exams after shell login -> passed
No token state -> explicit error, write actions disabled
Console after no-token check -> 0 errors, 0 warnings
Mobile 390x844 -> no page horizontal scroll
```

## 剩余风险

- 真实登录/token 获取仍不属于本 Story，考试页面依赖 `edugrade.access_token`。
- 后端考试 API 尚未返回 `created_at`，页面明确显示“后端未返回”。
- 试卷配置状态、答卷采集状态、阅卷进度和成绩发布状态需要后续页面/API 继续细化。
- 构建包体仍有 Vite chunk size warning，后续适合做路由级拆包。

## 下一步

进入 `STORY-025 Web 试卷与 Rubric 配置页面`。
