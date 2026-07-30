# STORY-061B1A 厂商沙箱准入门禁实现复审（2026-07-30）

## 结论

通过。已把真实厂商沙箱联调的前置条件实现为确定性、fail-closed 的纯函数门禁。该门禁不解析 Secret、不执行 I/O、不注册 Adapter，也不产生网络请求。

## 强制准入条件

- Provider 必须是 `external + dashscope_native`，禁止 OpenAI-compatible Adapter。
- Provider、Deployment 和 Tenant Policy 必须分别通过现有治理校验。
- Provider 必须 active，Deployment 必须 `shadow_only + available`，且只允许 `shadow_compare + manual_only`。
- Deployment 必须在租户 allowlist 内；文本和图片分别要求独立导出授权。
- 图片还必须具有单独的人工外发评审结论。
- Secret probe 必须显示解析器受支持、已配置且满足最小强度；决策结果不包含 Secret 引用或值。
- 每题与每场考试预算必须大于零，Deployment 必须具有已评审价格计量方式。
- 沙箱账号、合同、留存、数据驻留、价格和指定区域必须全部完成评审。
- 首次联调只允许合成数据。
- 批准引用必须是受限内部标识，批准有效期必须大于当前时间且不超过 90 天。

## 测试覆盖

- 合成文本影子沙箱满足全部条件时通过。
- 图片缺少独立评审时阻断，补齐后通过。
- 缺少凭据、账号、合同、留存、驻留、价格或合成数据限制时分别输出稳定 blocker。
- 非 `shadow_compare`、批准过期或批准期无界时阻断。
- OpenAI-compatible 协议、区域不一致和未知 modality 时阻断。
- blocker 排序稳定，不泄露 Secret 引用和批准事实。

## 运行边界

- `AssessSandboxAdmission` 当前没有生产调用方。
- 它只消费现有治理快照、脱敏 Secret probe 和审批事实，不读取环境变量或 Secret 文件。
- Provider 默认注册表仍只有 `local_llama_cpp`。
- v1/v2 grading-agent 路由、请求上限和应用均未修改。
- 真实 DashScope 账号、密钥、Endpoint 和网络调用仍不存在。

## 验证结果

- `go test ./... -count=1`：通过。
- `go vet ./...`：通过。
- `gofmt -l .`：通过，无未格式化文件。
- `staticcheck v0.7.0 ./...`：通过。
- `npm run check:story057`：通过。
- `git diff --check`：通过。

## 下一决策点

下一切片 061B1B 只允许建立未注册、无默认网络实现的原生传输接缝，并通过注入式合成 transport 验证协议。完成它也不代表真实沙箱已获批；061B1C 仍须真实账号和本门禁要求的全部审批事实。
