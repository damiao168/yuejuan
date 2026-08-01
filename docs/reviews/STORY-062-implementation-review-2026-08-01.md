# STORY-062 阅卷主链可靠性实现复审（2026-08-01）

## 结论

有条件通过软件实现复审。评分启动门禁、管理员只读监控、整卷阅卷监控、教师连续阅卷、任务租约、草稿版本冲突和预取失败保护均已实现并取得测试或浏览器证据。该结论不等于真实打印/扫描验收，也不等于 Production Ready。

## 实现范围

- 新增评分就绪度查询和结构化阻断项；就绪度检查与创建评分任务在同一事务保护范围内，活动任务和取消恢复均有门禁。
- 管理员阅卷工作区展示题块总量、自动/人工处理量、失败量、阅卷员进度和只读阅卷区；管理员不能写入教师草稿或最终分。
- `ScoringPaperMonitor` 按匿名答卷分组加载整卷页面，在原答题区域右上侧展示红色题分覆盖层；AI 建议分使用“≈”标记，并同时展示当前合计、页数和待确认题目。
- 教师工作台在提交后优先切换到下一份并预取任务工作区和题块影像；选择任务时重新核验归属和状态，陈旧任务、影像失败和草稿旧版本均安全失败。

## 验证证据

- Go：`go test ./internal/grading ./internal/modelgovernance ./internal/server -count=1` 通过。
- Web：`npm.cmd run typecheck --workspace apps/web-admin` 和 `npm.cmd run build --workspace apps/web-admin` 通过；构建仅有既有 Ant Design chunk 大小提示。
- PostgreSQL E2E：评分就绪度阻断、放行、取消恢复和活动任务冲突用例通过；Worker、OMR 和 Story-060 相关回归通过。
- Playwright 管理员验收：本地 Docker 环境中打开 `阅卷监控`，验证 `SIM-FJ2024-001` 的 2 页答卷、原图红色题分、AI 建议“≈”标识、`72/72` 当前合计和待确认清单。证据截图：`output/playwright/story062-admin-monitoring.png`、`output/playwright/story062-admin-monitoring-page2.png`。
- Playwright 教师验收：提交 Q4 后自动切换 Q5；预取成功、任务重分配后的 403 防陈旧缓存、答题图 503 回退和旧版本草稿 409 均已验证。故障注入后 Q1/Q5 已恢复为 `pending`、未分配且无租约。
- Docker：浏览器验收执行时，`edugrade-enterprise` 的 API、Web、PostgreSQL、Redis、MinIO、Qdrant 等服务均健康运行；本记录不把之后的 Docker Desktop 运行状态视为持续在线证明。

## 审查边界与未批准项

- 未执行真实打印机、扫描仪或纸张回扫；该项继续由 STORY-060 的 `V-060P` 物理验收负责。
- 未接入真实第三方模型 API、未发送学生数据、未声称 OCR 或模型准确率；第三方原生 Adapter、真实沙箱审批和生产路由仍属于 STORY-061 的外部前置条件。
- 当前评分任务仍有待人工确认项；管理员监控明确保持只读，教师最终裁定仍是成绩写入的唯一入口。

## 审批结论

批准 STORY-062 进入软件试点验收和后续人工阅卷回归；不批准将其标记为全链路生产就绪。下一复审点是完成教师批量任务与质量仲裁的持续运行证据，并在具备设备后补齐 STORY-060 物理链路报告。
