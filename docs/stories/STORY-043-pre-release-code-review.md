# STORY-043 上线前代码审查

## 状态

Reviewed - release blockers found.

本轮只做审查和记录，不修改业务代码。`lab/` 目录按项目说明视为智能体训练实验草稿，当前未开始实现、未与生产代码衔接，因此不纳入生产上线能力审查，也不作为未接线生产代码判定。

## 审查范围

- 后端：认证、RBAC、租户隔离、阅卷/仲裁、成绩发布、报告、文件、审计、服务启动和迁移脚本。
- Web 管理端：登录、权限展示、API 客户端、阅卷/成绩/报告/审计页面调用链。
- 桌面端：离线阅卷任务下载、离线存储、Tauri 权限。
- 部署：Docker Compose、示例环境变量、健康检查。
- 文档：安全原则、企业验收清单、Story 产物。

## 严重问题

### S1 默认迁移会创建固定密码的启用账号

证据：

- `services/api-gateway/migrations/000001_auth_rbac.sql:172` - `000001_auth_rbac.sql:183` 创建 `platform_admin`、`tenant_admin`、`school_admin`、`teacher`、`grader`、`auditor`，密码均为 `ChangeMe123!`。
- `services/api-gateway/migrations/000014_double_mark_arbitration.sql:27` - `000014_double_mark_arbitration.sql:31` 创建 `arbitrator`，密码为 `ChangeMe123!`。
- `services/api-gateway/migrations/000015_final_grades_publishing.sql:44` - `000015_final_grades_publishing.sql:48` 创建 `student`，密码为 `ChangeMe123!`。
- `apps/web-admin/src/pages/LoginPage.tsx:17` - `LoginPage.tsx:20` 默认填充 `demo / school_admin / ChangeMe123!`。

影响：

如果迁移脚本被用于真实环境，系统会带着公开可猜的高权限账号上线。即使文档把这些账号描述为本地演示账号，迁移本身没有环境门禁，风险仍然存在。

建议：

- 将演示账号从基础迁移中移出，放入 demo-only seed 脚本或 Docker demo profile。
- 生产迁移不得创建固定密码账号；初始管理员应通过一次性随机密码、安装向导或运维注入创建。
- 对所有种子账号增加强制改密、禁用或环境变量显式确认机制。

### S2 Web 管理端登录仍是 mock，未接真实认证链路

证据：

- `apps/web-admin/src/App.tsx:41` - `App.tsx:44` 登录动作只写入 `edugrade.mock_session=active` 并跳转。
- `apps/web-admin/src/auth/session.ts:12` - `session.ts:44` mock session 直接授予多项管理权限，包括组织、考试、成绩、报告、审计等。
- `apps/web-admin/src/api/client.ts:96` - `client.ts:98` API token 依赖手工写入的 `localStorage.edugrade.access_token`，不是登录流程产物。
- `apps/web-admin/src/pages/LoginPage.tsx:17` - `LoginPage.tsx:20` 表单默认值是演示账号，提交参数未用于真实登录。

影响：

后台产品仍存在“看起来可登录、实际不校验账号密码”的假功能。后端接口依然会校验 bearer token，但 Web 管理端自身无法完成企业用户的真实登录和权限加载，不满足上线验收。

建议：

- 接入真实 `/auth/login` 和 `/auth/me`，从后端用户、角色、权限生成前端 session。
- 生产构建禁用 mock session，并让未认证页面只展示真实登录入口。
- 增加 Web 登录、权限降级、登出、token 失效的端到端或集成测试。

## 高风险问题

### H1 阅卷/仲裁存在对象级权限边界不足

证据：

- `services/api-gateway/migrations/000013_human_review_workflow.sql:10` - `000013_human_review_workflow.sql:16` 将 `review:manage` 授予 teacher、grader、auditor 等多类角色。
- `services/api-gateway/internal/server/server.go:347` - `server.go:353` 阅卷任务的创建、列表、获取、分配、提交、退回共用 `requireReviewManage`。
- `services/api-gateway/internal/review/handlers.go:37` - `handlers.go:42` 列表只读取可选 `assigned_to` 查询参数，没有按当前用户强制收敛。
- `services/api-gateway/internal/review/store_postgres.go:51` - `store_postgres.go:62` 列表查询仅按租户和可选 assignee 过滤。
- `services/api-gateway/internal/review/store_postgres.go:150` - `store_postgres.go:168` 提交成绩时才校验 `AssignedTo == reviewerID`，说明其他读/退回/分配路径没有相同对象级约束。
- `services/api-gateway/internal/review/store_postgres.go:507` - `store_postgres.go:512` 仲裁列表同样只按租户和可选 assignee 过滤。
- `services/api-gateway/internal/review/store_postgres.go:577` - `store_postgres.go:603` 仲裁提交只在已有 assignee 时阻止非本人提交；列表和读取仍可跨本人任务。

影响：

普通阅卷员或仲裁员持有宽泛 manage 权限后，可能查看、退回、分配或读取非本人任务。桌面端虽然会用 `assigned_to=user.id` 拉取“我的任务”，但安全边界必须由后端强制，不能依赖客户端筛选。

建议：

- 拆分管理权限和工作权限，例如 `review:manage` / `review:work`、`arbitration:manage` / `arbitration:work`。
- 对 worker 角色默认按当前用户强制过滤列表、读取、下载、提交、退回。
- 仅主管类角色允许跨人分配、退回、查看全量任务。
- 增加“阅卷员不可读/不可退回/不可分配非本人任务”和“仲裁员不可读非本人任务”的测试。

### H2 多租户父子关系主要依赖业务代码，数据库外键未强制同租户

证据：

- `services/api-gateway/migrations/000002_organization.sql:14` - `000002_organization.sql:24` `grade.school_id` 只引用 `school(id)`。
- `services/api-gateway/internal/org/store_postgres.go:88` - `store_postgres.go:94` 创建年级时写入当前 `tenant_id` 和请求提供的 `school_id`，未在 SQL 层校验 school 属于同租户。
- `services/api-gateway/migrations/000002_organization.sql:27` - `000002_organization.sql:39` 班级引用 `school(id)`、`grade(id)`，不是复合租户外键。
- `services/api-gateway/internal/org/store_postgres.go:120` - `store_postgres.go:127` 创建班级时未校验 school/grade 与当前租户一致。
- `services/api-gateway/migrations/000003_exam_management.sql:1` - `000003_exam_management.sql:31` 考试和考试班级引用 school/class 也没有 tenant-aware FK。
- `services/api-gateway/internal/exam/store_postgres.go:18` - `store_postgres.go:39`、`store_postgres.go:182` - `store_postgres.go:190` 写入考试和考试班级时没有同租户父对象校验。

影响：

只要调用者能拿到或猜到其他租户 UUID，就可能创建“当前租户行引用其他租户父对象”的脏数据。后续查询通常会带 `tenant_id`，但跨租户引用会破坏隔离模型，可能导致数据串联、统计错误、工作流异常或间接泄露。

建议：

- 在关键表增加 `(tenant_id, id)` 唯一约束，并将子表改为复合外键引用 `(tenant_id, parent_id)`。
- 或者在每个写事务中用 `EXISTS` 明确校验父对象属于当前租户。
- 增加跨租户 parent id 写入失败测试，覆盖组织、考试、试卷、成绩、阅卷任务。

## 中风险问题

### M1 Web 管理端 bearer token 使用 localStorage

证据：

- `apps/web-admin/src/api/client.ts:96` - `client.ts:98` 从 `localStorage` 读取 `edugrade.access_token` 作为 bearer token。

影响：

当前未发现 `dangerouslySetInnerHTML`、`eval` 等明显 XSS 入口，但一旦未来页面或依赖出现 XSS，localStorage bearer token 可被直接窃取。

建议：

- Web 端优先使用 `HttpOnly + Secure + SameSite` cookie 会话。
- 如果继续使用 bearer token，应至少改为内存访问 token + refresh 机制，并配合严格 CSP。

### M2 OCR/AI/Agent 能力仍是 placeholder/mock，不能作为生产智能阅卷验收

证据：

- `ai-services/placeholder_server.py:9` - `placeholder_server.py:16` 明确返回 placeholder 服务信息。
- `services/api-gateway/internal/server/server.go:160` 使用 `subjective.NewMockLLMAdapter()`。
- `docs/deployment/enterprise-acceptance-checklist.md` 已将 OCR、AI 服务、Agent Runtime 标为上线前待接入。

影响：

这部分没有被伪装成已生产可用能力，风险主要是验收口径：当前系统可以验收流程、权限、文档和部分规则能力，但不能宣称真实 OCR/LLM/智能体阅卷已经完成。

建议：

- STORY-044 之后单独规划真实 OCR/AI/Agent 接入 story。
- 每个 mock 输出继续保留 `mock/source/confidence` 标记，直到真实服务接入并通过验收。

## 低风险问题

### L1 Request ID 接受客户端传入值但未做长度/字符规范化

证据：

- `services/api-gateway/internal/middleware/middleware.go:94` - `middleware.go:106` 直接复用请求头 `X-Request-ID` 或 `X-Correlation-ID`。

影响：

Go HTTP 层会限制非法响应头，服务也设置了 `MaxHeaderBytes`，因此不是直接高危漏洞。但过长或异常格式的 trace id 会影响日志检索、审计关联和可观测性质量。

建议：

- 对传入 request id 设置长度上限和字符白名单，非法时生成新的服务端 id。

## 已检查且未发现明显问题的点

- 未发现生产代码中有明显 SQL value 字符串拼接；动态 SQL 主要用于占位符编号、排序字段白名单或导出字段。
- 文件上传有大小限制、文件名清洗、类型校验；下载接口按租户查询并要求 `file:manage`。
- 学生报告接口在 handler 内区分 `report:read` 和 `student:report:read`，学生路径会校验本人 `student_id`。
- `/health` 公开，`/ready` 和系统探针受 `system:read` 保护。
- 桌面端 Tauri capability 当前只启用 `core:default`，未发现 shell 或宽泛文件系统能力。
- Docker Compose 使用 `infra/docker-compose/.env.example` 时，Postgres、Redis、MinIO、Qdrant、AI placeholder 地址均解析为容器网络地址。

## 建议修复顺序

1. 移除或门禁所有固定密码种子账号，先消除上线即被登录的风险。
2. 接入 Web 管理端真实登录和后端权限加载，删除生产 mock session。
3. 拆分阅卷/仲裁管理权限和工作权限，补齐对象级授权。
4. 补齐多租户父子关系的数据库约束或事务级校验。
5. 调整 Web token 存储策略。
6. 将真实 OCR/AI/Agent 能力拆入后续 story，并保留 placeholder 不可上线的验收标记。

## 可以直接修复的问题清单

- 将演示账号 seed 从基础 migration 移到 demo profile。
- 给 Web 登录页接入真实 `/auth/login`，并移除生产 mock session。
- 为 review/arbitration 增加 worker-scoped list/get/return/submit 策略。
- 为组织、考试、试卷、成绩等关键写路径添加跨租户 parent id 失败测试。
- 对 `X-Request-ID` 做长度与字符规范化。

## 需要确认的问题清单

- 生产部署是否允许保留任何内置演示账号；如果允许，是否必须默认禁用。
- Web 管理端真实认证采用 cookie session、bearer token，还是接入 SSO/OIDC。
- 阅卷主管、普通阅卷员、仲裁员、审计员的最终权限边界是否按“只看本人任务”执行。
- 首个正式版本是否需要真实 OCR/LLM/Agent worker；如果不需要，发布说明必须明确是流程演示/规则验收版本。

## 结论

STORY-043 审查完成，但正式上线结论为不通过。必须进入 STORY-044，优先修复固定默认账号、Web mock 登录、阅卷/仲裁对象级权限和多租户父子关系约束。
