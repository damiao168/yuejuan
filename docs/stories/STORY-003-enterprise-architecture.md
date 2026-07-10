# STORY-003 企业级技术架构文档

## 状态

Approved

## 自审审批

审批文件：`docs/stories/STORY-003-approval.md`

## 目标

编写 EduGrade Enterprise 的企业级技术架构文档，保存到 `docs/architecture/enterprise-architecture.md`。

## 范围

必须覆盖：

- 总体架构图，用文本图表示。
- Web 管理后台架构。
- Windows EXE 客户端架构。
- 后端服务架构。
- AI Agent 服务架构。
- OCR 服务架构。
- 数据存储架构。
- 对象存储设计。
- 缓存与任务队列设计。
- 多租户隔离设计。
- 权限系统设计。
- 审计日志设计。
- 模型服务抽象。
- 私有化部署架构。
- 本地离线阅卷架构。
- 文件上传与断点续传架构。
- 成绩发布与申诉流程架构。
- 可观测性。
- 备份恢复。
- 灾备预案。
- 安全边界。

## 非范围

- 不实现代码。
- 不创建数据库 migration。
- 不写具体 API 文档。
- 不实现 Web 或 EXE 页面。

## 验收标准

- 架构能支撑真实学校/机构使用。
- 说明每个模块职责。
- 明确第一版必须实现和可预留接口。
- 明确服务通信方式。
- 明确 PostgreSQL、Redis、MinIO、Qdrant 的使用边界。
