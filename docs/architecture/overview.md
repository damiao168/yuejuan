# Architecture Overview

## 当前阶段

本文件属于 `STORY-001 企业级项目骨架`。它只定义企业级系统边界和目录职责，不代表这些服务已经实现。

## 目标架构

```text
Web Admin / Teacher Portal / Student Portal / Desktop Client
  -> API Gateway
  -> Auth / Tenant / Exam / Paper / Submission / Grading / Report / Appeal / Audit Services
  -> PostgreSQL / Redis / MinIO / Qdrant / Observability
  -> AI Orchestrator
      -> OCR / Layout / Grading / Evidence / Consistency / Analytics Agents
```

## 应用层

- `apps/web-admin`：学校、机构、阅卷、成绩、报告、审计的管理后台。
- `apps/desktop-client`：Windows EXE 客户端，承担扫描工作站、离线阅卷和同步队列。
- `apps/teacher-portal`：教师 Web 端预留，后续可承接轻量阅卷和报告查看。
- `apps/student-portal`：学生端预留，后续承接成绩查看、反馈和申诉。

## 服务层

- `api-gateway`：认证鉴权、限流、请求 ID、租户上下文、统一错误。
- `auth-service`：登录、会话、角色、权限、OIDC/LDAP/SAML 预留。
- `tenant-service`：租户、学校、校区、年级、班级、学生边界。
- `exam-service`：考试、作业、状态流转和发布策略。
- `paper-service`：试卷、题目、Rubric、标准答案、版本审批。
- `submission-service`：答卷、页、质量状态、匿名码、采集流程。
- `grading-service`：AI 建议分、人工复核、双评、仲裁、最终分。
- `report-service`：学生、教师、管理者报告与导出。
- `appeal-service`：学生申诉、处理流转、改分闭环。
- `audit-service`：不可关闭的审计日志和导出留痕。
- `notification-service`：站内消息、任务提醒和后续 webhook。

## AI 服务层

- `orchestrator`：可审计、可重试、可回放的 Agent 工作流编排。
- `ocr-service`：OCR 服务接口，第一版可提供明确标记的 stub。
- `layout-service`：版面解析、答题区域识别和切分预留。
- `grading-agent-service`：客观题、填空题、主观题建议分服务边界。
- `evidence-agent-service`：采分点证据校验。
- `consistency-agent-service`：同类答案、阅卷员偏差和异常分检查。
- `analytics-agent-service`：学情诊断和考试质量分析。

## 数据与基础设施

- PostgreSQL：核心业务数据、状态流转、审计索引。
- Redis：缓存、任务状态、分布式锁、限流。
- MinIO/S3：试卷、答卷图片、报告和附件。
- Qdrant：样例答案、相似答案检索和后续向量能力。
- Docker Compose：本地私有化演示环境。
- Kubernetes/Helm：企业部署预留。

## 第一版必须实现与预留

第一版必须实现：多租户、权限、审计、考试、试卷、答卷、OCR 任务边界、人工复核、双评仲裁、成绩发布、申诉、报告、私有化部署。

第一版可预留接口：真实 OCR 模型、复杂公式识别、远程监考、高级模型调度、跨区域灾备。
