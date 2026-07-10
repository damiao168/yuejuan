# EduGrade Enterprise 企业级技术架构

## 1. 总体架构

```text
┌──────────────────────────────────────────────────────────────┐
│ Client Layer                                                 │
│  Web Admin │ Teacher Portal │ Student Portal │ Windows EXE   │
└───────────────────────┬──────────────────────────────────────┘
                        │ HTTPS / WebSocket
┌───────────────────────▼──────────────────────────────────────┐
│ API Gateway                                                  │
│  AuthN/AuthZ │ Tenant Context │ Rate Limit │ Request ID       │
└──────────────┬───────────────────────────────────────────────┘
               │ REST / gRPC / Internal Events
┌──────────────▼───────────────────────────────────────────────┐
│ Business Services                                             │
│ Auth │ Tenant │ Exam │ Paper │ Submission │ Grading │ Report │
│ Appeal │ Audit │ Notification                               │
└──────────────┬───────────────────────────────────────────────┘
               │ Events / Jobs / Signed file URLs
┌──────────────▼───────────────────────────────────────────────┐
│ AI Agent Layer                                                │
│ Orchestrator │ OCR │ Layout │ Grading │ Evidence │ Quality   │
└──────────────┬───────────────────────────────────────────────┘
               │
┌──────────────▼───────────────────────────────────────────────┐
│ Data & Infra                                                  │
│ PostgreSQL │ Redis │ MinIO/S3 │ Qdrant │ Logs │ Metrics      │
└──────────────────────────────────────────────────────────────┘
```

架构目标是支持真实学校、机构和考试中心的私有化使用。所有核心数据以租户为边界，所有关键操作可审计，AI 只作为建议与证据生成节点，不直接成为最终评分者。

## 2. Web 管理后台架构

Web 管理后台使用 React + TypeScript，后续可使用 Ant Design 风格的企业级组件体系。

职责：

- 租户、学校、年级、班级、学生和教师管理。
- 考试、试卷、题目、Rubric、阅卷策略配置。
- 答卷采集、OCR 状态、切分状态和质量问题处理。
- 阅卷工作台、人工复核、双评仲裁、成绩发布、申诉处理。
- 学情报告、考试质量分析、审计日志和系统状态。

架构要求：

- 所有接口通过 API Gateway，前端不直接访问对象存储、数据库或 AI 服务。
- 所有页面必须根据服务端权限返回决定可见字段和可执行动作，不能只靠前端隐藏。
- 所有 mock/stub 数据必须在 UI 中明确标记。
- 长任务通过轮询或 WebSocket 显示任务状态。

第一版必须实现：管理后台框架、登录态、考试/试卷/答卷/阅卷/成绩/申诉/审计核心页面。

可预留：教师独立门户、学生独立门户、多语言、复杂可视化大屏。

## 3. Windows EXE 客户端架构

Windows EXE 推荐 Tauri 2 + React + TypeScript。

职责：

- 服务端地址配置和设备绑定。
- 扫描工作站：批量选择 PDF/图片、上传队列、失败重试。
- 离线阅卷：下载任务包、离线草稿、联网同步、冲突处理。
- 本地诊断：连接状态、版本、缓存、队列、错误日志。
- 自动更新预留：内网更新源、签名校验、灰度升级。

本地能力边界：

- 本地缓存使用 SQLite 或安全存储抽象。
- 答卷图片、任务包和草稿必须加密或由安全存储接口托管。
- EXE 不直接访问对象存储，不持有 MinIO/S3 永久凭据。
- 离线提交必须带任务版本、Rubric 版本和签名，服务端负责幂等与冲突校验。

第一版必须实现：登录、服务端配置、设备绑定、文件选择上传、同步队列、离线阅卷接口骨架。

可预留：真实扫描仪驱动、摄像头监考、锁屏、复杂原生插件。

## 4. 后端服务架构

后端建议使用 Go。第一阶段可从模块化单体或少量服务起步，目录保留微服务边界，避免过早拆分造成复杂度失控。

服务职责：

- `api-gateway`：认证、鉴权、限流、租户上下文、统一错误、请求 ID。
- `auth-service`：账号、密码 hash、会话/JWT、角色、权限、OIDC/LDAP/SAML 预留。
- `tenant-service`：租户、学校、校区、年级、班级、学生、教师绑定。
- `exam-service`：考试创建、状态流转、适用班级、发布策略。
- `paper-service`：试卷文件、题目、答题区域、标准答案、Rubric 版本。
- `submission-service`：答卷、页、质量标记、匿名码、采集流程。
- `grading-service`：AI 建议分、人工分、双评、仲裁、最终分。
- `report-service`：报告生成、导出、水印和分享。
- `appeal-service`：申诉提交、处理、改分闭环。
- `audit-service`：审计日志、不可篡改链、导出记录。
- `notification-service`：站内消息、任务提醒、异步通知。

通信方式：

- 外部客户端到 Gateway：HTTPS REST，必要时 WebSocket。
- Gateway 到服务：第一版可用内部 REST；高并发任务可引入 gRPC。
- 服务间异步任务：Redis Streams、BullMQ、Celery、Temporal 或等价队列；第一版可先使用 Redis 队列抽象。
- AI 服务调用：后端通过 Orchestrator 创建 AgentJob，不让前端直接调用 AI 服务。

## 5. AI Agent 服务架构

AI Agent 是受控任务节点，不是聊天式多角色系统。

```text
Grading Service
  -> Orchestrator
      -> AgentJob
          -> OCR Agent
          -> Layout Agent
          -> Rubric Parser Agent
          -> Objective Grading Agent
          -> Subjective Grading Agent
          -> Evidence Verifier Agent
          -> Consistency Checker Agent
          -> Anomaly Detector Agent
          -> Analytics Agent
```

AgentJob 必须记录：

- tenant_id、exam_id、submission_id、answer_segment_id。
- agent_type、status、retry_count。
- input_payload、output_payload、error_message。
- model_version、prompt_version、rubric_version。
- mock 标记。
- created_at、updated_at。

第一版必须实现：AgentJob 数据结构、任务状态、失败重试、mock/stub 明确标记、结果落库、审计链路。

可预留：复杂模型调度、多模型投票、GPU 资源池、自动 Prompt 优化。

## 6. OCR 服务架构

OCR 服务通过统一 Adapter 暴露能力。

职责：

- 接收 OCR 任务。
- 读取 MinIO/S3 中的答卷页。
- 输出文本、bbox、confidence、page_id、engine_name、engine_version。
- 将低置信度结果标记为 `needs_manual_review`。
- 记录任务日志和失败原因。

第一版允许 mock OCR，但必须满足：

- 输出 `mock=true`。
- API 和 UI 中显示 mock/stub。
- 不得宣称已经接入真实 OCR。

后续可接入 PaddleOCR、本地 OCR、公式识别、云 OCR 或学校自建 OCR。

## 7. 数据存储架构

### 7.1 PostgreSQL

用途：

- 租户、学校、班级、用户、角色、权限。
- 考试、试卷、题目、Rubric、答卷、成绩、申诉。
- OCR 结果、AI 评分、人工分、最终分。
- 审计日志索引和业务状态。

要求：

- 核心表必须包含 tenant_id。
- 分数使用 numeric/decimal。
- 关键表包含 created_at、updated_at、deleted_at。
- 状态流转必须由服务端校验。

### 7.2 Redis

用途：

- 登录会话和短期缓存。
- 任务队列状态。
- 分布式锁。
- 限流计数。
- 文件上传分片状态。

Redis 不存放长期成绩、答卷正文或审计主记录。

### 7.3 MinIO/S3

用途：

- 试卷文件。
- 答卷图片和 PDF。
- 切分后的答案图片。
- 报告文件和导出文件。

规则：

- 数据库只保存 file_asset 元数据和对象 key。
- 客户端不得直接访问真实对象路径。
- 下载使用短期签名 URL 或后端流式下载。

### 7.4 Qdrant

用途：

- 样例答案向量。
- 相似答案检索。
- 同类答案分组。
- 后续辅助一致性检查。

第一版可只保留接口和集合设计，不要求真实向量检索上线。

## 8. 对象存储设计

对象路径建议：

```text
tenant/{tenant_id}/exam/{exam_id}/paper/{file_id}
tenant/{tenant_id}/exam/{exam_id}/submission/{submission_id}/page/{page_id}
tenant/{tenant_id}/exam/{exam_id}/answer-segment/{segment_id}
tenant/{tenant_id}/exam/{exam_id}/report/{report_id}
```

文件元数据必须包含：

- tenant_id、school_id、exam_id、submission_id。
- file_name、content_type、size、hash、storage_key。
- created_by、created_at、deleted_at。

文件访问必须经过权限检查和审计。

## 9. 缓存与任务队列设计

任务类型：

- 文件上传完成后的预处理任务。
- OCR 任务。
- 答题区域切分任务。
- AI 阅卷任务。
- 证据校验任务。
- 报告生成任务。
- 导出任务。

状态：

```text
pending -> running -> succeeded
pending -> running -> failed -> retrying -> failed
pending -> running -> needs_manual_review
```

要求：

- 支持失败重试和最大重试次数。
- 支持幂等键，避免重复执行。
- 支持任务日志查询。
- 任务失败不得阻塞整场考试，只能影响对应答卷或答案段。

## 10. 多租户隔离设计

隔离原则：

- 所有核心表包含 tenant_id。
- 所有请求在 Gateway 解析 tenant context。
- 所有查询必须带 tenant 过滤。
- 对象存储路径包含 tenant_id。
- Qdrant collection 或 payload 必须包含 tenant_id。
- 审计日志包含 tenant_id。

平台管理员访问多租户数据必须有显式平台级权限；普通租户用户不得通过参数切换 tenant_id。

## 11. 权限系统设计

权限由五层组成：

- RBAC：角色权限。
- ABAC：考试、学校、年级、学科、任务属性。
- Data Scope：租户、学校、年级、班级、题目、阅卷任务。
- Field Permission：姓名、学号、成绩、私密备注。
- Action Permission：导出、发布、批量改分、审批、归档。

服务端必须强制校验权限。前端只负责显示，不作为安全边界。

## 12. 审计日志设计

审计日志记录：

- tenant_id。
- actor_id。
- action。
- target_type。
- target_id。
- before_value。
- after_value。
- reason。
- ip_address。
- user_agent。
- request_id。
- created_at。

关键要求：

- 审计日志不可通过普通业务接口删除或修改。
- 改分、导出、发布、权限变更必须审计。
- 审计导出本身也必须产生审计日志。
- 可引入 hash chain 防篡改。

## 13. 模型服务抽象

模型服务必须通过统一接口调用。

输入：

- 脱敏后的题目、Rubric、答案 OCR 文本、证据图像引用。
- model_version、prompt_version、rubric_version。
- 评分策略和风险阈值。

输出：

- suggested_score。
- confidence。
- matched_points。
- missing_points。
- evidence。
- risk_flags。
- needs_human_review。
- mock。

模型输出必须 schema 校验，失败则作废并进入人工处理。

## 14. 私有化部署架构

第一版私有化部署：

```text
nginx
  -> web-admin static files
  -> backend API
backend
  -> postgres
  -> redis
  -> minio
  -> qdrant
  -> ai-services
```

要求：

- `.env` 管理密钥，不硬编码生产密码。
- 支持初始化脚本、数据库迁移和 MinIO bucket 初始化。
- 支持健康检查和日志查看。
- 支持备份恢复。

企业版预留：

- Kubernetes。
- Helm Chart。
- 外部 PostgreSQL/Redis/MinIO。
- GPU 节点和模型服务池。
- 多实例横向扩展。

## 15. 本地离线阅卷架构

流程：

```text
教师登录 EXE
  -> 获取 review_task 列表
  -> 下载任务包
  -> 离线阅卷并保存草稿
  -> 联网后同步
  -> 服务端校验任务版本、Rubric 版本、幂等键
  -> 成功写入 human_grade / 冲突进入处理
```

任务包内容：

- answer_segment。
- 答案图片短期访问或本地缓存副本。
- OCR 文本。
- Rubric。
- AI 建议分。
- model_version、prompt_version、rubric_version。

安全要求：

- 任务包有有效期。
- 本地缓存加密。
- 设备可远程吊销。
- 同步结果可见且可重试。

## 16. 文件上传与断点续传架构

上传流程：

```text
Client -> API: create upload session
Client -> API: upload chunk(s)
API -> Object Storage: write chunk/object
API -> DB: file_asset metadata
API -> Queue: preprocessing job
```

要求：

- 文件类型白名单。
- 文件大小限制。
- 文件 hash 防重复。
- 分片状态存 Redis。
- 上传完成后写审计。
- 私有文件下载必须鉴权。

## 17. 成绩发布与申诉流程架构

成绩发布：

```text
grading_done
  -> quality_check
  -> subject_lead_confirm
  -> grade_director_confirm
  -> published
  -> appeal_window
  -> archived
```

发布前阻断项：

- 未完成阅卷。
- 未完成仲裁。
- OCR 失败未处理。
- 答案切分失败未处理。
- 分数超过满分。
- 缺少最终分。
- Rubric 未锁定。

申诉处理：

```text
student appeal
  -> permission check
  -> reviewer decision
  -> optional score adjustment
  -> audit log
  -> student visible result
```

## 18. 可观测性

必须具备：

- request_id / trace_id。
- 结构化日志。
- API 请求日志。
- 错误日志。
- 慢查询日志预留。
- 服务健康检查。
- 队列长度和失败率。
- OCR/AI 任务成功率和耗时。
- 导出、发布、改分等关键审计指标。

禁止在普通日志中输出学生答案全文、成绩明细和敏感身份字段。

## 19. 备份恢复

备份对象：

- PostgreSQL 数据库。
- MinIO 对象。
- 配置文件。
- 许可证。
- Qdrant 集合，可按能力阶段启用。

恢复要求：

- 支持定时备份。
- 支持手动备份。
- 支持恢复演练。
- 明确 RPO/RTO。
- 备份文件加密和访问控制。

## 20. 灾备预案

第一版：

- 单机或单集群私有化部署。
- 定时备份。
- 故障后手动恢复。

企业版预留：

- 主备数据库。
- 对象存储复制。
- 多节点服务部署。
- 异地备份。
- 灰度升级和版本回滚。

## 21. 安全边界

安全边界：

- 客户端不可信。
- 前端权限不可信。
- 外部模型不可信。
- 上传文件不可信。
- 学生答案文本不可信。
- 普通日志不允许保存敏感数据。

必须防护：

- 越权访问。
- 租户逃逸。
- 文件路径穿越。
- 恶意 PDF/图片。
- Prompt 注入。
- 下载泄露。
- 会话劫持。
- 明文密码。
- 审计篡改。

## 22. 第一版实现边界

第一版必须实现：

- 后端基础服务和健康检查。
- 多租户、认证、权限、审计。
- 组织、考试、试卷、答卷、文件。
- OCR 任务接口和明确 mock/stub。
- 答案切分。
- Agent Orchestrator。
- AI 建议分接口和证据校验。
- 人工复核、双评仲裁、成绩发布、申诉。
- Web 管理后台核心页面。
- Windows EXE 骨架、扫描上传和离线阅卷基础能力。
- Docker Compose 私有化部署。

可以预留接口：

- 真实扫描仪驱动。
- 复杂公式识别。
- 高级远程监考。
- GPU 资源调度。
- 多区域灾备。
- 大规模模型评测平台。
