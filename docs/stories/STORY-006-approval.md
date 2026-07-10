# STORY-006 自审审批记录

## Story

STORY-006 认证与权限系统

## 审批结论

Approved

## 验收项

| 验收项 | 证据 | 结论 |
| --- | --- | --- |
| 用户、角色、权限、用户角色表 | migration 包含 `app_user`、`role`、`permission`、`user_role` | 通过 |
| 租户隔离 | migration 和 Store 查询均包含 tenant 关联；登录使用 `tenant_code` | 通过 |
| 登录接口 | `POST /api/v1/auth/login` 已接入并由 `TestLoginMeLogout` 覆盖 | 通过 |
| 登出接口 | `POST /api/v1/auth/logout` 删除 session，由测试覆盖 | 通过 |
| me 接口 | `GET /api/v1/auth/me` 需要认证，由测试和启动验证覆盖 | 通过 |
| session 认证 | token 只保存 hash，me/logout 通过 Bearer token 查 session | 通过 |
| 密码 hash | 使用 `golang.org/x/crypto/bcrypt`，测试用户也使用 hash | 通过 |
| 种子数据 | migration 初始化 platform_admin、tenant_admin、school_admin、teacher、grader、auditor | 通过 |
| 权限中间件 | `RequirePermission` 能拒绝缺少权限用户，由测试覆盖 | 通过 |
| 登录/登出/失败登录审计 | handler 调用 `Audit`，失败登录测试验证审计事件 | 通过 |
| 统一错误结构 | 未认证启动验证返回 JSON error + request_id | 通过 |
| 没有越界业务 | 未实现组织、考试、试卷、阅卷等业务 API | 通过 |

## 运行命令与结果

```powershell
go test ./...
```

结果：

```text
ok  	edugrade-enterprise/services/api-gateway/internal/auth
ok  	edugrade-enterprise/services/api-gateway/internal/config
ok  	edugrade-enterprise/services/api-gateway/internal/server
```

启动验证：

```powershell
go build -o .\bin\api-gateway.exe .\cmd\api-gateway
curl http://127.0.0.1:18086/api/v1/system/info
curl -i http://127.0.0.1:18086/api/v1/auth/me
```

结果：

```text
system/info -> capabilities include session_auth, rbac_middleware, login_audit
auth/me without token -> 401 unauthenticated
```

## 剩余风险

- 尚未运行真实 PostgreSQL migration 集成测试；当前验证以 migration 文件审阅和 MemoryStore HTTP 测试为主。
- OIDC/SAML/LDAP 未实现，属于企业集成后续能力。
- 数据范围只存储和返回基础 data_scope，尚未实现学校/班级/考试维度的完整策略计算。
- 登录失败对未知租户使用 platform tenant 审计兜底，后续可在租户服务完善后细化。

## 下一步

进入 `STORY-007 组织、学校、班级、学生管理`，实现租户、学校、年级、班级、学生 CSV 导入、教师班级绑定、数据范围控制和操作审计。
