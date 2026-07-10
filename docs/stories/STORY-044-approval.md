# STORY-044 完成记录

## Story

严重与高风险问题修复。

## 规格

- 已根据 STORY-043 审查结果确定优先修复默认账号、Web mock 登录、阅卷/仲裁对象级权限、多租户父对象归属。
- 已明确真实 OCR/LLM/Agent、Web token 存储架构、复合外键重建不纳入本轮。

## 规格审阅与修改

- 规格审阅后，将默认账号修复收敛为“禁用 + 随机密码 + 旧库清理迁移”。
- 将阅卷/仲裁权限收敛为 `manage` 与 `work` 两层，先通过 handler 强制对象级范围。
- 将多租户父对象修复收敛为 SQL/事务级校验，后续再做数据库约束硬化。

## 实现审阅

- 新增失败测试后实现修复，目标测试已由失败转为通过。
- 已确认 Web 源码中不再存在 mock 登录入口和默认账号密码。
- 已确认迁移不再创建固定默认密码账号。

## 测试

- `go test ./...`
- `npm.cmd run typecheck`
- `docker compose --env-file infra/docker-compose/.env.example -f infra/docker-compose/docker-compose.yml config`

## 结论

STORY-044 已完成严重和高风险问题的最小修复。仍有中风险和上线配套事项，需要进入后续 story 继续处理。
