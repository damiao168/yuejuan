# STORY-044 严重与高风险问题修复规格

## 目标

根据 STORY-043 审查结果，优先修复阻断上线的严重问题和高风险问题，不做大规模重构。

## 范围

本轮修复：

- 默认迁移不得创建可用的固定密码账号。
- Web 管理端登录必须调用真实后端认证接口，不再用 mock session 放行。
- 阅卷任务区分管理权限和工作权限，普通阅卷员只能查看和提交分配给自己的任务。
- 仲裁任务区分管理权限和工作权限，普通仲裁员只能查看和提交分配给自己的任务。
- 组织和考试写入路径必须校验父对象属于当前租户，先用 SQL/事务校验降低风险。

本轮不修复：

- 真实 OCR、LLM、Agent Runtime 接入。
- Web token 存储从 `localStorage` 迁移到 HttpOnly cookie。
- 全量复合外键重建。

## 规格审阅

- 默认账号风险属于严重问题，必须消除“公开密码 + active 用户”组合。
- Web mock 登录属于严重问题，必须让输入的租户、账号、密码进入真实 `/api/v1/auth/login`。
- 阅卷/仲裁不能只依赖客户端 `assigned_to` 查询参数，后端必须强制收敛。
- 多租户父子关系先在写入 SQL 中补 `EXISTS` 校验，后续 story 再升级为数据库级复合外键。

## 规格修改

审阅后收窄为最小可交付修复：

- 迁移中保留 demo 用户行但默认 `disabled`，密码改为不可预测随机值，避免破坏角色/演示引用。
- 新增 `review:work` 和 `arbitration:work` 权限；管理员保留 `manage`，普通工作角色使用 `work`。
- Web 登录继续使用现有 API client token 机制，先解除 mock 登录阻断；token 存储风险记录为剩余风险。

## 实现计划

1. 写失败测试，锁定迁移中不能出现 `crypt('ChangeMe123!'`。
2. 写失败路由测试，锁定 `review:work` 用户只能访问本人任务。
3. 写失败路由测试，锁定 `arbitration:work` 用户只能访问本人仲裁任务。
4. 修改迁移默认账号和权限种子。
5. 修改 review handler/server 路由权限和对象级检查。
6. 修改 Web 登录页、session 映射和认证 API。
7. 修改组织/考试 Postgres 写入 SQL，补同租户父对象校验。
8. 运行 Go 测试、Web typecheck、Compose config。

## 验收

- `go test ./...` 通过。
- `npm.cmd run typecheck` 通过。
- `docker compose --env-file infra/docker-compose/.env.example -f infra/docker-compose/docker-compose.yml config` 通过。
- STORY-044 文档和故事索引更新。

## 实现摘要

- 固定默认账号：基础迁移中的内置账号改为 `disabled`，密码改为随机不可预测值；新增 `000019_story044_rbac_hardening.sql` 清理旧库中仍使用默认密码的内置账号。
- 权限拆分：新增 `review:work` 和 `arbitration:work`，普通阅卷员/仲裁员不再持有管理权限。
- 对象级授权：`review:work` 用户列表/详情强制收敛到本人任务；`arbitration:work` 用户只能查看和提交分配给自己的仲裁任务。
- Web 登录：登录页改为调用真实 `/api/v1/auth/login`，启动时用 `/api/v1/auth/me` 恢复 session，移除 `edugrade.mock_session`。
- 多租户父对象：组织和考试 Postgres 写入路径增加同租户父对象校验，阻止跨租户 school/grade/class 引用。

## 新增测试

- `services/api-gateway/internal/auth/migrations_test.go`
  - 验证迁移不再用 `crypt('ChangeMe123!', gen_salt('bf'))` 创建固定默认密码。
- `services/api-gateway/internal/server/review_route_test.go`
  - 验证 `review:work` 只能列表/读取/提交本人任务，不能分配或退回。
  - 验证 `arbitration:work` 只能读取/提交分配给自己的仲裁任务。
- `services/api-gateway/internal/org/store_postgres_test.go`
  - 验证组织 Postgres 写入 SQL 包含同租户父对象校验。
- `services/api-gateway/internal/exam/store_postgres_test.go`
  - 验证考试 Postgres 写入 SQL 包含同租户 school/class 校验。

## 实现审阅

- 已复核 `ChangeMe123!` 在迁移中的剩余出现只用于旧库检测：`password_hash = crypt('ChangeMe123!', password_hash)`，不会创建新默认密码。
- 已复核 Web 源码不再包含 `edugrade.mock_session`、`mockSession`、默认账号或默认密码。
- 已复核 `review:work` / `arbitration:work` 的路由允许进入读取/提交路径，但 handler 会按当前用户强制对象级过滤。
- 已复核 `lab/` 仍未接入生产链路，本轮未修改。

## 修改实现

- 保留 Web 端 `localStorage.edugrade.access_token` 作为本轮剩余中风险；本轮只解除 mock 登录阻断，避免同时改动会话架构。
- 保留 SQL/事务级父对象校验作为本轮防线；复合外键硬化放入后续 story。

## 测试结果

- `go test ./...`：通过。
- `npm.cmd run typecheck`：通过。
- `docker compose --env-file infra/docker-compose/.env.example -f infra/docker-compose/docker-compose.yml config`：待最终收尾验证。

## 剩余风险

- Web bearer token 仍存放在 `localStorage`，后续应迁移到 HttpOnly Secure SameSite cookie 或内存 access token + refresh 机制。
- 真实 OCR/LLM/Agent Runtime 仍是 placeholder/mock，不能按生产智能阅卷能力验收。
- 生产环境禁用默认账号后，需要正式的首个管理员 bootstrap 流程或运维 SQL 流程。
- 多租户隔离已经在关键写入路径补校验，但数据库级复合外键仍需后续硬化。
