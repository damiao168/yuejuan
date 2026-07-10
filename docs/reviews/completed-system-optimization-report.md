# EduGrade Enterprise 已完成范围系统性优化报告

报告日期：2026-07-10  
项目目录：`D:\project\yuejuan`  
审阅范围：STORY-001～STORY-051 已实现内容  
结论等级：**代码级加固完成，可继续受控联调和试点准备；尚不满足正式生产发布条件**

## 1. 执行结论

本轮没有新增 Story、业务模块、页面、模型或工作流，也没有引入 River、Temporal、Kubernetes、SSO、真实主观题 AI、语义/视觉证据、答案分组或 `lab/` 集成。所有修改都用于修复和优化 STORY-001～STORY-051 已有实现。

本轮识别出的高风险代码问题已经完成修复，重点包括：Worker Runtime 读写越权、普通租户管理其他租户、文件与答卷跨租户/跨对象错配、图像质量关系约束不足、成绩状态回退和重复发布、陈旧租约重复提交、生产环境使用开发凭据、OCR/图像 Worker 异常退出和 Web 生产界面假入口等。

最终自动化回归、类型检查、静态检查和构建均通过。后续 STORY-052 已启动 Docker Desktop，并在真实 PostgreSQL 16 容器应用 `000001`～`000024`，完成 8 服务 Compose core、MinIO private bucket、管理员登录、重复初始化、PostgreSQL/MinIO 备份与隔离恢复。OCR + image-quality 的真实业务任务容器 E2E 仍未在本轮重跑，正式发布前仍需关闭对应门禁。

## 2. 范围与约束

### 2.1 本轮包含

- Go API Gateway 的认证授权、多租户、文件、答卷、试卷、成绩、OCR、图像质量和 Worker Runtime。
- Python OCR Worker 和 image-quality Worker 的运行稳定性与输入安全。
- Web Admin 的登录、生产导航、权限可见性、错误状态、表格过滤和构建性能。
- Desktop Client 的类型检查和生产构建回归。
- Docker Compose 配置解析、README、生产化路线图和本报告。

### 2.2 本轮明确不包含

- STORY-052 及之后任何 Story 的实现。
- 真实主观题模型推理、语义证据、视觉证据、相似答案或答案分组。
- River、Temporal、Kubernetes、Keycloak/SSO、OPA 等新基础设施。
- 新产品页面、新数据模型、新业务流程和 `lab/` 生产衔接。
- 把 mock、接口层或候选引擎描述为已经达到生产效果。

## 3. 优化前基线

优化前代码能够通过 Go 测试、Go vet、两个前端项目的类型检查/构建、OCR 9 项测试、图像质量 5 项测试和 STORY-049～051 静态检查，但存在以下主要问题：

| 等级 | 基线问题 | 影响 |
| --- | --- | --- |
| P0 | Worker Runtime 的读权限可用于执行 claim/complete/cancel 等写操作 | 只读运维账号可能驱动或篡改任务状态 |
| P0 | 非平台租户可进入租户创建/更新路径 | 多租户管理边界不足 |
| P0 | `file_asset`、submission page、quality run 的跨租户和跨 submission 关系缺少完整数据库兜底 | 错配数据可能污染 OCR、图像质量和答卷链路 |
| P0 | 成绩 finalize/confirm/publish 缺少完整的事务串行与防回退约束 | 重复操作可能造成状态回退或并发竞态 |
| P1 | 已成功任务的重复完成在校验 lease token 前返回幂等成功 | 陈旧 Worker 可能伪装成合法重复提交 |
| P1 | OCR/图像 Worker 的认证过期和引擎异常可能终止循环或形成错误重试语义 | 长时间运行稳定性不足 |
| P1 | production 环境仍可能使用不安全 cookie、开发数据库/MinIO 凭据和 localhost CORS | 容易把演示配置带入生产 |
| P2 | Web 登录页硬编码演示租户，顶栏存在无效搜索/通知/队列控件 | 生产体验不可信，容易误导用户 |
| P2 | Web 所有页面集中进入主 bundle | 首屏资源过大，优化前主 JS 约 2,039 KB |

仓库当前没有 `.git` 元数据，因此无法提供可靠的提交基线、`git diff` 或变更提交号。本报告以文件内容、命令结果和构建产物为证据。

## 4. P0 修复

### 4.1 Worker Runtime 权限分离

- 将执行权限与读取权限分开：创建、领取、心跳、完成、失败、取消和重投只接受 `ocr:manage` 或 `orchestrator:manage`。
- `system:read` 仅可查询任务和指标，不能驱动任务状态。
- 新增路由级权限回归测试，覆盖只读账号访问写端点返回 403。

涉及文件：`services/api-gateway/internal/server/server.go`、`server_test.go`。

### 4.2 平台租户管理边界

- 创建和更新租户必须同时满足平台租户身份与 `tenant:manage` 权限。
- 非平台租户调用租户列表时只能看到当前租户，不能枚举所有租户。
- 修正 E2E 预期，并补非平台拒绝、平台允许的测试。

涉及文件：`services/api-gateway/internal/org/handlers.go`、`handlers_test.go`、`internal/server/e2e_core_workflow_test.go`。

### 4.3 文件、答卷和质量数据关系

- 上传文件后清理 multipart 临时文件，避免大文件请求积累临时资源。
- 下载文件名改用结构化 `Content-Disposition` 生成，避免异常文件名破坏响应头。
- 新增文件 owner 元数据一致性校验，并正式支持 STORY-050 使用的 `submission_page_original`、`submission_page_normalized`。
- 答卷加页/换页前验证文件的 tenant、submission、exam 归属。
- 试卷绑定文件时将错配从模糊的 404 改为明确的 400 `paper_file_scope_mismatch`。
- 新增迁移 `000024_completed_scope_integrity_hardening.sql`，在数据库层增加复合唯一键、复合外键、owner CHECK 和索引。
- quality run 的源文件、标准化文件、submission page 与 latest quality run 均约束到同一 tenant、submission 和 page，避免“同租户不同答卷/不同页”错配。
- 迁移在加约束前执行脏数据预检，发现跨租户、跨答卷或 owner 不一致时直接失败，不静默修复历史数据。

涉及文件：`internal/files/*`、`internal/submission/*`、`internal/paper/*`、`migrations/000024_completed_scope_integrity_hardening.sql`、`internal/auth/migrations_test.go`。

### 4.4 成绩状态机与并发

- finalize、confirm、publish 在事务内获取 exam 级 PostgreSQL advisory lock，串行化同一考试的关键发布操作。
- finalize 只接受可进入待确认的状态，不能把已确认或已锁定成绩退回。
- confirm 要求全部目标成绩处于 `pending_confirmation`，重复确认被拒绝。
- publish 拒绝已发布/已锁定成绩，重复发布不再静默成功。
- Memory Store 与 PostgreSQL Store 保持相同状态语义，并增加防回退、重复确认、重复发布测试。

涉及文件：`services/api-gateway/internal/score/store_postgres.go`、`store_memory.go`、`store_test.go`。

## 5. P1 工程加固

### 5.1 Worker 租约、幂等与并发终态

- Worker Runtime 和 image-quality runtime 在处理“已成功任务的重复完成”前先验证 lease token。
- 陈旧 lease token 即使提交相同 payload 也会被拒绝。
- 新增并发领取测试，证明同一任务只能被一个 Worker 领取。
- 新增 complete 与 cancel 并发测试，证明只能有一个终态转换成功。
- image-quality PostgreSQL claim 查询增加 tenant + submission 关系，避免仅凭 id 关联。

### 5.2 OCR 业务提交恢复

- 修复“OCR 结果已写业务表，但 Runtime complete 失败”后的恢复路径。
- 相同 metadata/result 重试可继续完成 Runtime；内容变化的重复提交返回 409 `ocr_result_conflict`。
- OCR confidence、bbox 拒绝 NaN、Infinity、越界和非法坐标，避免无效浮点进入数据库或 JSON。

### 5.3 生产配置启动门禁

非 development/test/local 环境启动时新增强制校验：

- session cookie 必须启用 secure。
- PostgreSQL 不能使用开发凭据或 `sslmode=disable`。
- MinIO 不能使用示例开发凭据。
- CORS 不能继续允许 localhost。

这属于有意的兼容性收紧：原来能带着演示配置启动的“production”实例现在会拒绝启动，部署方必须显式配置真实密钥、TLS 和允许源。

## 6. P2 Worker 稳定性

### 6.1 OCR Worker

- 认证失败不再被吞掉并假装“没有任务”，主循环会重新登录。
- 下载失败、API 失败和 OCR 引擎异常按可重试错误回写。
- 非法 block、空文本、非有限 confidence、越界 confidence 和非法 bbox 被过滤。
- 主循环增加结构化日志和异常恢复，不记录密码或 token。
- 测试从 9 项增加到 11 项，新增认证过期、引擎错误和 NaN 输出覆盖。

### 6.2 图像质量 Worker

- 在完整解码前检查最大 5,000 万像素，降低压缩炸弹和超大图片内存风险。
- 损坏、不可读、超尺寸等确定性输入错误写为 `terminal_error`；网络/API 等临时错误保持可重试。
- 主循环支持认证过期重新登录和意外异常恢复。
- 测试从 5 项增加到 7 项，新增不可读和超尺寸图片覆盖。

## 7. P2/P3 Web 与桌面体验

### 7.1 生产可信度

- 登录页删除硬编码 Demo/platform tenant 选项，租户代码由用户输入。
- 顶栏删除没有真实行为的全局搜索、任务队列、通知和虚构“当前考试”。
- 阅卷工作台删除尚未实现的“相似答案、历史示例、快捷键”生产占位内容。
- 页面技术说明改为面向业务用户的文字，避免暴露实现细节或夸大能力。
- Forbidden/NotFound 增加可返回首页的恢复操作。

### 7.2 权限与错误状态

- 新增 `hasAnyPermission` 和路由 `anyPermissions`，仲裁员具备 `arbitration:work` 即可进入自己的处理页面；分配仍要求 `arbitration:manage`。
- 阅卷页面将评分、证据核验、退回等按钮按各自权限控制，避免只有页面级权限而内部操作全部可见。
- 文件名响应头解析失败时不再让下载流程因 URI decode 异常崩溃。
- 登录错误不再向用户直接暴露后端内部错误文本。
- DataTable 的搜索和过滤现在真正影响本地数据，并显示“过滤后/总数”。

### 7.3 构建性能

- Web 业务页面改为动态 `lazy` 加载，并分离 charts、motion 和 antd 公共依赖。
- Web 主入口 JS 从优化前约 2,039 KB 降到 210.85 KB，业务页面 chunk 约 5～23 KB。
- Desktop 主入口 JS 从约 1,141 KB 降到 244.55 KB。
- Ant Design 公共 chunk 仍为 Web 1,115.15 KB、Desktop 767.27 KB，构建保留大 chunk 警告；这是明确的剩余性能项，不通过调高 warning 阈值掩盖。

### 7.4 浏览器视觉验证

- 使用真实 Chromium 会话检查 Web production preview。
- 桌面视口：1440 × 900；移动视口：390 × 844。
- 登录页无文字溢出、控件遮挡、横向滚动或首屏内容缺失。
- 截图证据：`output/playwright/login-desktop.png`、`output/playwright/login-mobile.png`。
- 控制台仅出现 `/api/v1/auth/me` 连接 `127.0.0.1:8080` 失败；原因是视觉检查时 API Gateway 未启动，不是静态资源加载错误。

## 8. API 与兼容性影响

| 范围 | 变化 | 兼容性判断 |
| --- | --- | --- |
| Worker Runtime | `system:read` 不再可执行写操作 | 安全收紧；错误依赖该越权行为的客户端会收到 403 |
| Tenant Admin | 非平台租户不能创建/更新其他租户 | 安全收紧；平台管理工具需使用平台租户会话 |
| 文件/页面关联 | tenant、exam、submission、owner 不一致返回 400 | 数据正确性收紧；错误历史调用需修正元数据 |
| OCR complete | 相同完成结果可恢复，冲突结果返回 409 | 改善幂等；调用方需把 409 视为结果冲突 |
| Score | 重复 finalize/confirm/publish 或状态回退被拒绝 | 状态机收紧；调用方不能依赖重复操作成功 |
| Production config | 演示凭据、不安全 cookie/SSL/CORS 导致启动失败 | 有意的发布门禁；本地 development 不受影响 |
| 数据库 | 新增复合约束、CHECK 与索引，无删除列 | 逻辑向后兼容；迁移可能因历史脏数据失败，需要先治理 |

## 9. 最终验证结果

| 命令/验证 | 结果 |
| --- | --- |
| `go test -count=1 ./...` | 通过；22 个含测试的 Go package 通过，5 个 package 无测试文件 |
| `go vet ./...` | 通过 |
| `go test -race ./...` | 未执行成功；当前 Go 环境未启用 CGO，命令返回 `-race requires cgo` |
| `python -m pytest -q`（OCR） | 11 passed |
| `python -m compileall -q ocr_worker tests` | 通过 |
| `python -m pytest -q`（image quality） | 7 passed |
| `python -m compileall -q image_quality tests` | 通过 |
| `npm.cmd run typecheck`（workspace） | Web Admin、Desktop Client 均通过 |
| `npm.cmd run build`（workspace） | Web Admin、Desktop Client 均构建成功；保留 antd 大 chunk 警告 |
| `npm.cmd run check:production-routes` | 通过 |
| `npm.cmd run check:story049` | 通过 |
| `npm.cmd run check:story050` | 通过 |
| `npm.cmd run check:story051` | 通过 |
| `docker compose ... --profile ocr --profile quality config --quiet` | 通过 |
| Playwright 桌面/移动视觉检查 | 通过；API 未启动的连接错误已单独记录 |
| `docker info` | 后续 STORY-052 已通过；Docker Engine 29.6.1 |

## 10. 未执行项与环境限制

以下项目没有被标记为“通过”：

1. `000024` 已在干净/小数据 PostgreSQL 16 实例应用；仍未验证生产规模大表增加唯一约束/校验 CHECK 的锁时间。
2. 已启动完整 Compose core 并验证登录与恢复；OCR、image-quality worker profile 尚未配置服务账号并执行真实业务任务 E2E。
3. 未用本项目已授权真实试卷样本重新评测 PaddleOCR 的 CER/WER、手写召回、bbox、吞吐和资源消耗。
4. `go test -race` 因 CGO 未启用无法执行。
5. Web/desktop 当前没有独立 ESLint、单元测试或正式 Playwright 测试套件；本轮使用 TypeScript、生产路由静态检查、build 和浏览器视觉检查兜底。
6. 未执行 Tauri Windows 安装包签名、安装、升级和真实扫描设备验收。
7. 仓库无 Git 元数据，无法验证未提交变更来源或生成提交级审计记录。

## 11. 剩余风险与生产阻断项

### 11.1 发布阻断项

| 优先级 | 风险 | 关闭条件 |
| --- | --- | --- |
| Blocker | 迁移只在干净/小数据 PostgreSQL 验证 | 在生产等规模脱敏副本上执行预检和迁移，记录锁时间、升级窗口与恢复耗时 |
| Blocker | Worker 业务容器全链路未验收 | 配置最小权限 worker 账号，跑通上传 -> 质量检测 -> OCR -> segment -> 人工阅卷 -> 成绩确认/发布 |
| Blocker | OCR 缺正式项目样本效果报告 | 使用授权脱敏样本输出 CER/WER、bbox、低置信召回、P95、CPU/GPU 资源报告 |
| Blocker | 生产密钥、TLS、CORS 和对象存储凭据未实配 | 以非示例凭据启动 production，完成 TLS、cookie、CORS 和密钥轮换验证 |

### 11.2 重要剩余工程风险

- PostgreSQL 与 MinIO 属于两个资源系统，上传成功后数据库失败、数据库删除后对象删除失败仍需要运维补偿和孤儿对象清理策略。
- 部分“业务写入 + audit 写入”不是单一跨模块事务，正式生产前需确认关键审计失败时的阻断或补偿策略。
- Worker 目前通过用户式登录获得 Bearer 会话，服务账号最小权限、凭据轮换和会话撤销需要生产部署专项验证。
- 图像质量业务 run 与通用 Runtime task 的创建属于顺序调用，极端失败下需要巡检/补偿确保两侧状态不长期漂移。
- 学校、班级等更细粒度数据范围不能只依赖 tenant 级隔离；需要按现有角色和考试归属继续做真实用户矩阵验收。
- Ant Design vendor chunk 仍偏大；当前不影响正确性，但会影响低带宽首屏，需要后续基于真实性能数据决定组件级替换或更细粒度拆分。

## 12. 明确未实现能力

系统状态中的以下边界保持不变，本轮没有伪造实现：

- `real_subjective_model_inference`
- `semantic_evidence_verification`
- `visual_evidence_verification`
- `ai_grading`
- 自动模板识别、按题切图和完整扫描仪驱动闭环
- 答案分组、种子卷、完整质量运营体系
- 完整学生端、完整离线客户端和自动更新

`lab/` 仍只是智能体训练实验约束，没有与 API、Worker、数据库或生产部署衔接，也不计入任何上线能力。

## 13. 文档同步结果

- 根 `README.md` 已更新 OCR、图像质量、Worker Runtime 的真实状态和生产边界。
- `docs/deployment/production-readiness-roadmap.md` 已更新到 2026-07-10，并明确本轮只优化 STORY-001～STORY-051，后续 Story 仅是既有路线规划。
- 本报告作为本轮修复、测试、限制和剩余风险的统一审阅记录。

## 14. 最终判断

经过本轮优化，STORY-001～STORY-051 的实现质量、权限边界、数据一致性、Worker 恢复能力和 Web 生产可信度有明显提升，已达到继续进行真实环境集成验收的条件。

系统仍不能直接判定为“生产可上线”。Docker core、migration、登录与恢复已完成；最短关闭路径是：在生产等规模脱敏副本验证迁移窗口 -> 配置 worker 服务账号并跑通 OCR/quality 业务容器全链路 -> 用授权真实试卷完成 OCR/质量效果评测 -> 配置真实生产密钥/TLS/CORS -> 执行竞态、前端 E2E、性能和安全门禁。完成并留证后，才能进行正式发布评审。
