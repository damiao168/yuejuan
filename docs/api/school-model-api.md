# 学校模型 API 与考试资料识别

平台管理员在“模型配置”中选择学校，填写供应商、接口协议、Base URL、API Key、模型名称、模型版本和服务区域。可以创建多个配置，选择一个作为该校默认 API。

- 默认 API 用于该校考试资料的内容解析；图片预处理和文字 OCR 仍由本地 Worker 执行。
- 启用默认 API 后，资料中的文字会发送到指定供应商。页面在保存前显示这一用途。
- 未选择默认 API 时沿用本地模型；已选默认 API 被停用或密钥无法解密时返回失败，不静默切换供应商。
- 主观题评分仍遵守原有模型治理、评测和教师复核流程，不因资料解析 API 分配而切换。
- 密钥在 PostgreSQL 中通过 AES-GCM 加密，只返回掩码。网关仅在向已鉴权的内部解析服务发起请求时传递解密后的密钥；密钥不写入任务结果或模型提示词。

## 接口

平台接口同时要求平台管理员身份和 `model:provider:manage` 权限。

| 方法 | 路径 | 用途 |
| --- | --- | --- |
| GET | `/api/v1/platform/model-api-configs?tenant_id=...` | 查看学校配置，不返回密钥 |
| POST | `/api/v1/platform/model-api-configs` | 创建配置，正文包含 `tenant_id` |
| PATCH | `/api/v1/platform/model-api-configs/{id}?tenant_id=...` | 编辑配置，密钥留空表示保留 |
| POST | `/api/v1/platform/model-api-configs/{id}/probe?tenant_id=...` | 连接测试 |
| POST | `/api/v1/paper-imports/{id}/cancel` | 停止当前识别并保留已上传资料 |

OpenAI 兼容协议连接测试访问 `/models`，仅证明接口可访问，不保证指定模型具备完整 JSON 提取能力。DashScope 原生协议固定使用 `https://dashscope.aliyuncs.com/api/v1`，连接测试发送一条最小生成请求，可能消耗少量供应商额度。

解析请求使用当前学校的默认配置，逐请求创建模型适配器，不修改共享全局模型。兼容协议使用 JSON 模式并在本地严格验证结构；供应商仍须支持 JSON 模式和足够长的输出。

## 任务状态

页面展示页面预处理、文字识别、AI 内容解析的阶段进度，百分比为阶段指示。轮询临时失败后自动恢复；轮询期间不卸载页面。无题目、答案、解析或评分标准的资料显示“未识别到考试内容”，保留重新识别和手动补充入口。

停止后任务状态为 `cancelled`，后续到达的结果不能覆盖停止状态。重新识别复用已上传资料。
