# STORY-036 自审审批记录

## Story

STORY-036 UI 设计规范文档

## 审批结论

Approved

## 验收项

| 验收项 | 证据 | 结论 |
| --- | --- | --- |
| 指定文档存在 | `docs/ui-spec/enterprise-ui-guidelines.md` | 通过 |
| 覆盖 24 个主题 | 文档 1-24 节 | 通过 |
| 视觉 token 完整 | 颜色规范与视觉 Token | 通过 |
| 具体组件规范 | 表格、表单、按钮、标签、状态色 | 通过 |
| 页面专项规范 | 阅卷、报告、申诉、审计、EXE | 通过 |
| 响应式和可访问性 | 第 21-22 节 | 通过 |
| 禁止事项 | 第 23 节 | 通过 |
| 可直接指导实现 | 数值、布局、行为、验收清单 | 通过 |

## 运行命令与结果

```powershell
Test-Path .\docs\ui-spec\enterprise-ui-guidelines.md
Get-Content .\docs\ui-spec\enterprise-ui-guidelines.md -Encoding UTF8 | Measure-Object -Line
Select-String -Path .\docs\ui-spec\enterprise-ui-guidelines.md -Pattern ...
```

```text
文档存在 -> True
文档行数 -> 381
关键词覆盖检查 -> 通过
```

## 剩余风险

- 本 Story 是规范文档，不包含前端重构。

## 下一步

进入 `STORY-037 私有化部署 Docker Compose`。
