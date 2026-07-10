# STORY-036 UI 设计规范文档

## Plan

### 目标

编写 EduGrade Enterprise 的企业级 UI 设计规范，保存到 `docs/ui-spec/enterprise-ui-guidelines.md`。文档必须可直接指导 Web 管理后台、阅卷工作台和 Windows EXE 客户端后续实现，不写泛泛理念，给出具体组件、页面、状态、布局、响应式和禁止事项规范。

### 范围

- 覆盖原始需求列出的 24 个主题：
  - 设计原则。
  - 产品布局。
  - 导航结构。
  - 页面状态规范。
  - 颜色、字体、间距。
  - 表格、表单、按钮、标签、状态色。
  - 阅卷工作台、答卷查看器、AI 证据、Rubric 面板。
  - 快捷键。
  - 成绩报告、申诉、审计页面。
  - EXE 客户端规范。
  - 可访问性、响应式、禁止事项。
- 使用项目已有 token：
  - `#1677FF`
  - `#52C41A`
  - `#FAAD14`
  - `#FF4D4F`
  - `#13C2C2`
  - `#F5F7FA`
  - `#FFFFFF`
  - `#1F2329`
  - `#646A73`
  - `#E5E6EB`
- 结合当前已实现页面：
  - Web 管理后台真实 API 页面。
  - Web 阅卷工作台。
  - Windows EXE 扫描工作站与离线阅卷基础。
- 更新 Story 索引与审批记录。

### 非范围

- 不重写已实现前端页面。
- 不新增设计系统代码、组件库 package 或 Figma 文件。
- 不生成图片资产。
- 不替代后续专项 Story 的页面实现验收。

### 修改文件

预计新增：

- `docs/ui-spec/enterprise-ui-guidelines.md`
- `docs/stories/STORY-036-approval.md`

预计修改：

- `docs/stories/README.md`
- `docs/stories/STORY-036-ui-design-guidelines.md`

### 验收标准

- 文档存在于 `docs/ui-spec/enterprise-ui-guidelines.md`。
- 明确覆盖 24 个需求主题。
- 包含具体 token、尺寸、密度、状态、组件行为和页面结构规范。
- 明确 Web 管理后台、阅卷工作台、Windows EXE 的不同布局原则。
- 页面状态包含 loading、empty、error、success、warning。
- 表格、表单、按钮、标签、状态色规范可直接落地。
- 阅卷工作台、答卷查看器、AI 证据、Rubric 评分面板有具体布局与交互要求。
- 成绩报告、申诉、审计页面有页面级规范。
- EXE 客户端包含扫描、离线阅卷、同步、诊断、安全存储/未接入标识规范。
- 可访问性、响应式和禁止事项明确。
- Story 文档包含 Plan、Plan Review、Implementation、Implementation Review、Fixes、Approval。

## Plan Review

### 是否越界

未越界。本 Story 只交付 UI 设计规范文档，不改动产品代码、不新增设计系统实现、不引入新视觉资产。

### 是否遗漏显式需求

未遗漏。计划逐条覆盖原始需求 24 个主题和视觉 token，并要求文档给出具体组件与页面规范。

### 是否符合当前仓库实际

符合。当前仓库已有 `docs/ui-spec/ui-principles.md` 作为骨架原则，Web 管理后台和 EXE 客户端已形成实际界面模式。本 Story 在新文件中沉淀完整规范，不破坏既有原则文档。

## Implementation

- 新增 `docs/ui-spec/enterprise-ui-guidelines.md`。
- 文档按企业级前端实现所需内容组织：
  - 设计原则。
  - 颜色规范与视觉 token。
  - 字体与间距。
  - Web 管理后台、阅卷工作台、Windows EXE 布局。
  - 导航结构。
  - 页面状态。
  - 表格、表单、按钮、标签和状态色。
  - 阅卷工作台、答卷查看器、AI 证据、Rubric 面板、快捷键。
  - 成绩报告、申诉、审计、EXE 客户端专项规范。
  - 可访问性、响应式、禁止事项和前端验收清单。
- 更新 Story 索引。

## Implementation Review

### 验收检查

| 验收项 | 证据 | 结论 |
| --- | --- | --- |
| 文档保存路径正确 | `docs/ui-spec/enterprise-ui-guidelines.md` | 通过 |
| 覆盖 24 个需求主题 | 文档 1-24 节 | 通过 |
| 包含视觉 token | 颜色规范与视觉 Token 表格 | 通过 |
| 可指导前端实现 | 含数值、布局、状态、组件行为、验收清单 | 通过 |
| Web 管理后台布局 | 第 5、6 节 | 通过 |
| 阅卷工作台布局 | 第 5、12 节 | 通过 |
| Windows EXE 规范 | 第 5、20 节 | 通过 |
| loading/empty/error/success/warning | 第 7 节 | 通过 |
| 表格/表单/按钮/标签 | 第 8-11 节 | 通过 |
| 答卷/AI/Rubric/快捷键 | 第 13-16 节 | 通过 |
| 报告/申诉/审计 | 第 17-19 节 | 通过 |
| 可访问性/响应式/禁止事项 | 第 21-23 节 | 通过 |

### 发现的问题

- 初稿标题使用“视觉 Token”，与原始需求中的“颜色规范”显式词不完全对齐。

## Fixes

- 将第 2 节标题修正为“颜色规范与视觉 Token”，更直接覆盖原始显式需求。

## Approval

### 审批结论

Approved

### 运行命令与结果

```powershell
Test-Path .\docs\ui-spec\enterprise-ui-guidelines.md
Get-Content .\docs\ui-spec\enterprise-ui-guidelines.md -Encoding UTF8 | Measure-Object -Line
Select-String -Path .\docs\ui-spec\enterprise-ui-guidelines.md -Pattern ...
```

```text
文档存在 -> True
文档行数 -> 381
关键词覆盖检查 -> 覆盖设计原则、Web 管理后台、阅卷工作台、Windows EXE、loading/empty/error/success/warning、颜色、字体、间距、表格、表单、按钮、标签、状态色、答卷查看器、AI 证据、Rubric、快捷键、成绩报告、申诉、审计、EXE 客户端、可访问性、响应式、禁止事项
```

### 新增文件

- `docs/ui-spec/enterprise-ui-guidelines.md`
- `docs/stories/STORY-036-approval.md`

### 修改文件

- `docs/stories/README.md`
- `docs/stories/STORY-036-ui-design-guidelines.md`

### 剩余风险

- 本 Story 只交付规范文档，不强制重构既有页面；后续页面 Story 需要按本文档做实现验收。

### 下一步

进入 `STORY-037 私有化部署 Docker Compose`。
