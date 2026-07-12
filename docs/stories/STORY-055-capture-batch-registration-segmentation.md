# STORY-055 采集批次、扫描导入、页面处理、模板配准和自动切题

## Status

Implementation In Progress

## Goal

把当前“一个文件创建一份单页答卷，再由用户逐份点击质量、OCR 和切题”的技术页面，升级为扫描员可独立操作的批次化生产流程：批量导入 PDF/PNG/JPEG/TIFF，安全拆页，组织答卷，匹配学生，执行真实图像质量与标准化，识别模板页，完成页面配准并生成真实答题区域图片；任何缺页、重复页、低置信匹配或配准失败都进入可恢复的人工处理队列，不得静默进入 OCR 和评分。

完成后的主流程：

```text
创建采集批次
-> 批量导入原始文件
-> 安全解码与拆页
-> 去重和页面排序
-> 组织为答卷
-> 学生/考号匹配
-> 图像质量检测与标准化
-> 试卷版本和页码识别
-> 模板配准
-> 按题切图
-> 人工处理低置信问题
-> 批次完成并移交 OCR
```

## User Outcomes

- 扫描员以“采集批次”为工作单位，能看到文件数、页数、答卷数、正常数、待确认数和失败数。
- 支持拖拽 PDF、多选图片和多页 TIFF；一个多页文件不再被错误当成一页答卷。
- 用户可追加、删除、旋转、重排页面，并可拆分或合并答卷；所有操作有撤销前确认和审计记录。
- 学生匹配采用答卷首页、识别信息和候选学生三栏工作区；低置信结果必须人工确认。
- 固定版式页面自动匹配试卷版本和页码，配准后按已锁定模板切出真实图片资产。
- 问题页集中进入待处理队列，修复后只重跑受影响页面和下游结果，不重做整个批次。

## Existing Capability and Gaps

可复用能力：

- `submission`、`submission_page`、MinIO `file_asset` 和租户复合约束。
- STORY-050 `image-quality-worker`、不可变质量 run、标准化 RGB PNG 和质量门禁。
- STORY-051 PostgreSQL-backed Worker Runtime 的 claim、lease、retry 和 dead-letter 语义。
- STORY-054 已锁定 `answer_sheet_template`、归一化题目区域和 readiness 门禁。
- 现有 `answer_segment`、OCR Worker 和受保护 Blob 下载。

必须补齐：

- 当前没有 `capture_batch`，上传队列只存在浏览器内存中，刷新后丢失。
- 当前一个文件直接创建一份 submission 并只关联第 1 页，不支持 PDF/TIFF 拆页。
- 当前 submission 在创建时即要求答卷语义，不能表达“尚未分组的批次页面”。
- 当前没有页面版本/页码识别、学生候选、缺页重复页诊断和人工匹配工作区。
- 当前切题只复制题目 `answer_area` 坐标，不执行 Homography，也不生成裁剪图片。
- 当前答案区域兼容旧 `w/h`，模板使用 `width/height`；必须统一边界适配，数据库事实源以模板 layout 为准。
- 当前前端逐份并发加载页面、OCR 和 segment，批量考试会产生 N+1 请求。

## Supported Scope

第一版正式支持：

- 已进入 `collecting` 的固定版式考试。
- PDF、PNG、JPEG、单页或多页 TIFF 文件导入。
- 一个文件一份答卷、连续多份答卷合并 PDF、散页图片批量导入。
- QR Code、Data Matrix、Code 128 中的受控 EduGrade 页面标识。
- 固定印刷锚点和 ORB/AKAZE 页面特征匹配。
- 已锁定答卷模板的页面配准和矩形题目区域切图。
- 人工学生匹配、页面归属、页码、旋转、排序、拆分、合并和配准确认。

明确不承诺：

- 任意未知试卷自动理解或无模板切题。
- 仅依靠手写姓名 OCR 自动确认学生。
- 通用 180 度内容方向识别达到零人工。
- 复杂非矩形区域、跨页单题自动拼接和公式结构解析。
- 真实扫描仪驱动兼容验收；Windows 扫描路径在 STORY-060 完成，当前以文件导入和模拟设备验收。

## Technical Route and Open-source Reference

### Control plane

- 保持 Go 模块化单体、PostgreSQL、MinIO 和现有 Worker Runtime，不引入 Kafka、Temporal 或新数据库。
- Go API 负责租户、权限、状态机、批次编排、幂等、审计、文件归属和人工操作；worker 不直写数据库。
- 业务记录与 worker task/outbox 在同一 PostgreSQL 事务创建，避免“页面已保存但任务未创建”。

### Page processing worker

- 新增 `services/page-processing-worker`，Python 3.11 + OpenCV headless + NumPy + Pillow + pypdfium2 + zxing-cpp Python binding。
- pypdfium2/PDFium 用于受限 PDF 页数检查和逐页渲染；其 API 支持按页渲染，且许可路线比强 copyleft PDF 绑定更适合私有化产品。
- Pillow 用于 JPEG/PNG 解码、EXIF 方向和多页 TIFF 帧遍历。
- ZXing-C++ 用于 QR/Data Matrix/Code 128；它支持 Python binding、线程安全和 Apache-2.0 许可。条码内容必须使用签名或校验码保护的 EduGrade schema，不能直接信任任意文本 ID。
- OpenCV ORB 作为默认无专利负担的局部特征，AKAZE 作为低纹理备选；KNN ratio test + RANSAC `findHomography` 估计 3x3 变换矩阵。
- 配准优先级：受控二维码/条码 -> 模板注册点 -> ORB/AKAZE 特征 -> 人工确认。OCR 页码只能作为候选证据，不能作为唯一依据。
- 参考 OMRChecker 的模板驱动、预处理流水线和可视化中间结果思想，但不嵌入其 CLI、模板 JSON 或自动评分逻辑；其项目也明确指出通用 auto-align 性能有限，因此本系统必须保留置信门禁和人工路径。

### Registration and cropping

- worker 使用标准化页面作为输入，把答卷页 warp 到模板页面尺寸；保存 registered RGB PNG，不覆盖原图或 STORY-050 标准化图。
- 保存 source-to-template、template-to-source 矩阵、特征数、inlier 数/比例、重投影误差、覆盖率、算法/profile/版本和耗时。
- 题目区域从锁定模板版本读取 `x/y/width/height`，在 registered page 上裁剪，生成独立 PNG file asset。
- 每个 segment 绑定 source page、normalized page、registered page、template/version、processing run、question/version 和 crop hash，确保申诉时可重现。

## State Machines

### Capture batch

```text
draft -> uploading -> matching -> processing -> needs_review -> ready -> completed
  |          |           |            |              |          |
  +----------+-----------+------------+--------------+----------+-> cancelled
```

- 状态由服务端根据聚合结果推进，前端不能任意写入。
- `completed` 后仍可追加补扫页面，但必须显式“重新打开批次”，记录原因并创建新 revision。

### Capture page

```text
uploaded -> decoded -> grouped -> quality_checking -> normalized
-> page_matching -> registration -> segmenting -> ready
```

异常状态：`needs_review`、`quality_rejected`、`failed`、`deleted`。人工修正后从受影响阶段重新调度。

### Submission matching

- `unmatched`：没有可靠学生候选。
- `suggested`：存在候选但尚未确认。
- `confirmed`：由签名条码高置信确认或人工确认。
- `unknown`：明确作为未知答卷保留，不允许进入成绩发布。
- `conflict`：同一学生多份答卷或标识冲突，必须人工处理。

## Data Model

迁移使用 `000027_story055_capture_processing.sql`，不得修改已应用的 000026。

### `capture_batch`

- `id`, `tenant_id`, `exam_id`, `name`, `source_type`, `status`, `revision`
- `operator_id`, `scanner_device`, `idempotency_key`
- `file_count`, `page_count`, `submission_count`, `normal_count`, `review_count`, `failed_count`
- `started_at`, `completed_at`, timestamps, soft delete
- 唯一约束：`tenant_id + exam_id + idempotency_key`（非空时）

### `capture_file`

- `id`, `tenant_id`, `capture_batch_id`, `file_asset_id`, `original_name`
- `content_type`, `sha256`, `byte_size`, `page_count`, `status`, `error_code`
- `idempotency_key`, `uploaded_by`, timestamps
- 同一批次相同 hash 默认识别为重复，允许用户明确保留副本但必须标记原因。

### `capture_page`

- `id`, `tenant_id`, `capture_batch_id`, `capture_file_id`, `source_index`
- `submission_id`, `submission_page_id`, `assigned_page_no`, `sequence_no`, `rotation_degrees`
- `decoded_file_asset_id`, `status`, `duplicate_of_page_id`, `revision`
- `page_identity`, `match_candidates`, `manual_override`, timestamps
- 页面属于批次后才可绑定 submission；移动、拆分、合并通过服务端事务更新。

### `page_registration_run`

- 不可变 run，包含 `source_file_asset_id/hash`、`template_id/content_hash`、`page_no`
- `processing_status`, `match_status`, `confidence`, `method`, `profile_version`
- `source_to_template_matrix`, `template_to_source_matrix`
- `feature_count`, `match_count`, `inlier_count`, `inlier_ratio`, `reprojection_error`
- `registered_file_asset_id`, lease/attempt/result version、worker、错误和耗时

### `answer_segment` upgrade

- 增加 `template_id`, `template_content_hash`, `registration_run_id`
- 增加 `normalized_bbox`, `pixel_bbox`, `crop_file_asset_id`, `crop_sha256`
- 增加 `question_version`, `processing_status`, `confidence`, `source`
- 唯一性升级为当前有效版本语义；旧结果保留历史并在源页、模板或题目版本变化时失效。

### `capture_operation`

- 记录 rotate/reorder/split/merge/delete/restore/match/override/reopen。
- 保存 actor、reason、before/after 摘要、request id、时间；不保存原始文件二进制。

## API Contract

### Batch and ingestion

- `POST /api/v1/exams/{examId}/capture-batches`
- `GET /api/v1/exams/{examId}/capture-batches`
- `GET /api/v1/capture-batches/{id}`（返回聚合摘要，避免前端 N+1）
- `POST /api/v1/capture-batches/{id}/files`（登记已上传 file asset，支持幂等键）
- `POST /api/v1/capture-batches/{id}/process`
- `POST /api/v1/capture-batches/{id}/reopen`
- `POST /api/v1/capture-batches/{id}/cancel`

### Page organization

- `GET /api/v1/capture-batches/{id}/pages`
- `PATCH /api/v1/capture-pages/{id}`（rotation/sequence/page assignment，revision 乐观锁）
- `POST /api/v1/capture-batches/{id}/submissions/split`
- `POST /api/v1/capture-batches/{id}/submissions/merge`
- `POST /api/v1/capture-pages/{id}/delete`
- `POST /api/v1/capture-pages/{id}/restore`

### Student and page matching

- `GET /api/v1/capture-batches/{id}/matching-queue`
- `POST /api/v1/submissions/{id}/student-match/confirm`
- `POST /api/v1/submissions/{id}/student-match/unknown`
- `POST /api/v1/capture-pages/{id}/page-match/confirm`
- 候选响应只包含当前考试范围内学生；姓名权限不足的角色返回脱敏候选。

### Registration and segmentation

- `POST /api/v1/submissions/{id}/process-pages`
- `GET /api/v1/submissions/{id}/processing-summary`
- `GET /api/v1/submission-pages/{id}/registration-runs`
- `POST /api/v1/page-registration-runs/{id}/confirm`
- `POST /api/v1/page-registration-runs/{id}/retry`
- `GET /api/v1/answer-segments/{id}/image`（受保护下载）
- 内部 worker 接口延续 Worker Runtime 的 claim/lease/heartbeat/result 契约，不另造无锁 pending 列表。

所有写接口拒绝未知字段，检查 tenant、exam、batch、submission、page、template 和 file asset 的同域归属，并记录审计。

## Matching and Confidence Rules

- 条码 payload 至少包含 schema version、tenant/exam、paper version、template version、page no、candidate token、nonce/checksum；生产密钥签名留到 STORY-061 完成，但本 Story 数据格式和验证接口必须就位。
- 条码校验失败只生成 `invalid_barcode` 问题，不回退为可信文本。
- 学号/姓名 OCR 只生成候选；自动确认必须满足唯一候选、考试范围、阈值和无冲突。默认对手写姓名不自动确认。
- 页面特征配准至少要求最小有效匹配数、最小 inlier ratio、最大重投影误差和合法四边形；阈值来自不可变 profile。
- 置信度分值必须伴随 method、evidence 和 threshold；禁止只保存一个无法解释的 0-1 数字。
- 缺页、重复页、未知页、多答卷同一学生、模板版本不一致和低置信配准全部阻止批次 `completed`。

## Idempotency, Consistency and Recovery

- 文件登记使用 batch idempotency key + SHA-256；重复请求返回同一 capture file。
- PDF/TIFF 拆页以 `capture_file_id + source_index + decoder_profile` 唯一，worker 重试不得生成重复页面。
- 页面操作使用 revision 乐观锁，冲突返回 409 和当前版本。
- 业务状态与 worker task/outbox 同事务；任务失败可从数据库事实重建。
- worker 结果以 run + attempt + result_version 幂等，旧 lease 不得覆盖新 attempt。
- 源文件、旋转、页序、submission 归属、模板或 profile 变化会失效受影响的 registration 和 segment，不删除历史。
- MinIO staged asset 只有在 hash、魔数、尺寸和 run 归属校验后 committed；未引用对象进入孤儿巡检。

## Frontend Product Design

### Capture batch list

- Exam Workspace 的“采集”页先显示批次列表，而不是单份答卷技术表。
- 顶部是创建批次、导入文件和刷新；主体是高密度表格，显示状态、来源、文件/页/答卷数、待确认、失败和时间。
- 业务状态使用“等待上传、正在匹配、正在处理、需要确认、可完成、已完成”，不暴露 task id、lease 或 dead letter。

### Batch workspace

- 顶部固定批次名称、考试、状态、真实进度和主要操作。
- Tabs：概览、文件与页面、学生匹配、处理问题、答卷结果。
- 文件导入进度来自服务端，刷新/换电脑可恢复；失败项支持重试和替换。
- 页面整理使用缩略图条带与明确的旋转、前移/后移、拆分、合并、删除图标按钮。

### Matching workspace

- 左：答卷首页/当前页；中：条码、学号、姓名和页码证据；右：候选学生。
- 主操作：确认、跳过、标记未知；支持上下项和 Enter 连续确认，焦点可见且无键盘陷阱。
- 不依赖颜色表达风险，显示原因、置信度和需人工操作。

### Processing review

- 原图/标准化图/配准图可切换对比，叠加模板边界和题目区域。
- 失败信息回答发生什么、影响哪些页面、如何修复和是否可重试。
- 大图按需 Blob 加载，缩略图分页或虚拟化；不得一次下载全考试原图。

## Security, Privacy and Audit

- `capture:manage` 管理批次，`submission:manage` 管理答卷，`segment:manage` 处理切题；姓名候选另需组织姓名读取权限。
- 扫描员仅能操作授权考试，不能通过请求体指定其他学校、考试或学生。
- 文件下载继续走鉴权 API，不生成公开 MinIO URL。
- PDF/TIFF 设置文件大小、页数、像素、解压后总量、处理时间和内存上限，防止压缩炸弹与资源耗尽。
- 禁止将真实答卷文件名、学生姓名、条码内容和图像写入普通日志。
- 自动匹配、人工确认、拆分合并、旋转删除、配准 override 和批次完成全部写审计。

## Error and Recovery

- 文件不可解码：文件级失败，其他文件继续处理；可替换后重试。
- Worker 不可用：批次显示“等待处理”，保留任务；不得误报图像不合格。
- 质量 review：进入处理问题，人工放行仍保留原始质量结论。
- 页面无法匹配：允许人工选择试卷版本和页码，再重跑配准。
- 配准失败：显示原图和模板，允许人工确认四角/锚点；不生成伪造 crop。
- 切图失败：只重跑该页 segments，不重复创建 submission 或 OCR 任务。
- 浏览器中断：服务端批次与上传结果保留，重新进入可继续。
- 并发编辑：409 冲突，前端提示刷新或重新应用，不覆盖他人调整。

## Acceptance Criteria

- [ ] 扫描员可创建采集批次并在刷新后继续。
- [ ] PDF、多图片和多页 TIFF 可安全拆页，原始文件不被覆盖。
- [ ] 同一文件重复登记保持幂等并提示重复。
- [ ] 可追加、旋转、重排、删除/恢复、拆分和合并页面。
- [ ] 可人工匹配学生并处理未知或冲突答卷。
- [ ] 图像质量 worker 对每个当前页面真实运行，标准化资产可追溯。
- [ ] 页面按条码、锚点或特征匹配得到试卷版本与页码；低置信结果进入人工确认。
- [ ] OpenCV Homography 真实执行并保存矩阵、inlier 和误差证据。
- [ ] 已锁定模板区域生成真实 PNG crop，不只是 bbox 元数据。
- [ ] 源页、旋转、模板或题目变化会使旧配准和切图失效。
- [ ] 缺页、重复页、未知学生、低置信配准和失败任务阻止批次完成。
- [ ] API 权限、跨租户/跨考试归属、未知字段和状态机测试通过。
- [ ] Go/Python/Web 测试、生产构建、Compose 迁移和 Worker 重启恢复通过。
- [ ] Playwright 真实完成批次创建、导入、匹配、处理问题、配准、切图和批次完成。
- [ ] 桌面与 390px 移动端无关键重叠；扫描员核心批量操作以桌面端为正式支持体验。

## Verification Plan

```text
Go: batch/page/matching/registration/segment store + handler + tenant/status/idempotency tests
Python: PDF/TIFF decode, QR, ORB/AKAZE homography, crop, limits, retry/idempotency tests
Web: typecheck/build, batch workflow, conflict/error/loading/empty/partial states
Database: migration 000027, composite FKs, indexes, invalidation, rollback rehearsal
Compose: API/Web/page-processing/image-quality/MinIO/PostgreSQL real flow
Playwright: desktop full flow and 390px review; refresh/resume and partial failure
Fixtures: normal, skewed, blurred, missing, duplicate, wrong page, invalid barcode
```

## Plan Review

规格审阅发现并纠正以下风险：

1. 不能直接扩展现有“一个文件一份 submission”上传函数。多答卷 PDF 和散页在分组前没有 submission 语义，必须先落到 capture batch/file/page，再事务绑定答卷。
2. 不能把 STORY-050 质量标准化等同于模板配准。质量 worker 只保证可处理图，Homography 和模板证据必须使用独立不可变 registration run。
3. 不能继续由 Go handler 读取 question.answer_area 并复制 bbox。切图需要真实 registered asset、模板版本和 crop file asset，由 Python worker 执行。
4. 不能只采用 OCR 页码或姓名。正式优先级必须是受控条码、注册点、特征匹配、人工确认；OCR 仅提供候选。
5. 不能直接嵌入 OMRChecker。其模板和自动评分范围与本项目主观题、租户、审计和 Worker Runtime 不一致，只借鉴模板驱动和中间结果可视化。
6. 不能使用 PyMuPDF 作为默认 PDF 路线而忽略许可边界；继续采用项目已验证的 pypdfium2/PDFium。
7. 不能让每个页面自己排队并由前端拼进度。批次聚合、任务创建和状态推进必须由服务端事务与 worker runtime 驱动。
8. 不能在低置信时“取最高候选”静默自动确认。所有阈值外结果必须进入明确人工队列并阻断批次完成。
9. 不能只保存最终 crop。原图、标准化图、配准图、矩阵、模板 hash、题目版本和 crop hash 都是申诉与重现所需证据。
10. Windows 扫描仪是外部硬件条件，不能伪造验收；本 Story 完成文件导入和扫描模拟器契约，真实设备验收留到 STORY-060。

## Spec Fixes

- 引入 capture batch/file/page 三层摄取模型，把“文件”与“答卷”解耦。
- 明确新增独立 page-processing worker，并复用现有质量 worker 和 Worker Runtime，而不是复制队列。
- 将真实页面配准、registered asset 和 crop asset 纳入正式数据链路。
- 增加学生/页面匹配证据、阈值、人工队列和批次完成阻断规则。
- 增加事务 outbox、幂等、revision 冲突、源变更失效和 MinIO staged/committed 规则。
- 增加桌面批次工作区、三栏匹配、处理问题页及服务端聚合 API，消除当前 N+1 页面模式。
- 明确开源组件许可与职责边界，并记录无真实扫描仪时的可验证范围。

修订结论：Spec Ready，可以进入实现。

## Quality Auto-Chaining Verification (2026-07-11)

The first implementation blocker is closed and was verified through the public capture APIs and real Compose workers:

- File decode now creates one image-quality run and one Worker Runtime task per capture page in the same database transaction.
- Capture pages enter `quality_checking`; `passed` moves to `normalized`, while `review`/`failed` blocks registration and enters the issue path.
- Registration selects `submission_page.normalized_file_asset_id` only. It cannot fall back to `capture_page.decoded_file_asset_id`.
- Normalized uploads carry the submission exam ID, and registration continues to enforce tenant, exam, asset hash, and file ownership checks.
- Migration `000030_story055_capture_quality_source_asset.sql` permits tenant-scoped decoded-asset reuse before submission assignment while retaining cross-tenant protection.
- Image-quality Worker Docker layers now cache OpenCV, NumPy, and Pillow independently from application source changes.

Real acceptance batch: `ef89f64a-f6fe-495f-bf3e-554f94a5c9a3`.

- Page 1: quality `passed`, registration `completed/matched`, confidence `1.0`, registration source equals normalized asset, and one real segment crop was materialized.
- Page 2: quality `review`, capture page `needs_review`, and no registration run was created.
- API Gateway, image-quality Worker, and page-processing Worker remained healthy after rebuilding and restarting.

The remaining implementation order is now: student/page matching -> split/merge/delete/restore -> registration manual confirm/retry -> completion-gate Playwright -> implementation review -> Approved.

## Candidate and Page Matching Verification (2026-07-11)

This slice follows the first-principles rule that a physical answer script must be bound to exactly one eligible student and one page number through explainable, reversible decisions:

- The candidate set is derived only from `exam_class -> active student`; tenant-wide student search is not accepted as an exam roster.
- Migration `000031_story055_candidate_page_matching.sql` adds explicit identity status, optimistic revision, evidence, and one-active-submission-per-exam/student constraints.
- Manual confirmation submits only a roster student ID. The server resolves the authoritative name and student number and records actor/reason evidence.
- Unknown answer scripts are a first-class blocking state. Duplicate student/candidate bindings and stale revisions return HTTP 409.
- Manual page-number confirmation is revisioned and invalidates previous registration and segment results.
- Batch aggregation keeps unassigned identities in `matching`, unknown/conflict identities in `needs_review`, and allows `ready` only after identity and page gates pass.
- The Web batch workspace now provides a three-column answer-script, page-evidence, and exam-roster workflow with desktop and 390px responsive layouts.

Real verification used batches `ef89f64a-f6fe-495f-bf3e-554f94a5c9a3` and `7c803f65-b2f5-4f53-b84e-7778ea5367bf`: one roster candidate was returned, the first script was matched, a cross-batch duplicate returned HTTP 409, another script was marked unknown, and page confirmation advanced its revision from 2 to 3.

The remaining implementation order is now: split/merge/delete/restore -> registration manual confirm/retry -> completion-gate Playwright -> implementation review -> Approved.

## Reversible Page Organization Verification (2026-07-11)

The page-organization slice is complete under the invariant that no physical page or historical evidence is destroyed:

- Delete/restore changes a revisioned business status; original assets and database rows remain available.
- Split moves selected active pages into a new unassigned submission and rejects moving every page out of the source.
- Merge is restricted to submissions in the same tenant/batch/exam workflow and renumbers active pages deterministically.
- Moving a page creates a new active `submission_page`; the historical page remains soft-deleted because immutable quality runs reference it.
- Normalized assets are not copied across submissions. The moved page returns to unchecked quality and must regenerate normalization, registration, and crops.
- All affected registration runs and answer segments are invalidated, and delete/restore/split/merge operations are audited with actor and reason.
- Web controls expose familiar delete, restore, split, and merge icons in the batch workspace.

Real verification on batch `7c803f65-b2f5-4f53-b84e-7778ea5367bf` produced one audit record for each operation. A two-page submission became two one-page submissions after split and returned to one two-page submission after merge.

The remaining implementation order is now: registration manual confirm/retry -> completion-gate Playwright -> implementation review -> Approved.

## Registration Review and Retry Verification (2026-07-11)

- Processing summary reports ready, blocked, and pending pages with one explicit next action per blocker.
- Human confirmation is allowed only for completed low-confidence runs with a registered asset; it preserves algorithm confidence and records actor, time, and reason.
- Failed/dead-letter registration can be requeued through Worker Runtime; the same immutable run receives a new runtime attempt instead of overwriting history.
- The Web processing tab displays server-derived counts and offers confirm/retry only when a blocker has a valid registration run.
- Real retry verification used run `ad9d2910-8e60-431d-9996-0d924b00ebb3`: the summary returned `retry_registration`, and runtime attempt count advanced from 3 to 4 before the intrinsically invalid fixture returned to dead letter.

The remaining implementation order is now: completion-gate Playwright -> implementation review -> Approved.

## Completion Gate Verification (2026-07-11)

- Batch aggregation now counts only active pages. A logically deleted historical page remains visible and restorable but no longer inflates page/submission counts or blocks readiness.
- A batch becomes `ready` only when it has at least one active page, every active page is `ready`, and every active submission has a matched student identity.
- The Web workspace exposes an explicit, confirmed "完成批次" command only in `ready`; the API still enforces the authoritative `ready -> completed` transition.
- Migration `000033_story055_capture_complete_operation.sql` adds the missing `complete` operation to the transactional capture audit constraint.
- Real Playwright verification used batch `ef89f64a-f6fe-495f-bf3e-554f94a5c9a3`: deleting its blocked second page changed active pages from 2 to 1 and review count from 1 to 0, exposed the completion command, and completion persisted with exactly one `complete` operation record.

Implementation review remains open. Completion must not be marked Approved until the remaining gaps listed below are closed and re-reviewed.

## Completed Batch Read-Only Verification (2026-07-11)

- PostgreSQL write paths now lock and validate the authoritative capture batch inside the same transaction before file registration/queueing, page mutation, delete/restore, split/merge, student/page matching, registration queueing, confirmation, or retry.
- `completed` and `cancelled` reject writes with the existing invalid-transition contract; a completed batch can be changed only after an explicit audited reopen transition.
- The in-memory store follows the same rule for file registration, queueing, and page updates, with regression coverage.
- The Web workspace keeps evidence preview available but disables import, processing, page organization, matching, and registration actions for completed/cancelled batches.
- Direct API bypass verification against completed batch `ef89f64a-f6fe-495f-bf3e-554f94a5c9a3` returned HTTP 409 for page rotation, deleted-page restore, and student identity mutation. Batch/page revisions and completed status remained unchanged.

## Controlled Barcode Contract

The barcode is evidence for page/template identity, not authority by itself. ZXing only reports what pixels contain; the API makes every trust decision.

- Wire format: `EG1.<base64url(canonical-json)>.<base64url(hmac-sha256)>`.
- Required claims: `v=1`, `kid`, `tenant_id`, `exam_id`, `template_id`, `template_content_hash`, `page_no`, and a random `nonce`.
- Canonical JSON uses UTF-8, sorted keys, no insignificant whitespace, and integer page numbers. Unknown claims are retained in raw evidence but do not change validation semantics.
- Keys are selected by `kid`, supplied from deployment secrets, and never sent to Workers or browsers. Rotation accepts configured active/verification keys; new issuance uses only the active key.
- The API validates signature in constant time, version, required claims, UUIDs, positive page number, tenant/exam ownership, locked template identity, and exact template content hash.
- Worker observations contain symbology, raw text, polygon, orientation, and decode error only. Raw values are bounded in count and length before persistence.
- A valid observation produces an explainable `controlled_barcode` page candidate. Invalid observations persist only a normalized rejection code such as `signature_invalid`, `exam_mismatch`, or `template_hash_mismatch`; secrets and raw stack errors are never exposed.
- One valid unambiguous candidate may assign the page number. Multiple conflicting valid barcodes, any claim mismatch, or page-number disagreement enters `needs_review`; it must never silently choose the first result.
- Barcode evidence is additive. Registration still verifies the locked template and produces independent Homography evidence before segments are accepted.

Spec review: the contract deliberately excludes student identity and scores, so copied blank sheets cannot bind a submission to a student or alter grading. It also avoids public-key complexity until offline third-party printing is a real requirement; deployment-managed HMAC with `kid` rotation is sufficient for the current server-issued template workflow.

## Controlled Barcode Verification (2026-07-11)

- The API loads an active signing key and rotation-compatible verification keyring from dedicated deployment secrets. Production startup rejects a missing/short active key.
- `POST /api/v1/answer-sheet-templates/{id}/page-barcodes` accepts no client claims and issues one independently nonced barcode per page from the authoritative locked template layout.
- Page-processing Worker uses ZXing-C++ to return bounded text, symbology, polygon, and orientation observations. The API revalidates callback bounds before entering the transaction.
- `ApplyFileResult` verifies HMAC and tenant/exam/template/hash/page ownership, persists only hashed raw values plus normalized evidence, and creates explainable `controlled_barcode` candidates.
- One valid unambiguous candidate assigns the page number. A controlled barcode with invalid signature/claims or conflicting valid candidates remains blocked after quality processing until manual page confirmation records an override.
- Real signed-template verification used locked template `522e54c3-77d6-4b12-b347-89d776e19826`. Batch `895aec29-60cf-4243-93d7-6e5ff45056aa` decoded a real QR PNG as `verified` with one page-1 candidate. Tampered batch `905ef3b3-9c1f-48ae-81d3-dc72af96f6b0` persisted `signature_invalid`, produced zero candidates, and ended in `needs_review`.

## STORY-055A Manual Registration Correction Specification

### Product outcome

A scanner operator must be able to recover a page that automatic template positioning cannot align, without understanding Homography and without allowing an unchecked preview to become grading evidence. The workflow is desktop-first and preserves every automatic and manual decision.

### User workflow

1. From a registration blocker, the operator opens "校正页面边界".
2. The workspace displays the normalized source page and authoritative locked-template page side by side.
3. Four labelled points are shown in stable order: top-left, top-right, bottom-right, bottom-left. Source points are editable; template points default to template corners and are editable only in an explicit advanced-anchor mode.
4. Zoom, pan, fit, reset, and local magnification support pixel-accurate placement. Coordinates are normalized to each image, so browser size never changes the submitted geometry.
5. "预览校正" creates an asynchronous isolated preview. It does not update the active registration run, capture page, answer segments, or batch gate.
6. The preview overlays template boundaries and question regions and reports user-facing checks: boundary complete, orientation valid, question regions in bounds, and alignment quality acceptable. Technical matrix/coverage values remain available under details.
7. Only a ready preview that passes hard geometry checks can be applied. Apply requires a reason and the same capture-page revision used to create the draft.
8. Apply atomically creates a new immutable manual registration run, promotes the preview registered asset and crops, invalidates the previous active run/segments, updates the page/batch state, and records actor/reason/before/after evidence.
9. The latest applied correction can be undone while the batch is writable. Undo restores the exact prior active registration/segment snapshot and invalidates the correction result; it never reruns a possibly changed algorithm.

### Coordinates and validation

- A point is `{x,y}` with finite values in `[0,1]`, interpreted against the exact source/template dimensions stored in the correction draft.
- Exactly four unique source and four unique template points are required in clockwise order.
- The polygons must be convex, non-self-intersecting, and have at least 5% of their image area.
- The derived matrix must be finite and invertible; mirrored orientation, condition number above the configured limit, target coverage outside `[0.70,1.15]`, or any question region outside the registered image is a hard failure.
- Preview returns normalized error codes, never OpenCV stack text. Invalid geometry is rejected before a Worker task is created when possible and rechecked in Python before processing.
- Limits: one active draft per page/operator, at most eight preview attempts per page revision, 15-minute draft expiry, and the existing image pixel limits.

### Persistence and states

Add immutable `page_registration_correction` records:

```text
id, tenant_id, capture_page_id, base_registration_run_id, source_page_revision,
template_id, template_content_hash, page_no,
source_points, template_points, status,
preview_registered_file_asset_id, preview_segments, source_to_template_matrix,
template_to_source_matrix, coverage, reprojection_error, validation_report,
runtime_task_id, created_by, applied_by, reason,
previous_registration_snapshot, expires_at, applied_at, undone_at, created_at, updated_at
```

States are `draft -> queued -> preview_ready -> applied -> undone`; `draft/queued` may become `failed` or `expired`, and a newer applied correction marks older drafts `superseded`. State changes use optimistic revision and database transactions.

Preview assets use owner type `page_registration_correction_preview`. They are protected but never returned by the segment image API as active grading evidence. Retention cleanup may remove expired, unapplied preview assets only after the configured evidence window.

### API contract

```text
POST /api/v1/page-registration-runs/{id}/corrections
GET  /api/v1/page-registration-corrections/{id}
POST /api/v1/page-registration-corrections/{id}/preview
POST /api/v1/internal/page-registration-corrections/{id}/result
POST /api/v1/internal/page-registration-corrections/{id}/failure
POST /api/v1/page-registration-corrections/{id}/apply
POST /api/v1/page-registration-corrections/{id}/undo
```

- Create accepts page revision, source/template points, advanced-anchor flag, and no asset/template identity supplied by the client.
- Preview is idempotent by correction revision and points hash. Duplicate requests return the existing task/result.
- Result callbacks require Worker Runtime lease ownership and validate every uploaded asset against tenant, exam, owner, hash, and expected dimensions.
- Apply and undo require `capture:manage`, a non-empty reason, writable batch, current page revision, and one transaction spanning run/segment/page/batch/audit changes.
- GET returns source/template download references through authorized APIs, dimensions, points, status, validation report, and preview references; it never exposes object-store URLs.

### UI and accessibility

- The correction editor is a full-width operational workspace, not a nested modal. Leaving with unsaved point changes requires confirmation.
- Point handles have colour plus shape/label distinctions, a minimum 24px visual target and 44px hit area, keyboard arrow adjustment, and an accessible point list with numeric inputs.
- A clear source/template legend and linked point numbering prevent correspondence mistakes.
- Errors use operational language: "页面边界交叉", "页面方向可能翻转", "校正后题区超出页面". Matrix terms appear only in expandable details.
- Mobile permits evidence viewing and status actions but directs precision correction to desktop.

### Audit, concurrency, and recovery

- Create, preview request/result, apply, undo, failure, expiry, and supersede events carry request/trace IDs where available.
- Worker/API restart resumes queued previews through Worker Runtime. Duplicate result callbacks are idempotent.
- Rotating, deleting, restoring, splitting, merging, changing page number, or completing/reopening the batch invalidates or supersedes incompatible drafts.
- Apply versus page mutation and apply versus batch completion serialize on the capture-batch/page rows. A stale operation returns HTTP 409 and preserves the preview for inspection.

### Acceptance criteria

- [ ] A low-texture or strongly perspective-distorted fixture can be corrected from four points and produces the expected registered image and real question crops.
- [ ] Invalid, crossed, mirrored, duplicate, tiny, out-of-range, NaN, and stale points cannot create active evidence.
- [ ] Preview never changes active registration, segments, page status, or completion counts.
- [ ] Apply changes run/segments/page/batch exactly once and writes one business audit event.
- [ ] Undo restores the exact previous registration and segment hashes and writes one audit event.
- [ ] Source rotation or page revision between preview and apply returns 409 with no partial changes.
- [ ] Completed/cancelled batches reject create, preview, apply, and undo.
- [ ] Cross-tenant/cross-exam correction and asset access are rejected.
- [ ] Worker restart, duplicate callback, preview failure, and retry converge without duplicate active crops.
- [ ] Desktop Playwright completes create -> adjust -> preview -> apply -> undo; 390px shows evidence and the desktop-required message without overlap.

### Specification review and fixes

1. A synchronous preview API was rejected because Homography, image upload, and crop generation can exceed HTTP timeouts; preview uses Worker Runtime.
2. Applying only a matrix was rejected because OCR needs immutable registered/crop assets tied to that exact result; preview produces isolated assets and apply promotes them transactionally.
3. Recomputing on undo was rejected because algorithms/templates may change; apply stores an exact previous evidence snapshot for deterministic restoration.
4. Pixel coordinates were rejected because responsive rendering and source normalization change dimensions; persisted coordinates are normalized and dimension-bound.
5. Client-supplied template IDs/hashes were rejected; the API derives them from the base run and locked template.
6. A canvas-only editor was rejected for accessibility; keyboard and numeric point controls are part of the required UI.
7. Allowing previews to clear blockers was rejected; only apply may change active business state.

Specification review conclusion: Ready for implementation. The first implementation slice is schema/domain validation, followed by Worker preview, transactional apply/undo, and the desktop workspace.

### Manual correction implementation progress (2026-07-11)

- Migration `000034_story055_manual_registration_correction.sql` adds the correction state model, active-draft uniqueness, preview limits, Worker task type, and correction audit operations.
- Go and Python independently reject invalid, duplicate, crossed, mirrored, tiny, non-finite, and out-of-range quadrilaterals.
- Authorized create/get/preview APIs bind the draft to the authoritative base run, locked template, page number, page revision, batch state, actor, expiry, and points hash.
- `page-processing-worker` executes isolated `manual_four_point` previews, uploads registered/segment assets under `page_registration_correction_preview`, and returns bounded matrix/coverage validation evidence through lease-bound callbacks.
- Callback validation checks tenant, exam, correction owner, and content hash before the correction can become `preview_ready`.
- Real preview correction `c5b8f054-099d-4698-822c-fa71c5889892` completed with coverage `0.9999932075839946`, one registered preview and one crop. The capture page revision remained `2`, and its page/batch blockers remained unchanged.

Remaining in STORY-055A: transactional apply with previous-evidence snapshot, deterministic undo, stale/concurrent rejection tests, and the desktop correction workspace. This slice is not yet Approved.

### Manual correction apply/undo verification (2026-07-11)

- Apply locks correction, capture page, and batch; requires the bound page revision and latest base run; snapshots only the affected submission page's active run/segment evidence.
- One transaction creates an immutable `manual_four_point` registration run, promotes preview registered/crop assets, invalidates the prior run, updates page/batch gates, and writes one `registration_correction_apply` operation.
- Undo is allowed only before any subsequent page mutation. It invalidates the manual run/crops, restores exact prior run status and segment bbox/crop hashes from the snapshot, advances page revision, and writes one `registration_correction_undo` operation.
- Automatic registration completion/failure now advances capture-page revision, so a correction preview cannot be applied across a concurrent processing result.
- Real correction `c5b8f054-099d-4698-822c-fa71c5889892` applied as run `84460318-e160-442c-a2dd-3ae280a043f7` and then undid successfully. The base run returned to `terminal_error`, the page returned to `needs_review`, the manual segment became `invalidated`, and apply/undo operation counts were exactly one each.

Remaining in STORY-055A: explicit stale/concurrent API acceptance cases and the desktop correction workspace with Playwright. This slice is not yet Approved.

### STORY-055A final acceptance (2026-07-12)

- Added an authorized correction-context API that derives the normalized source, locked template page, exact dimensions, and authoritative exam ownership from the active registration run. Legacy normalized assets without a denormalized `exam_id` remain usable because ownership is verified through the tenant-bound run/template/exam relationship; preview assets still require exact exam, owner, correction ID, and SHA-256 matches.
- Added the full-width desktop correction workspace: source/template side-by-side rendering, four labelled draggable points, keyboard arrows, numeric coordinates, zoom, reset, opt-in template anchors, asynchronous preview, validation status, apply, and undo.
- Registration actions and the correction entry are independent. Any active registration blocker with a run ID keeps the correction entry, including after an apply/undo cycle.
- All latest-run lookups used by processing summary, correction context, and correction creation now select the latest non-deleted, non-`invalidated` run. An undone manual run can no longer hide the restored base blocker.
- Apply clears the reason before undo so rollback requires a separate operator reason. Applied point controls are read-only. At 390px, canvases, point controls, reason input, reset, and mutation commands are hidden; the operator sees status/preview plus a desktop-required notice.
- Real Playwright acceptance used base run `ad9d2910-8e60-431d-9996-0d924b00ebb3`. Preview reached `preview_ready` with 100% coverage and a real registered image/crop. Correction `5da94e2b-a5ae-4f0e-b243-884006955adf` applied as run `10c49429-9610-446d-8cc5-57676162eee4`, then undid through the UI.
- Database acceptance confirmed one apply audit and one undo audit, page revision advancement, base-run restoration to `terminal_error`, manual run and crop evidence at `processing_status=invalidated`, and processing-summary restoration to one registration blocker referencing the base run.
- Go `go test ./...` and `go vet ./...`, Web typecheck/build, Compose health checks, desktop Playwright, and 390px Playwright passed. Optimistic correction/page revisions and transactional batch/page locks cover stale apply, concurrent page mutation, and completion races with HTTP 409 and no partial state.

STORY-055A is complete. STORY-055 remains open: proceed with 055B Segment Image/Evidence API, 055C TIFF and malformed-input production matrix, 055D continuous issue-handling UX, then 055E cross-layer E2E/performance/review before Approved.

## STORY-055B Segment Image/Evidence API (2026-07-12)

### Specification and review

- Consumers address an answer segment, never a general file asset or MinIO key: `GET|HEAD /api/v1/answer-segments/{id}/image` and `GET /api/v1/answer-segments/{id}/evidence`.
- The API resolves tenant, submission/exam, question/template version, registration run, bbox, crop hash, and file asset server-side. No object-store URL or crop asset ID is exposed.
- An image is readable only when both segment and registration run have `processing_status=completed`. Invalidated, missing, legacy-without-crop, or deleted evidence returns HTTP 409; unknown/cross-tenant IDs return 404.
- File metadata must match the evidence exam and SHA-256, have positive size and `image/*` content type, and have an exact allowed owner: automatic crops bind to the registration run; manual crops bind to the currently applied correction.
- Read permissions are available to segment, OCR, grading, evidence, review, and arbitration workers/operators. Mutation permissions are unchanged.
- The image response is streamed by the API from private object storage with `nosniff`, inline disposition, immutable private cache policy, a SHA-256 ETag, HEAD support, and conditional 304. GET image access is audited; HEAD/304 do not create high-volume view audits.

Rejected alternatives: exposing signed MinIO URLs would duplicate authorization and complicate revocation; redirecting through `/files/{id}` would expose implementation IDs and allow callers to bypass active-evidence checks; embedding image bytes in evidence JSON would increase memory and network overhead.

### Implementation and acceptance

- Added `SegmentEvidence` and a tenant-bound PostgreSQL evidence projection joining submission, active segment, registration run, and applied manual correction.
- Added server-side active-evidence validation and streaming handlers without buffering the crop in API memory.
- Real automatic segment `bbcceaf2-c997-4e3f-a292-0553befa5f3b` returned evidence 200 and a 79,124-byte `image/png`; its ETag matched the persisted crop SHA-256, conditional GET returned 304, and HEAD returned 200 with the same length.
- Real undone manual segment `1ec955e5-ea44-493b-b8e4-9a08fd37a9cb` returned HTTP 409 `answer_segment_evidence_unavailable` and no image bytes.
- API `go test ./...` and `go vet ./...` passed before deployment. Compose API/nginx health and authenticated browser API acceptance passed.

STORY-055B is complete. Next: STORY-055C TIFF and malformed-input production matrix.

## STORY-055C TIFF and Malformed Input Matrix (2026-07-12)

### Specification and review

- Binary signature detection takes precedence over declared MIME. A TIFF uploaded as `application/pdf` is decoded as TIFF; declared TIFF with invalid bytes receives `invalid_tiff`.
- TIFF frames are validated before copying/materialization. Per-page pixels, cumulative document pixels, and frame count are bounded independently; a limit failure closes already copied frames and creates no capture pages.
- Supported production classes are bilevel Group 4, grayscale LZW, RGB Deflate, and mixed multi-frame TIFF. Every accepted frame is normalized to RGB PNG before quality, identity, registration, and segmentation.
- Corrupt/truncated TIFF warnings and decoder exceptions are converted to stable domain errors. Pillow/OpenCV traces and parser warnings are not exposed to operators.
- A terminal file failure must atomically update the file and aggregate the batch to `needs_review`; retryable failures keep the batch in `processing`. No terminal failure may leave a batch indefinitely processing.

### Implementation and acceptance

- Refactored `decoder.py` to sniff PDF by magic, otherwise inspect images with Pillow, validate TIFF frames before materialization, close partial results on failure, and normalize TIFF failures to `invalid_tiff`.
- Expanded the Worker suite to 18 passing tests covering Group 4/LZW/Deflate, 1-bit/L/RGB, multi-frame order, MIME mismatch, frame/page/total limits, truncated TIFF, invalid image, PDF, JPEG, and PNG normalization.
- Real Compose batch `f416283d-118a-4157-85f7-ab7cc530d80f` uploaded a 52,912-byte two-page LZW TIFF. The file completed with `page_count=2`; pages 1 and 2 materialized as independent PNG assets of 10,715 and 10,773 bytes. The later `needs_review` state came from page identity/template workflow, not decoding.
- Real terminal-failure batch `317fdc68-79f5-4a31-8bc0-0bb1f6b0516b` uploaded a 64-byte truncated TIFF. It converged to `batch=needs_review`, `failed_count=1`, `file=failed`, `error_code=invalid_tiff`, and zero pages.
- Go capture/server tests and vet plus all 18 Python tests passed. API and Worker images were rebuilt and verified healthy in Compose.

STORY-055C is complete. Next: STORY-055D continuous issue handling, completion, and reopen UX.

## STORY-055D Continuous Issue Handling and Lifecycle UX (2026-07-12)

### Specification and review

- The issue count and issue content must be derived from the same sources. Failed files and review/quality/failed pages are both first-class queue items; an operator must never see “1 issue” with an empty panel.
- File failures show the source filename and stable error code. The primary recovery command returns the operator to the file area to upload a replacement while preserving the failed record for audit.
- Page issues reuse the existing evidence preview, rotation/delete, matching, registration retry/confirm, and manual correction commands. Resolving an issue refreshes authoritative batch/processing summaries rather than mutating counters locally.
- Batch completion is offered only in authoritative `ready` state and makes every mutation path read-only. A completed batch exposes a dedicated reopen command only to managers.
- Reopen requires a new non-empty operator reason and uses the audited backend transition. It is not combined with refresh or edit controls, and an empty reason never calls the API.

### Implementation and acceptance

- Added a controlled capture-workspace tab state so issue actions can take the operator directly to the relevant file workflow.
- Added failed-file issue rows with source name, normalized error code, and replacement action; page issue tables remain in the same queue below file failures.
- Added the completed-batch reopen command and reason modal, plus a typed Web API client for the existing audited endpoint.
- Real Playwright verification on terminal-failure batch `317fdc68-79f5-4a31-8bc0-0bb1f6b0516b` showed one issue containing `truncated.tiff`, `invalid_tiff`, and the replacement command instead of an empty panel.
- Real completed batch `ef89f64a-f6fe-495f-bf3e-554f94a5c9a3` remained read-only, displayed “重开批次”, and retained the modal when confirmation was attempted without a reason.
- Web typecheck and production build passed; deployed Web/API/Worker/nginx remained healthy.

STORY-055D is complete. Next: STORY-055E cross-layer risk E2E, performance smoke, implementation review, and final fixes.

## Out of Scope

- OCR 结果编辑和客观题评分，归 STORY-056。
- 主观题真实模型和证据门禁，归 STORY-057。
- 阅卷员分配、匿名、双评、仲裁和质量中心，归 STORY-058。
- Windows TWAIN/WIA/SANE 设备适配和离线同步，归 STORY-060。
- 100 人完整 UAT、性能容量和生产签名/TLS 门禁，归 STORY-061。

## Implementation Progress (2026-07-11)

### 已实现并验证

- PostgreSQL `capture_batch` / `capture_file` / `capture_page` / `page_registration_run` / `capture_operation` 与 `answer_segment` 证据字段已迁移；`000001` 到 `000029` 已在空数据库完整回放。
- 独立 `page-processing-worker` 已部署，支持 PDF、PNG/JPEG 和多页 TIFF 安全拆页，使用 pypdfium2/PDFium、Pillow、OpenCV 与 ZXing-C++ 依赖路线。
- 批次创建、文件登记、事务入队、Worker Runtime claim/lease/retry/dead-letter、页面物化、刷新恢复和重复内容页资产复用已实现。
- 已实现真实 ORB/AKAZE + RANSAC Homography、像素一致快速路径、双向矩阵、feature/match/inlier、inlier ratio、重投影误差、覆盖率和 confidence 证据。
- 已实现锁定模板题区的真实 PNG crop，registered page 与 crop 均进入受保护 `file_asset`，并回写 `answer_segment` 的模板 hash、run、bbox、crop hash 与 confidence。
- 页面旋转使用 revision 乐观锁并使旧 registration/segment 失效；跨考试 decoded/registered/crop 资产校验已加入。
- 配准失败使用专用回调同步 runtime task、registration run、capture page 与批次状态；批次只允许 `ready -> completed`。
- Web 已提供批次工作区、文件与页面、页面处理、处理问题和移动端操作；桌面与 390px Playwright 截图已保存。

### 真实验收证据

- 验收批次：`25f1e735-8c7f-4d81-bf46-bc71088b94d8`。
- 两页 PDF 拆分结果：1 个源文件、2 个 capture page、1 个 submission；相同图像页安全复用同一不可变文件资产，但保留两个独立页面记录。
- 配准 run：`f20ca93f-20d8-403c-affb-03c3077a317c`，`pixel_identity`，confidence `1.0`，状态 `completed/matched`。
- registered PNG：`2550x3301`；Q1 crop：`1912x645`，crop SHA-256 `fe33aaa55016e50f71d67514386bfd0305c1577602bf3d78363161eb5a85c8cb`。
- 模板只有 1 页时，第 2 页明确进入 `needs_review`，批次进入 `needs_review`，无法完成，证明缺页/多余页门禁生效。
- 测试：Go `go test ./...` 与 `go vet ./...`；Python `7 passed`；Web typecheck/build；Compose API/Web/worker 健康；空库迁移回放通过。

### 实现审阅修复

1. 共享 runtime task 类型检查未包含 `capture_file_decode`，新增 `000029` 扩充页面处理任务类型。
2. Worker 租户与业务租户不一致时无法 claim，当前采用每租户最小权限 worker 身份；跨租户服务身份治理保留到 STORY-060。
3. 重复页面 PNG 上传 409 曾导致重试耗尽，现复用文件服务返回的授权 `existing_file`。
4. runtime 先成功、业务结果后失败会造成不一致，现先幂等物化业务结果，再完成 runtime；重试可收敛。
5. 成功任务残留历史 `error_code`，现成功完成同时清空 task 与 attempt 错误字段。
6. 旋转后旧 crop 仍有效，现同一事务将 registration run 与 segment 标记 `invalidated`。

### 尚未完成，禁止标记 Approved

- 低置信配准已能人工确认算法结果，但人工四角/锚点校正与重跑 Homography 的完整接口和 UI 尚未实现。
- 专用 segment image API 尚未补齐；当前 crop 资产已真实生成，但消费方仍需通过通用文件接口读取。
- TIFF、损坏文件、低纹理、强透视、缺页、重复页、断点恢复和跨租户场景仍需扩展为 Compose + Playwright 端到端矩阵；现有 Python 单元测试只覆盖其中的解码与合成算法路径。

下一实现顺序：人工四角校正 -> segment image API -> 异常样本 E2E 矩阵 -> 实现审阅复核 -> Approved。
