# STORY-053 产品外壳、角色首页、机构启用和考试工作区

## Status

Approved

## Goal

把 STORY-052 的“可联调模块集合”升级为学校人员可以从登录后入口开始工作的产品外壳。机构管理员无需 Swagger、临时 SQL 或开发人员协助即可完成基础启用；业务人员进入一场考试后能够持续保留考试上下文并进入后续业务页面。

## User Outcomes

- 机构管理员首次登录后看到启用向导，并可创建学校、学年/年级、班级、学生、基础用户和第一场考试。
- 启用进度由真实业务数据推导并保存在服务端资源中，退出后重新登录可以继续。
- 考试负责人首页优先看到待办、进行中考试、风险和下一步，不显示数据库、Worker、MinIO 等底层术语。
- 阅卷员只看到继续阅卷和自己的工作量入口。
- 扫描员只看到采集、上传失败、待匹配和缺页/重复页入口。
- 运维管理员才可以看到服务健康和任务运行状态。
- 用户选择考试后进入统一 Exam Workspace，顶部始终保留考试名称、科目、状态、进度和主要操作。

## Scope

### Product shell

- 按角色与权限生成主导航，不允许所有角色看到全部菜单。
- 业务主导航收敛为：工作台、考试、采集、阅卷、质量、发布、报告。
- 机构管理员增加“组织与用户”，系统管理员增加“系统运维”。
- 顶栏显示当前机构和个人菜单；普通业务用户不显示底层技术状态。
- 所有新增页面具备 Loading、Empty、Error、Permission denied 和 Retry 状态。

### Role home

- 依据当前用户角色和权限展示对应首页。
- 首页数据来自现有考试、答卷、阅卷、成绩、审计或系统状态 API；部分接口失败时展示局部失败，不把整页清空。
- 常用操作不超过 6 个，并直接进入真实页面。
- 当前最重要的待办和进行中考试位于第一屏。

### Organization activation

- 启用步骤：机构信息、学年与年级、班级、学生、基础用户、第一场考试、完成。
- 支持保存真实资源后退出并继续，不使用只存在浏览器中的伪进度。
- 支持学生 CSV 示例下载、导入预览、字段映射、重复检测和错误行下载。
- 基础用户创建仅覆盖用户名、显示名、初始密码和一个内置业务角色；启停、锁定、会话撤销、密码重置和服务账号归 STORY-060。
- 页面不显示 tenant_id、数据库 id、内部权限代码。

### Exam workspace

- 从考试列表或首页进入 `/exams/:examId/overview`。
- 工作区头部展示考试名称、科目、状态、负责人、阶段进度和主要操作。
- 内部导航覆盖概览、学生、试卷、题目与 Rubric、答卷模板、采集、处理、阅卷、质量、成绩、申诉、报告、设置。
- 本 Story 只建立上下文、导航和真实概览；各业务页继续复用当前已实现页面，复杂配置与准备门禁由 STORY-054 完成。
- 当前考试保存在 URL，而不是依赖易失的全局变量；刷新和复制链接后上下文仍然有效。

### Backend and audit

- 新增租户内基础用户列表、角色列表和创建接口，密码使用现有 bcrypt 规则存储。
- 用户创建必须校验租户内用户名唯一、角色归属当前租户，并写审计日志。
- 不允许客户端提交 tenant_id 或任意 permission 集合。
- 继续复用 Go 模块化单体、PostgreSQL、现有认证和 RBAC，不引入新服务或新数据库。

## Out of Scope

- 完整考试配置向导、试卷/模板版本化和准备检查，归 STORY-054。
- Capture Batch、扫描仪接入、页面重排和自动切题，归 STORY-055。
- 新的客观题引擎、专业阅卷台、AI、质量中心、发布中心和学生端，归 STORY-056～059。
- 用户锁定/解锁、密码重置、会话治理、Service Principal 和运维中心，归 STORY-060。
- 正式 UAT、性能、安全、安装包和灾备总门禁，归 STORY-061。

## Data and API Contract

- `GET /api/v1/users`：返回当前租户用户的安全字段，不返回密码哈希。
- `GET /api/v1/roles`：返回当前租户可分配的内置角色代码和业务名称。
- `POST /api/v1/users`：创建当前租户用户并分配一个角色。
- 学校、年级、班级、学生和考试继续使用现有 API。
- 学生导入由前端完成本地预览和字段映射，再调用现有 CSV 导入接口；服务端继续执行租户和父对象校验。

## Security and Tenant Rules

- 用户管理接口要求 `org:manage`。
- 所有用户与角色查询强制限定当前 session 的 tenant。
- 密码不进入响应、日志或审计 after_value。
- 创建用户、学校、年级、班级、学生导入和考试继续写审计。
- 未完成机构启用不绕过后端权限，也不阻断运维管理员进入系统状态页。

## Files Expected to Change

- `services/api-gateway/internal/auth/*`
- `services/api-gateway/internal/server/*`
- `apps/web-admin/src/api/*`
- `apps/web-admin/src/auth/*`
- `apps/web-admin/src/components/*`
- `apps/web-admin/src/pages/*`
- `apps/web-admin/src/router/*`
- `apps/web-admin/src/styles.css`
- `docs/stories/README.md`
- `scripts/check-story053-product-shell.mjs`
- 相关 Go/TypeScript/Playwright 测试

## Acceptance Criteria

- [x] 机构管理员只通过 Web 可以创建学校、年级/学年、班级、学生、基础用户和第一场考试。
- [x] 中途退出后，重新登录可从真实已完成步骤继续。
- [x] 学生 CSV 可预览、映射、检测重复并下载错误行。
- [x] 不同角色首页内容与主导航不同，普通业务角色不看到系统依赖和内部任务术语。
- [x] 所有首页操作进入真实可用页面，不存在无行为按钮。
- [x] 从考试列表进入 Exam Workspace 后，刷新仍保留考试上下文。
- [x] 工作区内部导航保留 examId，并能跳转到现有真实业务页面。
- [x] 基础用户创建不能跨租户分配角色，响应和日志不泄露密码哈希。
- [x] 新增 API 具备权限、租户隔离、重复用户名和审计测试。
- [x] Web typecheck/build、Go test/vet 和 STORY-053 静态检查通过。
- [x] Playwright 覆盖管理员启用、角色导航和考试工作区核心路径。

## Verification

```text
cd services/api-gateway && go test -count=1 ./... && go vet ./...
npm.cmd run typecheck
npm.cmd run build
npm.cmd run check:story053
Playwright: login -> onboarding -> create first exam -> open workspace -> refresh
```

## Plan Review

结论：需要修订后进入实现。

发现的问题：

- 总控任务要求完整启用流程包含创建用户，原有后端没有安全的租户内用户创建 API，不能用 bootstrap 或临时 SQL替代。
- 如果为首页新增一个大型聚合后端，会把多个后续 Story 的业务统计提前耦合；本 Story 应复用现有 API，并允许局部失败。
- 如果用 localStorage 保存“已完成步骤”，资源创建失败或换浏览器后会产生虚假进度；应从真实学校、年级、班级、学生、用户和考试数据推导。
- 当前考试必须进入 URL，否则刷新、深链接和并发打开多场考试都会丢失或串场。
- 当前角色模型可能一个用户有多个角色，首页选择不能仅看 `roles[0]`，必须结合权限并采用明确优先级。
- 总控任务的完整 CSV 导入能力较大；本 Story 应完成首次启用所需的预览、映射、重复与错误下载，批量用户导入留到 STORY-060。

## Spec Fixes

- 将基础用户列表/角色列表/创建纳入本 Story，并严格限定为“首次启用最小能力”。
- 首页采用现有 API 的并行读取与局部失败，不新增跨域大聚合服务。
- 启用进度由真实资源推导，浏览器只保留非权威的当前步骤提示。
- Exam Workspace 使用带 `examId` 的 URL 作为唯一上下文来源。
- 角色首页按权限能力优先级选择：运维、扫描、阅卷、考试负责人/机构管理员。
- 学生 CSV 在客户端安全解析预览后生成服务端接受的标准 CSV；错误行可下载，服务端仍作为最终校验者。

修订结论：Spec Ready。

## Implementation

### Backend and data

- 新增 `GET /api/v1/users`、`GET /api/v1/roles` 和 `POST /api/v1/users`，统一由 `org:manage` 权限保护。
- 用户、角色与角色分配均在 PostgreSQL 查询和事务层限定当前租户；客户端不能指定 `tenant_id`、权限集合或高权限角色。
- 新用户密码沿用 bcrypt，并执行至少 12 位、大小写字母、数字和符号的强度规则；响应与审计数据不包含密码或哈希。
- 新增 migration `000025_story053_product_shell_rbac.sql`，补齐学校管理员创建和管理考试所需权限；migration 已在本机真实 PostgreSQL 执行。
- 考试 API 补回 `created_at`、`updated_at` 字段，考试列表可显示真实创建时间，不再以缺省技术提示代替。

### Product shell and onboarding

- 主导航按角色与权限生成；阅卷员只看到工作台和阅卷，机构管理员看到考试业务与组织管理，平台管理员才看到系统运维。
- 首页改为角色化工作台。普通用户看到待办、进行中的考试、常用操作和需要关注；平台管理员保留服务状态入口。
- 新增组织启用向导，覆盖机构、学年/年级、班级、学生 CSV、基础用户和第一场考试。进度从服务端真实资源推导，刷新或重新登录后可继续。
- CSV 使用 Papa Parse 在浏览器本地完成编码解析、字段映射、预览、重复检测和错误行下载，合格行再交给现有服务端导入接口执行最终租户与父对象校验。

### Exam Workspace

- 新增 `/exams/:examId/:section` 深链接，考试 ID 是唯一上下文来源；刷新、复制链接和同时打开多个考试不会串场。
- 工作区头部显示考试名称、科目、真实关联年级、状态、负责人、总体进度、刷新和下一步操作。
- 内部导航覆盖 13 个要求环节。已完成的试卷、采集、阅卷、质量、成绩、申诉和报告页面直接嵌入并接收 `initialExamId`，避免再次选择考试或回到全局页面。
- 工作区概览并行读取考试、试卷、题目、答卷和年级信息；考试主数据失败显示可重试错误，局部数据失败显示业务警告并保留其余页面。
- 桌面与 390 px 移动视口均完成浏览器检查；长导航横向滚动，内容不会挤压或覆盖。

### Deployment, rollback and user documentation

- Compose 部署继续使用现有 Go API、PostgreSQL、React 静态站点和 Nginx，不增加新的运行服务。
- 数据库升级通过顺序 migration 前滚；代码回滚时保留新增表权限和兼容字段，旧版本可忽略新增 API，禁止删除已创建用户或考试数据。
- 用户操作说明见 `docs/user-guides/organization-onboarding-and-exam-workspace.md`。

## Implementation Review

结论：发现并修复 7 项产品或安全问题，完成后无 STORY-053 阻断项。

1. 学校管理员原角色缺少 `exam:manage`，启用向导会在最后一步失败。通过 migration 补齐并在真实数据库验证。
2. 初版可分配角色包含租户管理员，存在组织管理员提权风险。内存和 PostgreSQL Store 均排除 `platform_admin`、`tenant_admin`，并增加越权测试。
3. Ant Design 分步表单复用字段状态，前一步输入会污染后一步。各步骤改为独立表单实例并在切换时销毁非当前字段。
4. 考试列表曾向用户显示创建者 UUID 和“真实 API”标识。改为当前用户业务名称，并清理正式业务页中的研发状态文案。
5. 工作区导航长标签在桌面端被截断。调整为内容宽度并允许外层横向滚动，移动端截图复验通过。
6. 工作区最初跳回全局业务页，用户需要再次选择考试。现有业务页增加可选 `initialExamId` 并直接嵌入工作区。
7. 工作区头部最初缺少年级。通过考试班级关联真实反查年级；跨年级显示“多个年级”，局部查询失败不阻断工作区。

剩余风险：Vite 构建仍报告 Ant Design 分块超过 500 kB；页面已按路由懒加载，进一步分块和加载性能由 STORY-061 的性能门禁量化处理。Windows Docker 数据盘依赖系统盘剩余空间，属于部署容量检查项。Nginx 在上游容器重建后需要 reload/restart，需在 STORY-061 升级脚本中自动化。

## Implementation Fixes

- 增加角色边界、重复用户名、跨租户角色、伪造 tenant 字段、密码泄露和权限缺失的后端测试。
- 增加生产菜单静态门禁和 STORY-053 产品外壳检查。
- 修复考试时间字段、角色首页差异、学校名称解析、表单状态隔离、考试上下文嵌入、年级显示和响应式导航。
- 将“后端未返回”“真实 API”等内部研发文案改为用户可理解的业务状态；模拟 AI 标识保留，避免把实验结果伪装为正式模型输出。

## Business Acceptance Evidence

- 在真实 Compose 环境创建验收学校、学年、年级、班级、1 名学生、1 名阅卷员和第一场数学考试，全程未调用 Swagger、SQL 或手工补状态。
- CSV 预览同时识别有效行、重复学号和未知班级，只导入有效行；错误数据可下载。
- 启用进度达到 6/6，浏览器刷新后仍为 6/6。
- 阅卷员重新登录后只看到工作台与阅卷；机构管理员看到考试业务和组织管理；平台运维菜单未暴露给普通用户。
- 工作区在桌面和移动端打开并刷新后保持同一考试；试卷配置页嵌入后自动选中当前考试。

## Approval

Approved。

批准依据：Go 全包测试与 vet、Web/桌面端 typecheck 与生产构建、STORY-049～053 静态回归均通过；API 和 Web 生产镜像重建成功，Compose 8 个项目服务健康，Nginx 健康检查返回 200；生产地址下管理员工作区刷新、真实年级、嵌入考试上下文和阅卷员角色菜单通过 Playwright，两个验收会话均无控制台错误或警告。

本批准只覆盖 STORY-053。考试完整配置、采集处理、评分、质量、发布、运维与 V1.0 总验收仍由 STORY-054～061 完成，因此不得据此声明 EduGrade Enterprise V1.0 Production Ready。
