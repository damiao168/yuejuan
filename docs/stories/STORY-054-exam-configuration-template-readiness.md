# STORY-054 考试配置、试卷模板、题目、Rubric 和开考准备

## Status

Approved

## Goal

把现有“考试 CRUD + 试卷与题目编辑页”升级为一条可以真正完成开考准备的纵向流程。学校管理员必须能在 Exam Workspace 内完成考试范围、试卷版本、题目、答案、Rubric、固定版式答卷模板和准备检查；任何关键配置不完整时，服务端必须阻止考试进入采集。

## User Outcomes

- 考试负责人从工作区看到清晰的配置步骤、完成度、阻断项和下一步，不需要在全局菜单反复选择考试。
- 试卷、题目、标准答案和 Rubric 使用明确版本；已经锁定的评分依据不能被静默覆盖。
- 管理员可在试卷/答卷底图上框选每题答题区域，不再手写坐标 JSON。
- 管理员可运行开考准备检查，看到每一项是否通过、为什么失败以及跳转到哪里修复。
- 只有服务端检查全部通过并由有权限人员确认，考试才能进入“准备完成”；配置变化会使旧确认失效。

## Supported Scope

### Exam configuration

- Exam Workspace 内提供基础信息、学生范围、试卷、题目与答案、Rubric、答卷模板、准备检查的连续配置流程。
- 复用现有学校、班级、试卷文件、题目、答案版本和 Rubric API；不重写已可用页面。
- 考试总分必须等于题目分值合计；题号、排序和答题区域必须完整。
- 客观题必须有可解析的标准答案；主观题必须有已锁定且分值匹配的 Rubric。

### Versioned answer sheet template

- 新增 `answer_sheet_template` 版本实体，按考试保存模板名称、关联试卷版本、页尺寸、页面数、定位标记、身份区域和题目区域。
- 坐标统一保存为相对页面宽高的归一化值 `x/y/width/height`，范围为 0～1；同时记录 `page_no` 和 `question_id`。
- 模板草稿可编辑；锁定后只读。修改已锁定模板必须创建新版本，不能原地覆盖。
- 第一版支持 PDF/PNG/JPEG 试卷文件作为底图。PDF 在浏览器使用 PDF.js 渲染，不把原始试卷发送到第三方服务。
- 视觉编辑器支持选择页面、缩放、拖拽框选、移动/调整题目区域、删除、题目关联和保存；复杂自动版面理解留给 STORY-055。

### Readiness gate

- 新增实时准备检查和确认接口，检查项至少包括：考试班级与学生、试卷文件、题目数量、题目总分、答案、主观题 Rubric、答题区域、锁定模板、题目区域覆盖和区域合法性。
- 检查结果使用稳定的业务代码、中文消息、严重级别和修复 section；前端不解析英文数据库错误。
- 检查通过后记录不可变快照、配置指纹、确认人和时间，并把考试置为 `ready`（准备完成）。
- 通用状态接口不得绕过准备检查进入 `ready` 或 `collecting`。开始采集前再次比较配置指纹；旧快照失效则回到配置处理。
- 所有检查、确认、失效和模板版本操作写审计。

## Technical Route and Open-source Reference

- 参考 [1EdTech QTI 3](https://www.1edtech.org/standards/qti/index) 的 item/test/response/scoring 分层，但本 Story 不引入完整 QTI 运行时。现有关系模型继续作为系统事实源，未来在边界层提供 QTI 导入导出。
- 参考 [TAO Authoring](https://userguide.taotesting.com/user-documentation/latest/public/what-is-tao) 的“内容创作、评分配置、预览、发布/交付”分段，把开考确认作为独立门禁，而不是一个无条件状态按钮。
- 参考 [Moodle Question Bank](https://docs.moodle.org/402/en/mod/quiz/question) 的题目版本、ready/draft 状态和历史思路，锁定后的答案与 Rubric 采用追加版本，避免覆盖已用于阅卷的依据。
- 参考 [OMRChecker](https://github.com/Udayraj123/OMRChecker) 的模板驱动思想，但不直接嵌入其 Python CLI。项目采用与现有 worker 契约一致的归一化 JSON，STORY-055 再由图像处理 worker 消费。
- 前端选用 `pdfjs-dist` 仅做本地 PDF 页面渲染，使用原生 Pointer Events + CSS overlay 完成矩形编辑；不增加 Fabric/Konva 等大型画布框架，控制 bundle 和交互复杂度。

## Data Model

### `answer_sheet_template`

- `id`, `tenant_id`, `exam_id`, `exam_paper_id`
- `version_no`, `name`, `status` (`draft|locked|retired`)
- `page_count`, `layout` JSONB, `content_hash`
- `created_by`, `locked_by`, `locked_at`, timestamps, soft delete
- 唯一约束：`tenant_id + exam_id + version_no`
- `layout.pages[]` 包含 `page_no`, `width`, `height`, `registration_marks[]`, `identity_regions[]`, `question_regions[]`

### `exam_readiness_snapshot`

- `id`, `tenant_id`, `exam_id`, `configuration_hash`
- `status` (`passed|invalidated`), `checks` JSONB
- `confirmed_by`, `confirmed_at`, `invalidated_at`, `invalidation_reason`
- 每次确认追加快照；不覆盖历史审计证据。

## API Contract

- `GET /api/v1/exams/{examId}/answer-sheet-templates`
- `POST /api/v1/exams/{examId}/answer-sheet-templates`
- `PATCH /api/v1/answer-sheet-templates/{id}`（仅草稿）
- `POST /api/v1/answer-sheet-templates/{id}/lock`
- `POST /api/v1/answer-sheet-templates/{id}/clone`
- `GET /api/v1/exams/{examId}/readiness`
- `POST /api/v1/exams/{examId}/readiness/confirm`
- `POST /api/v1/exams/{examId}/start-collection`

所有接口要求 `exam:manage`；模板底图下载继续要求现有 `file:read`。请求体拒绝未知字段，所有 Store 查询限定 session tenant。

## Frontend Product Design

- Visual thesis：安静、密集、以试卷页面为主画布的考试配置台，强调“当前阻断项”和“下一步”，不做营销式卡片。
- Content plan：顶部考试上下文与步骤进度；左侧页面/题目列表；中央试卷画布；右侧区域属性与准备检查；底部只保留当前步骤主操作。
- Interaction thesis：步骤切换保持 examId；框选区域时即时高亮对应题目；保存/锁定使用短反馈并在配置失效时明确显示状态变化，不使用装饰动画。
- `/exams/:examId/template` 渲染模板视觉编辑器；`/overview` 展示服务端 readiness，而不是前端估算。
- PDF 页面按需渲染并释放 Blob URL/Canvas；不一次加载整份大试卷。
- 每一个保存动作具备 saving/saved/error/conflict；锁定冲突提示刷新或克隆新版本。

## Security, Tenant and Audit

- 模板、试卷、题目和考试必须属于同一租户和同一考试；数据库使用复合租户外键。
- `layout` 限制页数、区域数、JSON 大小、数值范围和重复 question_id，防止资源滥用与畸形坐标。
- 只信任服务端计算的配置指纹和 readiness；客户端展示结果不参与状态决策。
- 锁定模板、确认准备和开始采集记录 actor、request id、IP、配置 hash；审计中不保存原始试卷二进制。

## Error and Recovery

- PDF 渲染失败：保留模板元数据，提示下载检查或上传支持格式；不丢失已保存区域。
- 局部页面底图失败：允许切换其他页面并重试当前页。
- 保存冲突：返回 409 和当前版本，前端提示刷新或克隆，不覆盖其他管理员修改。
- readiness 不通过：返回 200 + `ready=false` 与完整 checks，属于业务结果，不伪装成系统错误。
- 配置在确认后变化：旧快照标记 invalidated，考试不能开始采集；用户可修复后重新确认。

## Acceptance Criteria

- [x] 管理员在 Exam Workspace 内完成考试配置，不需要重复选择考试。
- [x] 可上传/选择试卷版本并建立版本化答卷模板。
- [x] PDF/图片底图可视，管理员可用鼠标框选、移动和调整题目区域。
- [x] 模板坐标和题目关系保存到真实 API，刷新后恢复。
- [x] 锁定模板不可原地修改，可克隆为新版本。
- [x] 服务端 readiness 覆盖学生范围、试卷、题目、总分、答案、Rubric、模板和区域。
- [x] 配置不完整时无法进入准备完成或开始采集。
- [x] readiness 通过后记录快照、配置 hash、确认人和审计。
- [x] 确认后改变配置会使旧确认失效。
- [x] 跨租户/跨考试模板关联被拒绝，未知字段与畸形 layout 被拒绝。
- [x] Go test/vet、Web typecheck/build、静态门禁和真实浏览器流程通过。
- [x] Compose migration、API/Web 镜像和回滚说明验证通过。

## Out of Scope

- 自动识别定位点、自动配准、自动切题和采集批次，归 STORY-055。
- 阅卷员分配、校准、匿名规则、双评和抽检，归 STORY-058。
- QTI 导入导出、题库跨考试复用和多人 Rubric 审批工作流，不作为 V1.0 开考阻断项。
- 任意版式自动理解、复杂公式自动识别和无模板阅卷不在 V1.0 正式支持范围。

## Verification Plan

```text
Go: template/readiness handler + memory store + Postgres integration contracts
Web: typecheck, production build, STORY-054 static gate
Security: permission, tenant, cross-exam, unknown fields, malformed layout, locked conflict
Business: create paper/questions/answers/rubric/template -> readiness fail/pass -> confirm -> refresh -> start collection
Regression: STORY-049～053 checks and full Go tests
Deployment: migration 000026 on real PostgreSQL; rebuild API/Web; Compose health
Playwright: desktop and 390px, PDF/image template editor, readiness repair links, refresh context
```

## Plan Review

结论：原计划方向正确，但必须补齐以下边界后才能实现。

1. 不能继续使用现有 `validate-paper-config` 作为开考门禁。它只检查题目存在、分值和 `answer_area` 非空，没有检查试卷、答案、Rubric、模板、学生或状态绕过。
2. 不能把 `configured` 直接改文案当作“准备完成”。总控明确区分“配置中”和“准备完成”，需新增 `ready` 状态并保留历史语义。
3. 不能只在前端禁用按钮。通用状态 API 目前允许 `configured -> collecting`，必须在服务端移除绕过路径，开始采集走专用门禁接口。
4. 不能继续要求管理员编辑答题区域 JSON。V1.0 真实用户需要可视化框选，JSON 仅作为后端契约和高级诊断数据。
5. 不直接部署 TAO/Moodle 或完整 QTI 栈。它们解决更广泛的在线考试交付，会与当前纸质阅卷架构、权限和数据模型重复；只借鉴版本化、状态和互操作边界。
6. 不直接将 OMRChecker 模板格式作为数据库事实源。其模板针对特定 OMR 处理流程，本项目还需要主观题区域、身份区、页面配准和后续 worker 契约，采用项目自己的稳定 schema 更合适。
7. 配置确认必须可失效。只保存一个 `ready=true` 会在题目或模板变化后形成虚假准备状态，必须使用配置指纹并在开始采集前复验。
8. 答卷模板底图涉及受保护试卷文件，必须通过现有鉴权下载 Blob，在浏览器本地渲染，不生成公开 MinIO URL。

## Spec Fixes

- 将“完整性检查”升级为服务端 readiness 聚合与确认快照。
- 新增独立 `ready` 状态和专用 `start-collection` 门禁接口。
- 增加版本化答卷模板、归一化 layout schema、锁定/克隆和并发冲突。
- 增加可视化区域编辑器和按需 PDF.js 渲染，移除普通用户直接维护坐标 JSON 的主流程。
- 将配置指纹、失效规则、租户复合外键、资源上限、审计和回滚纳入规格。
- 明确 QTI/TAO/Moodle/OMRChecker 仅为技术参考，不引入第二套平台。

修订结论：Spec Ready。

## Implementation

- 数据库迁移 `000026_story054_exam_readiness.sql` 增加 `ready` 考试状态、版本化 `answer_sheet_template`、`exam_readiness_snapshot`、租户复合外键和关键配置变更失效触发器。
- Go API 实现模板创建、更新、锁定、克隆、准备检查、负责人确认和开始采集；通用状态接口不再允许绕过门禁进入 `ready` 或 `collecting`。
- readiness 由服务端聚合 8 项检查并计算稳定配置指纹；题目、答案、Rubric、试卷、班级、学生或模板变化会撤销旧确认。
- Web 在 Exam Workspace 内接入学生范围、试卷与题目、答卷模板、开考准备页面；工作区状态在配置、确认和开始采集后即时刷新。
- 答卷模板编辑器通过受保护文件接口下载 Blob，支持 PNG/JPEG 真实尺寸和 PDF.js 本地按页渲染，支持框选、移动、缩放、删除、保存、锁定和克隆。
- 修复既有 PostgreSQL 题目列表未回填最新标准答案和 Rubric 的问题，确保 readiness 使用真实完整数据。

## Implementation Review

审阅结论：首次实现功能边界正确，但真实生产浏览器验收发现并修复以下问题：

1. PDF worker 产物存在，但 Nginx 将 `.mjs` 返回为 `application/octet-stream`，浏览器拒绝作为 ES module 加载。Web Nginx 已为 `.mjs` 映射 `application/javascript`，真实 PDF 页面随后成功绘制。
2. PNG/JPEG 初版使用固定 A4 尺寸，可能使非 A4 图片的归一化坐标失真。现改为加载后读取 `naturalWidth/naturalHeight`，再建立页面布局。
3. 试卷上传和题目/Rubric 变化后，服务端状态已失效但工作区标题可能短暂保留旧值。现由 `PaperRubricPage` 回调刷新父工作区。
4. 390px 窄屏下长模板版本名称会裁切工具栏。现将移动端工具栏改为单列，并约束 Select 为容器宽度与省略显示。
5. PostgreSQL 锁定模板时 `jsonb_build_object` 参数类型推断失败。参数增加显式 `int/float8` 转换后，真实数据库锁定成功。

未阻断项：Ant Design 公共 chunk 仍超过 Vite 500 kB 提示阈值，页面已经按路由懒加载；进一步依赖拆分归入 STORY-061 性能与发布门禁，不影响本 Story 功能验收。

## Verification Evidence

- `go test ./...`：通过。
- `go vet ./...`：通过。
- Web `tsc --noEmit`：通过。
- Web production build：通过；PDF worker 独立构建为 `.mjs` 资产。
- PostgreSQL：迁移 000026 已在真实 Compose 数据库执行并登记。
- 真实业务流：1 个班级、1 名学生、2 个试卷版本、1 道 100 分主观题、标准答案、已锁定 Rubric、3 个模板版本。
- 图片模板：创建、保存、锁定、克隆通过；克隆后旧 readiness 自动失效，未重新确认时 `start-collection` 返回 409。
- PDF 模板：受保护 PDF 上传、1 页识别、本地渲染、Q1 框选、保存、锁定通过。
- readiness：8/8 检查通过；配置变化后从 `ready` 回落 `configured`；重新确认后进入 `ready`；专用门禁成功进入 `collecting`。
- Playwright：桌面 1280x720 与移动端 390x844 验收通过，无关键控件重叠；证据位于 `output/playwright/story054-*.png`。
- Compose：API、Web、PostgreSQL、MinIO、Redis、Qdrant、Nginx 均保持健康，生产入口为 `http://127.0.0.1:8088`。

## Approval

所有验收标准均已由自动化测试、真实 PostgreSQL/MinIO、生产 Docker 镜像和浏览器业务流覆盖。实现审阅发现的问题已修复并复验，STORY-054 批准完成。下一项为 STORY-055：答卷批次采集、模板识别、页面配准与按题切割。
