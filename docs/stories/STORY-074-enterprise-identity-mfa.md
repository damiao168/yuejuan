# STORY-074：企业身份与 MFA

状态：Planned。优先级：P1。依赖：既有 auth、会话撤销与审计；真实学校 IdP。

## 问题与范围

机构需要统一身份入口，并在成绩发布、批量改分和凭据管理前验证近期强认证。先实施 OIDC，再 TOTP 与 step-up；WebAuthn 后续独立验收。

- OIDC Authorization Code + PKCE：机构 provider 配置、固定 redirect URI、state/nonce、issuer/audience/签名/时间校验、JWKS 轮换、失败关闭。
- 以 `(tenant,issuer,subject)` 绑定本地账号；邮箱相同不能自动绑定或跨租户合并。机构管理员审批身份/角色映射，平台权限和保密题库访问不由外部普通 role claim 自动授予。
- TOTP 注册需当前认证，确认后生效；受控加密 secret，恢复码仅存摘要、一次使用，失败限速并防止同一时间步重复认证。
- 服务端 step-up 记录 method、auth_time、actor、session、作用域和有效期，绑定敏感命令；只在前端弹框不算完成。OIDC provider 是否满足强认证需显式校验 acr/amr 策略。
- 成绩发布、批量改分、API Key/AI Provider Key 管理与平台高风险操作接入 step-up。管理员 MFA 重置与应急账号使用留独立审计，并保持既有本地 bootstrap 恢复入口可控。
- WebAuthn 后续校验 RP ID、origin、challenge、用户验证与凭据绑定；不借其名称宣称首版已支持无密码登录。

## 非范围

首版不做 SAML/LDAP、不自动迁移所有账号、不共享机构凭据、不把 LTI Launch 当普通 OIDC provider，不凭一个 IdP claim 默认跳过本地高风险确认。

## 预计修改文件

修改 `internal/auth/`、高风险业务 handler 和会话中间件；拟新增 provider/identity binding/MFA challenge/recovery 迁移、管理端身份设置与认证页、OpenAPI/SDK、部署 Runbook。

## 测试方式与验收标准

本地协议夹具验证 nonce/state/PKCE、issuer/audience、失效 token、JWKS 轮换与安全角色映射；真实 PostgreSQL 验证恢复码并发一次性和 challenge 重放。敏感 API 直接调用也必须拒绝过期 step-up。真实 IdP 和账户恢复演练单独留证。

首版验收要求跨租户 issuer/sub 无法错绑、外部角色不越权、撤销会话失去强认证状态、TOTP 与恢复码重放拒绝、认证故障不生成登录会话、发布审计可追溯近期认证。依据 [OIDC Core](https://openid.net/specs/openid-connect-core-1_0.html)；后续强凭据依据 [WebAuthn](https://www.w3.org/TR/webauthn-3/)。

## 规划审阅与实施记录

规划已分开机构登录、MFA 和命令级强认证。2026-09-14 已先完成无外部 IdP 依赖的 TOTP 验证器管理后端试点：注册确认、独立加密密钥、一次性恢复码以及关闭／恢复码轮换的操作级挑战；实现边界与验证记录见 [TOTP 试点](../architecture/teacher-account-security-mfa-pilot.md)。该试点默认关闭，不提升全局会话等级、不要求业务 MFA，也不开放 SSO。原定真实学校 OIDC、管理端注册／挑战界面、业务命令强认证、管理员恢复演练与审批均未完成，整体 Story 尚未验收，不能据此宣称企业身份或强制 MFA 已上线。
