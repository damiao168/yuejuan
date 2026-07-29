# STORY-060D 软件联合验收记录（2026-07-29）

## 结论

STORY-060 的**软件范围验收通过**，状态调整为 `Software Accepted / Physical Validation Pending`。

本结论不等于 STORY-060 全量批准，也不等于产品可进入 Production Ready。当前没有真实打印机和扫描仪，验收标准 3 中“30 份答题卡真实打印回扫”的物理链路没有执行，必须作为外部验收硬门禁保留。

软件范围通过后可以开始 STORY-061 的实现；但在实体回扫报告批准前：

- 不得向试点学校开放批量正式印卡。
- 不得把条码打印、扫描、配准链路标记为生产验证完成。
- 不得以 PDF 渲染、屏幕截图或软件解码结果代替实体设备证据。

## 验收基线

- 被测提交：`5a97aa6`
- 分支：`main`
- 浏览器入口：`http://127.0.0.1:8088`
- 容器运行时：Docker Desktop 29.6.1
- 数据库：一次性 PostgreSQL 16 容器
- 桌面工具链：Rust/Cargo 1.97.1，安装于 D 盘
- 物理设备：无真实打印机、无真实扫描仪

## 验收标准结果

| 验收项 | 结果 | 证据与边界 |
| --- | --- | --- |
| 1. 500 人花名册对账、抽走答卷、缺考与发布门禁 | 通过 | `TestStory060FiveHundredStudentRosterScaleE2EWithPostgresTestDatabase` 通过；存储对账 166 ms、完整对账 181 ms |
| 2. 卡纸、插页、缺页不造成后续学生错位 | 软件范围通过 | `TestStory060MultiPageInsertionMissingPageAndPrintPackageE2EWithPostgresTestDatabase` 通过；插入页进入人工路径，受控页码不漂移 |
| 3. 30 份真实打印回扫、解码率与配准偏差 | 待外部验收 | A4 PDF、旧/新条码兼容和生产同源软件解码已有自动化证据；真实纸张、打印缩放、扫描畸变和设备噪声未验证 |
| 4. 模板级 OMR 校准、无偏分层抽样和审批 | 通过 | STORY-060 模板 profile 校准 PostgreSQL E2E 通过；高/低置信、blank、ambiguous 样本均纳入 |
| 5. 失败文件重跑、同 hash 重传、质量人工放行 | 通过 | `TestStory060CaptureRecoveryE2EWithPostgresTestDatabase` 通过；恢复 Worker task、保留审计事实并继续配准 |
| 6. 门禁事实生产者与人工评分关联 AI 建议 | 通过 | STORY-060/既有评分测试通过；恒零假门禁已删除，`human_grade.ai_grade_id` 由服务端关联 |
| 7. 全量工程门禁 | 通过 | 见下方命令矩阵 |

## PostgreSQL 联合回归

以下五组测试在一次性 PostgreSQL 16 实例中联合执行，全部通过，总耗时 40.837 秒：

- `TestStory060CaptureRecoveryE2EWithPostgresTestDatabase`
- `TestStory060RosterReconciliationAndAbsenceE2EWithPostgresTestDatabase`
- `TestStory060FiveHundredStudentRosterScaleE2EWithPostgresTestDatabase`
- `TestStory060MultiPageInsertionMissingPageAndPrintPackageE2EWithPostgresTestDatabase`
- `TestStory060StudentSheetIdentityE2EWithPostgresTestDatabase`

CI 的 PostgreSQL workflow 使用 `PostgresTestDatabase|Story056.*E2E` 过滤器，上述五组测试名均包含 `PostgresTestDatabase`，因此已进入 `main` 的持续回归范围。

## 真实浏览器回归

使用 headed Chromium 完成登录和考试工作区回归，未直接调用页面内部实现：

1. 进入真实考试的“采集”页，确认生产入口为“答卷采集批次”。
2. 验证文件与页面表格、四份答卷的页面处理状态，以及四份待确认学生匹配任务。
3. 验证学生匹配页明确展示答卷页码、名册确认、未知标记和合并入口。
4. 进入“识别处理”页，确认页面醒目标记“当前页面是兼容模式”，并明确指向“答卷采集批次”作为生产入口。
5. 页面交互后没有新增控制台错误；唯一错误是未登录启动阶段 `/api/v1/auth/me` 返回的预期 401。

浏览器截图和快照保存在本机忽略目录 `output/playwright/story060d/`，不提交账号会话或临时验收产物。

## 工程门禁

本地通过：

- `go test ./... -count=1`
- `go vet ./...`
- `staticcheck@v0.7.0 ./...`
- `govulncheck@v1.6.0 ./...`：业务代码调用路径 0 个漏洞
- `go mod verify`、`go mod tidy -diff`、`gofmt`
- `npm run typecheck`
- `npm run build`
- Web 生产路由、桌面安全配置、STORY-049/050/051/052/053/057 静态门禁
- `check:lab-integration`
- lab 129 项测试、22 条合成评测和开发门禁
- OCR worker 31 项、图像质量 worker 13 项、页面处理 worker 55 项
- AI 服务 22 项 unittest、AI evaluation 4 项 pytest
- `ruff 0.16.0`
- 主项目和 lab 的 `npm audit --audit-level=high`：0 个漏洞
- 主 Compose、STORY-056 Compose、STORY-060 Compose 配置校验
- `cargo fmt --check`、`cargo check --locked`、`cargo clippy --locked --all-targets --all-features -- -D warnings`

前端构建仅有 Ant Design chunk 超过 500 kB 的既有性能提示，不影响本次正确性验收，但应在后续性能切片处理中继续拆包。

## 外部验收清单

获得真实打印机和扫描仪后，必须使用锁定模板完成：

1. 打印至少 30 份学生答题卡，记录打印机型号、驱动、纸张、DPI、缩放和单双面设置。
2. 覆盖平板扫描与自动进纸；至少包含轻微旋转、偏移、阴影、折痕和不同批次纸张。
3. 统计逐页条码解码率、学生归属正确率、页码正确率和配准偏差分布。
4. 人工制造插页、缺页、重复学生答卷和卡纸续扫，确认后续答卷不串位。
5. 由非开发人员复核原始扫描件、系统归属、异常队列和审计日志。
6. 报告通过后再将 STORY-060 标记为 `Approved`，并解除正式批量印卡门禁。
