# STORY-066：智能评分生产失败关闭

状态：Implemented

## 问题

本地开发 Mock、演示环境和真实模型服务过去缺少统一的显式开关。模型未接入时若静默落到 Mock，会让测试结果被误认为真实智能评分。

## 环境策略

- `dev/test/local`：未配置服务地址时允许显式显示为 Mock，便于本地开发。
- `demo`：仅当 `EDUGRADE_AI_GRADING_ENABLED=true` 且 `EDUGRADE_ALLOW_MOCK_AI=true` 时允许 Mock。
- `production/staging`：永远拒绝 Mock；启用智能评分必须配置受治理的服务地址和凭据。
- 生产可将 `EDUGRADE_AI_GRADING_ENABLED=false` 安全启动，此时智能评分显示“已关闭”。

## 失败语义

- 新增受保护的 `GET /api/v1/ai-grading/status`，返回 `real`、`mock` 或 `disabled` 及受治理模型/提示词版本。
- 禁用或服务不可用时，智能评分接口返回稳定 503，不创建 Mock 分数或失败评分事实。
- 模型调用失败会将运行记录置为失败并写审计，但不能改写最终成绩。
- 客观题自动判分、人工阅卷、双评和仲裁不依赖智能评分状态，必须继续可用。

## 非范围

- 不接 OpenAI-compatible 统一协议；第三方模型仍走 STORY-061 的原生厂商适配与治理。
- 不在本 Story 做模型微调或训练智能体。
- 模型治理页展示配置与评测，不等同于生产运行就绪。

## 验收证据

- 配置测试覆盖生产关闭可启动、生产拒绝 Mock、演示 Mock 必须显式授权。
- Handler 测试覆盖禁用状态返回 503。
- 系统状态、模型治理和阅卷运营页面显示真实运行模式。

