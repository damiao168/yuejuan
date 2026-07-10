# EduGrade Enterprise

企业级多智能体智能阅卷与学情诊断平台。

当前仓库按 Story 分步推进，已建立 monorepo、PRD、架构、数据库设计和 Go API Gateway，并逐步交付后端核心业务能力。后续功能仍按 `docs/stories` 中的 Story 顺序逐步实现、测试和审批。

## 产品边界

EduGrade Enterprise 的目标形态是：

- Web 管理后台
- Windows EXE 客户端
- 后端 API 服务
- AI Agent 服务
- OCR 服务接口
- 报告服务
- 审计服务
- Docker Compose 私有化部署
- 文档与验收体系

AI 阅卷只提供建议分，必须保留人工确认、复核、仲裁和审计链路。尚未实现的能力必须明确标注为待实现、stub 或 mock，不允许冒充真实能力。

## 当前状态

> 2026-07-10 快照：STORY-001～STORY-052 已按现有范围完成交付。本轮完成多租户、文件关系、任务租约、成绩状态机、生产配置、Worker 异常恢复和 Web 生产界面加固，并在真实 Docker 环境完成 8 服务 healthy、`000001`～`000024` migration、MinIO private bucket、管理员登录、重复初始化、PostgreSQL/MinIO 备份与隔离恢复验证。当前可用于受控联调和试点准备；正式上线仍需真实密钥/TLS、生产规模迁移窗口、OCR/quality 业务 E2E、性能和安全门禁。

- 已建立企业级 monorepo 骨架。
- 已建立 Story 分步交付规则。
- 已实现 Go API Gateway 基础服务、健康检查、依赖检查、session 认证、RBAC 中间件、组织管理基础 API、考试管理 API、试卷文件元数据登记、题目配置、Rubric 版本管理、私有文件上传/下载 API、答卷采集 Submission API、OCR 任务/结果接口、answer_segment 元数据生成、多智能体 Orchestrator 控制面、客观题/填空题规则判分、主观题 AI 评分接口层、规则级证据校验 Agent、人工复核/阅卷任务后端 API、双评策略、双评会话、仲裁任务、`final_grade` 后端写入、最终成绩汇总、确认、发布、锁定、学生已发布成绩查询、CSV 导出、学生申诉创建/处理/改分留痕/关闭/统计后端 API，以及学情报告、班级报告、题目分析、阅卷质量分析和报告 CSV 导出后端 API。
- 已实现 `apps/web-admin` 的 Web 管理后台基础框架：React + TypeScript + Vite + Ant Design，包含登录页、主布局、导航、权限路由、API Client、统一状态组件、表格/表单封装和清晰标注的 mock 框架页；考试管理页面、试卷与 Rubric 配置页面、答卷采集页面、阅卷工作台、双评仲裁页面、成绩管理与发布页面、学情报告页面、申诉中心页面、审计日志页面已对接真实后端 API，并在缺少真实 token 时显示明确错误状态，不回退 mock 数据。其他业务页面仍按后续 Story 对接真实 API。
- 已建立 `apps/desktop-client` 的 Windows EXE 客户端骨架：Tauri 2 + React + TypeScript，包含服务端配置、真实登录 API、真实复核任务列表 API、扫描工作站、离线阅卷基础工作台、同步队列、系统诊断、本地日志和清晰标注的“未配置/待接入”本地能力状态。扫描工作站已支持真实考试选择、批量 PDF/图片预览、本地质量检查、可恢复队列元数据、断网检测、联网后续传当前会话文件、失败重试、真实文件上传 API、submission page 关联和服务端质量门禁入口。离线阅卷基础工作台已支持获取当前教师任务、聚合真实任务包 API、当前会话答案图片预览、Rubric/AI/OCR 展示、Web Crypto 加密草稿、同步前冲突检测、真实人工评分提交入口、同步状态和过期缓存清理。
- 已实现 PostgreSQL 驱动的 Agent Worker Runtime 和语言无关 HTTP Worker 协议，覆盖 claim、lease/heartbeat、幂等完成、失败重试、超时恢复、死信、取消、人工重投、指标和审计；OCR 与图像质量任务已接入该运行时。River、Temporal 等未引入。
- 已实现独立 Python OCR Worker，可从私有文件接口读取输入、调用配置的 OCR 引擎并回写 text、bbox、confidence、引擎/模型/配置版本。PaddleOCR 是当前本地部署候选，但是否达到生产效果仍需用本项目已授权真实试卷样本评测，不能只凭接口跑通判定。
- 已实现独立 Python 图像质量 Worker，覆盖模糊、曝光、倾斜、缺边/空白等质量指标、输入尺寸上限、标准化 RGB PNG、不可变 quality run、租约重试和确定性错误终止；低质量页面可在 OCR 前被质量门禁拦截。
- 已提供 `infra/docker-compose` 私有化演示部署配置，包含 API Gateway、Web Admin、PostgreSQL、Redis、MinIO、Qdrant、AI services 占位、Nginx，以及可选 Prometheus/Grafana；提供 `.env.example`、迁移服务、MinIO bucket 初始化、初始化脚本、备份/恢复脚本和部署 README。
- 已完成 STORY-052 预生产部署闭环：配置预检、可替换基础镜像、migration filename/SHA-256 tracking、旧库显式 baseline、可重复初始化、匿名/登录冒烟、二进制安全备份、manifest/hash、隔离数据库恢复和 MinIO 对象恢复。操作见 [预生产部署 Runbook](docs/deployment/preproduction-runbook.md)。
- 真实主观题模型推理、语义/视觉证据核验、图片裁剪/自动版面切分、真实扫描仪驱动、完整离线阅卷、自动更新服务、学生端页面等能力尚未实现；现有主观题 AI、证据和部分桌面能力只提供接口边界、规则能力或明确的待接入状态，不得作为真实生产能力宣传。
- 本轮优化的改动、验证证据、未执行项和剩余风险见 [已完成范围系统性优化报告](docs/reviews/completed-system-optimization-report.md)。

## 目录结构

```text
apps/
  web-admin/             Web 管理后台
  desktop-client/        Windows EXE 客户端
  teacher-portal/        教师 Web 端，预留
  student-portal/        学生端，预留
services/                后端服务边界
ai-services/             AI/OCR/Agent 服务边界
packages/                共享类型、SDK、UI、设计 token
infra/                   私有化部署、Kubernetes、监控、脚本
docs/                    PRD、架构、API、数据库、安全、UI、部署
tests/                   e2e、load、security、ai-evaluation
```

## Story 工作流

每次只执行一个 Story。Story 完成后由 Codex 按该 Story 的验收标准自审自批，再进入下一个 Story。

当前顺序见 [docs/stories/README.md](docs/stories/README.md)。

## 开发命令

当前 Web 管理后台基础框架已可构建。常用命令：

```bash
npm.cmd install
npm.cmd run typecheck
npm.cmd run build
npm.cmd run dev
```

桌面客户端常用命令：

```bash
npm.cmd --workspace apps/desktop-client run dev
npm.cmd --workspace apps/desktop-client run build
npm.cmd --workspace apps/desktop-client run tauri:dev
npm.cmd --workspace apps/desktop-client run tauri:build
```

`tauri:dev` 和 `tauri:build` 需要本机安装 Rust/Cargo 以及 Windows 打包依赖。

Docker Compose 私有化演示：

```bash
cd infra/docker-compose
copy .env.example .env
docker compose --env-file .env -f docker-compose.yml config
powershell -ExecutionPolicy Bypass -File .\scripts\init.ps1
```
