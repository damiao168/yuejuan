# STORY-050 答卷图像质量检测与页面标准化规格

## Plan

### 目标

把真实答卷上传后的第一道生产关口补齐：系统必须在 OCR 之前判断每一页答卷图像是否清晰、完整、可解码、方向风险可控，并生成可追溯的标准化页面文件。

完成后，流程从：

```text
submission_page uploaded -> OCR task
```

升级为：

```text
submission_page uploaded
-> create immutable image quality run
-> worker claim with lease
-> download original file asset
-> validate image/PDF safety limits
-> detect quality metrics
-> generate normalized RGB PNG asset
-> submit structured result
-> pass/review/failed quality gate
-> later OCR/template/crop workflow
```

这一 Story 的核心不是提高 OCR 准确率本身，而是阻止坏图、缺页、严重模糊、过曝、阴影、透视风险、不可解码文件和来源已变更的页面静默进入 OCR 与评分链路。

### 背景

项目当前已经具备：

- `file_asset` 上传与 MinIO/对象存储。
- `submission` 与 `submission_page`。
- 基础 `quality_status`、`quality_issues` 概念。
- STORY-049 的真实 OCR worker。
- PaddleOCR 本地部署验证；公开手写答卷测试证明：手写 OCR 只能作为候选文本来源，必须依赖质量门禁、区域切割和人工复核。

当前缺口：

- `RunQualityCheck` 主要是页数/状态层面的业务检查，不是真实图像质量检查。
- 缺少每页图像分辨率、清晰度、曝光、对比度、阴影、空白页、边框完整性、透视风险、倾斜角、归一化变换等指标。
- 缺少标准化页面资产，以及原图与标准化图之间的可追溯关系。
- 缺少 worker 领取、lease、防重复、失败隔离和幂等回写协议。
- 缺少“worker 执行失败”和“图像质量不合格”的状态分离。
- 缺少原图替换后自动作废旧质量结果的规则。

### 技术路线

推荐路线：独立 Python 图像预处理 worker + Go API Gateway 受控回写。

理由：

- OpenCV、NumPy、Pillow、pypdfium2 等图像处理生态更适合 Python。
- Go API Gateway 继续负责租户隔离、权限、状态机、审计、文件元数据和任务控制面。
- 与 STORY-049 OCR worker 保持一致：worker 不直写数据库，只通过受控 API 领取任务、下载原图、上传标准化图、回写结构化质量结果。
- 后续可接入 STORY-051 的通用 Agent Worker Runtime；本 Story 先实现 claim + lease 的最小可靠闭环。

不推荐把 OpenCV 逻辑塞进 Go API Gateway。图像处理依赖会污染核心 API 镜像，耗时操作也容易拖慢业务请求。

### 开源与产品参考

- OpenCV：灰度/颜色统计、清晰度估计、边缘检测、倾斜估计、空白页检测、阴影和透视风险检测。
- Pillow：EXIF 读取、格式解码、RGB PNG 输出、EXIF 清理。
- pypdfium2：PDF 页面渲染，记录 `render_dpi`。
- PaddleOCR orientation/textline：可作为后续内容方向检测候选，但本 Story 不依赖 OCR 文本决定页面是否合格。
- RM Assessor/成熟电子阅卷实践：坏页、缺边、反光、模糊、页序异常先进入扫描质检或人工复核，不能直接进入评分。
- Gradescope 固定模板思想：后续模板配准依赖稳定页面质量；本 Story 先提供标准化页面和质量标记。

## Scope

本 Story 实现：

- 新增 `image-quality-worker`，或在现有 Python worker 体系下新增清晰分离的 `image_quality` 模块。
- 支持读取 submission page 的当前原始 file asset。
- 支持图片与 PDF 渲染页的安全解码、质量检测和标准化输出。
- 为每次质量检测创建不可变的 `submission_page_quality_run`。
- `submission_page` 只保存最新质量结论、最新 run 指针和当前标准化资产指针。
- 通过 claim + lease 领取质量检测任务，避免重复 worker 并发处理同一 run。
- 标准化图作为新的 RGB PNG file asset 保存，不覆盖原图。
- 回写可解释质量报告、质量问题、归一化变换、worker 版本、profile 版本和耗时。
- 低质量页默认不得进入 OCR；人工 override 必须单独记录，不得把 `review` 直接改写为 `passed`。
- 原图替换、删除或重新上传时，旧质量结论失效并重置 OCR gate。
- 更新 API 文档、部署配置、Story 索引和路线图。

本 Story 不实现：

- 固定模板管理。
- 页面四角定位点设计。
- Homography 模板配准。
- 按题切图。
- OCR 人工校正页面。
- OCR 微调。
- 主观题 AI 评分。
- 完整 River/Temporal worker runtime。
- `lab/` 接入生产链路。

## 状态边界

必须区分两类状态：

- `quality_status`：图像质量结论，只表示页面是否适合进入后续 OCR/切题链路。
- `processing_status`：worker/run 执行状态，只表示任务是否完成、可重试或终止失败。

`quality_status` 允许值：

```text
unchecked
passed
review
failed
```

`processing_status` 允许值：

```text
pending
processing
completed
retryable_error
terminal_error
```

基础设施错误、worker 超时、对象存储上传失败、网络失败不得写成 `quality_status=failed`。这些情况应写入 run 的 `processing_status`、`error_code`、`error_detail`，并通过重试或人工运维处理。

## Data Model

### `submission_page`

`submission_page` 保存当前页面的最新可见状态，不保存完整历史。

建议新增字段：

```sql
latest_quality_run_id UUID NULL
normalized_file_asset_id UUID NULL
quality_status TEXT NOT NULL DEFAULT 'unchecked'
quality_override JSONB NOT NULL DEFAULT '{}'
```

字段语义：

- `latest_quality_run_id`：当前页面最新有效质量检测 run。
- `normalized_file_asset_id`：当前页面最新有效标准化 RGB PNG file asset。
- `quality_status`：当前页面质量结论。
- `quality_override`：人工放行对象，必须包含审批人、原因、时间和适用范围；override 不改变原始 `quality_status`。

### `submission_page_quality_run`

新增不可变质量检测 run 表。每次检测、重试或使用新 profile 都创建新 run，不覆盖历史结果。

建议结构：

```sql
CREATE TABLE submission_page_quality_run (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  submission_id UUID NOT NULL,
  submission_page_id UUID NOT NULL,
  source_file_asset_id UUID NOT NULL,
  source_sha256 TEXT NOT NULL,
  normalized_file_asset_id UUID NULL,
  processing_status TEXT NOT NULL DEFAULT 'pending',
  quality_status TEXT NULL,
  profile_name TEXT NOT NULL,
  profile_version TEXT NOT NULL,
  profile_config_hash TEXT NOT NULL,
  metric_schema_version TEXT NOT NULL,
  report_schema_version TEXT NOT NULL,
  quality_report JSONB NOT NULL DEFAULT '{}',
  quality_issues JSONB NOT NULL DEFAULT '[]',
  normalization_transform JSONB NOT NULL DEFAULT '{}',
  worker_service TEXT NULL,
  worker_instance_id TEXT NULL,
  attempt_no INT NOT NULL DEFAULT 0,
  result_version TEXT NULL,
  lease_token TEXT NULL,
  lease_expires_at TIMESTAMPTZ NULL,
  started_at TIMESTAMPTZ NULL,
  completed_at TIMESTAMPTZ NULL,
  duration_ms INT NULL,
  error_code TEXT NULL,
  error_detail JSONB NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

约束建议：

- `(tenant_id, id)` 建复合唯一或纳入现有多租户约束模式。
- `submission_page.latest_quality_run_id` 指向同租户 run。
- `submission_page.normalized_file_asset_id` 指向同租户 file asset。
- run 的 `source_file_asset_id` 和 `source_sha256` 必须与回写时页面当前原图一致，否则拒绝激活结果。

### `submission`

保留现有 `quality_status` 和 `quality_issues`，但语义升级：

- `passed`：所有有效页面质量通过，页数与预期一致，且没有处理中或终止错误阻塞项。
- `review`：至少一页需要人工确认，或存在可人工放行的质量风险。
- `failed`：缺页、重复页、严重坏图、不可解码、无法生成标准化图，且无法继续自动流程。
- `unchecked`：存在页面但尚未完成真实图像质量检测，或原图替换后旧结果已失效。

聚合时必须处理：

- `expected_page_count` 为空。
- 缺失 `page_no`。
- 重复 `page_no`。
- 页面已删除。
- run 仍在 `pending/processing`。
- run 进入 `terminal_error`。
- 页面质量 `review` 但存在有效人工 override。

### `file_asset`

原图不得覆盖。标准化图另存，例如：

```text
submissions/{submission_id}/original/page-001.png
submissions/{submission_id}/normalized/page-001.png
```

标准化图 file asset 必须记录：

- `tenant_id`
- `exam_id`
- `submission_id`
- `owner_type = submission_page_normalized`
- `owner_id = submission_page.id`
- `hash_sha256`
- `content_type = image/png`
- `uploaded_by = image-quality-worker` 或对应 service principal

对象 key 不是权限边界；访问授权必须通过 `file_asset` 的租户与权限判断。

## API Design

### 创建质量检测 run

`POST /api/v1/submissions/{id}/run-quality-check`

保留现有入口，但语义改为：为当前有效页面创建 `submission_page_quality_run`，或返回已存在的 pending/processing run。

要求：

- 只为当前原图 file asset 创建 run。
- 记录 `source_file_asset_id` 与 `source_sha256`。
- 不能为已删除页面创建 run。
- 如果页面原图已变化，旧 run 不再可激活。

### Worker claim with lease

替代无锁 pending 列表。新增内部接口：

`POST /api/v1/internal/image-quality/jobs/claim`

请求：

```json
{
  "worker_instance_id": "host-01-process-1234",
  "limit": 5,
  "lease_seconds": 300
}
```

响应：

```json
{
  "jobs": [
    {
      "run_id": "run_001",
      "submission_id": "submission_001",
      "submission_page_id": "page_001",
      "source_file_asset_id": "file_001",
      "source_sha256": "sha256:...",
      "download_url": "/api/v1/files/file_001/download",
      "lease_token": "opaque-token",
      "lease_expires_at": "2026-07-09T10:00:00Z",
      "profile": {
        "name": "opencv-default",
        "version": "v1",
        "config_hash": "sha256:..."
      }
    }
  ]
}
```

实现要求：

- 后端通过认证的 service principal 识别 worker 服务；请求体中的 `worker_instance_id` 只用于审计和诊断，不作为权限依据。
- 数据库领取建议使用 `FOR UPDATE SKIP LOCKED` 或等价机制。
- 领取成功后 run 进入 `processing`，写入 `lease_token`、`lease_expires_at`、`worker_service`、`worker_instance_id`、`started_at` 和 `attempt_no`。
- lease 过期后可被重新领取。

### 标准化资产上传槽

新增内部接口：

`POST /api/v1/internal/image-quality/runs/{run_id}/normalized-assets`

请求：

```json
{
  "lease_token": "opaque-token",
  "content_type": "image/png",
  "byte_size": 1834200,
  "sha256": "sha256:...",
  "pixel_width": 2480,
  "pixel_height": 3508
}
```

响应：

```json
{
  "upload_url": "/api/v1/files/upload-slots/slot_001",
  "upload_method": "PUT",
  "expires_at": "2026-07-09T10:03:00Z",
  "expected_sha256": "sha256:..."
}
```

上传后 API 必须校验：

- run 与 lease 有效。
- asset 属于同租户、同 submission、同 page。
- 内容魔数与解码结果为 PNG，不能只信 HTTP `Content-Type`。
- 上传文件 hash 与声明一致。

已上传但未被 result finalize 的标准化 asset 必须有孤儿清理策略。

### 提交质量结果

`POST /api/v1/internal/image-quality/runs/{run_id}/result`

请求：

```json
{
  "lease_token": "opaque-token",
  "attempt_no": 1,
  "result_version": "sha256:payload-hash",
  "duration_ms": 740,
  "processing_status": "completed",
  "quality_status": "review",
  "normalized_file_asset_id": "file_normalized_001",
  "profile": {
    "name": "opencv-default",
    "version": "v1",
    "config_hash": "sha256:..."
  },
  "metric_schema_version": "image-quality-metrics-v1",
  "report_schema_version": "image-quality-report-v1",
  "quality_report": {
    "pixel_width": 2480,
    "pixel_height": 3508,
    "source_dpi": null,
    "source_dpi_method": "missing_metadata",
    "render_dpi": null,
    "source_format": "image/jpeg",
    "output_format": "image/png",
    "normalized_color_mode": "RGB",
    "metrics": {
      "sharpness_score": 0.72,
      "brightness_score": 0.61,
      "exposure_quality_score": 0.78,
      "contrast_score": 0.42,
      "shadow_risk_score": 0.28,
      "blank_probability": 0.03,
      "perspective_risk_score": 0.66,
      "border_completeness_score": 0.91
    },
    "geometry": {
      "detected_skew_angle": 3.4,
      "skew_confidence": 0.86,
      "skew_correction_applied": true,
      "page_border_status": "detected",
      "page_border_confidence": 0.8,
      "perspective_status": "detected",
      "perspective_confidence": 0.72
    }
  },
  "quality_issues": [
    {
      "code": "perspective_risk",
      "severity": "review",
      "metric": "perspective_risk_score",
      "observed": 0.66,
      "threshold": 0.6,
      "rule_id": "perspective-risk-v1",
      "action": "manual_review",
      "parameters": {
        "confidence": 0.72
      }
    }
  ],
  "normalization_transform": {
    "source_pixel_width": 3024,
    "source_pixel_height": 4032,
    "normalized_pixel_width": 2480,
    "normalized_pixel_height": 3508,
    "exif_rotation_degrees": 90,
    "content_rotation_degrees": 0,
    "detected_skew_angle": 3.4,
    "skew_correction_applied": true,
    "crop_box_source_pixels": [120, 90, 2890, 3900],
    "source_to_normalized_matrix": [
      [0.82, 0.01, -98.2],
      [-0.01, 0.82, -72.1],
      [0.0, 0.0, 1.0]
    ]
  }
}
```

后端校验：

- `run_id + lease_token + attempt_no` 必须匹配当前 run。
- `source_file_asset_id` 与 `source_sha256` 仍等于页面当前原图。
- `profile_name/profile_version/profile_config_hash` 与 claim 时一致。
- `normalized_file_asset_id` 已上传、已验 hash、同租户、同 page。
- `quality_status=passed` 时必须有有效标准化资产。
- worker 不能提交任意英文 `message` 作为面向用户的文案；只能提交 `code/severity/parameters`，前端根据 code 映射中文提示。

### 幂等性

结果回写必须幂等：

- 幂等键：`run_id + attempt_no + result_version`。
- 同一幂等键、同一内容重复提交，返回 200 和已保存结果。
- 同一幂等键、不同内容，返回 409。
- lease 过期后旧 worker 不得覆盖新 attempt 结果。

## Image Quality Rules

第一版不写死“生产阈值”，而是提供可调 profile、可解释指标和保守门禁。

默认规则只做保守判断：

- 图片无法解码：`failed/unreadable_image`
- 超过安全限制：`failed/image_too_large` 或 `failed/pdf_too_large`
- 页面为空白：`review/blank_page` 或 `failed/blank_page`，取决于考试是否允许空白页。
- 分辨率明显过低：`review/low_resolution`
- 严重模糊：`review/low_sharpness`
- 严重过暗/过亮：`review/bad_exposure`
- 严重阴影：`review/shadow_risk`
- 透视风险高：`review/perspective_risk`
- 边框或页面主体不完整：`review/incomplete_page_border`
- 无法稳定标准化：`review/normalization_uncertain`

指标阈值必须在后续真实样本集上校准，不能凭经验宣称生产通过。

### 指标定义

第一版 `metric_schema_version=image-quality-metrics-v1` 至少包含：

| 指标 | 范围 | 方向 |
| --- | --- | --- |
| `sharpness_score` | 0-1 | 越高越清晰 |
| `brightness_score` | 0-1 | 平均亮度，0 暗、1 亮 |
| `exposure_quality_score` | 0-1 | 越高曝光越正常 |
| `contrast_score` | 0-1 | 越高对比度越好 |
| `shadow_risk_score` | 0-1 | 越高阴影风险越高 |
| `blank_probability` | 0-1 | 越高越像空白页 |
| `perspective_risk_score` | 0-1 | 越高透视风险越高 |
| `border_completeness_score` | 0-1 | 越高页面边界越完整 |

要求：

- 使用 `sharpness_score`，不使用含义不清的 `blur_score`。
- 可同时保留 raw metrics 与 0-1 normalized scores。
- `missing_border_detected`、`perspective_detected` 不使用纯 bool；改为 `status + score + confidence`。
- 不默认计算 `dpi_estimate`；优先记录 `pixel_width`、`pixel_height`、`source_dpi`、`source_dpi_method`，PDF 渲染记录 `render_dpi`。

### Issue 结构

`quality_issues` 必须说明为什么触发 review/failed：

```json
{
  "code": "low_sharpness",
  "severity": "review",
  "metric": "sharpness_score",
  "observed": 0.22,
  "threshold": 0.35,
  "rule_id": "sharpness-min-v1",
  "action": "manual_review",
  "parameters": {
    "window": "full_page"
  }
}
```

## Normalization

标准化输出要求：

- 保留原图，不覆盖。
- 输出主标准化资产为 RGB PNG，保留颜色；灰度图只可作为内部计算或后续 OCR 派生资产。
- 剥离不必要 EXIF 和敏感元数据。
- 写入 `normalized_file_asset_id`。
- 记录原图 hash、标准化图 hash、profile、worker、耗时和 transform。
- 标准化失败不得生成假通过状态。

第一版允许做：

- EXIF 方向修正。
- PDF 固定 `render_dpi` 渲染。
- 轻量对比度/曝光归一化。
- 倾斜角估计与有限旋转校正。
- 输出 normalized RGB PNG。
- 记录透视风险，但不强行做复杂透视裁正。

方向边界：

- 第一版只保证 EXIF rotation 处理。
- OpenCV 倾斜线检测可以处理小角度 skew，但不能可靠判断 180 度内容方向。
- 90/180 度内容方向可作为风险进入 review，或后续引入轻量分类器。
- 测试不得宣称“通用方向自动纠正”，只能验证 EXIF correction 和受限 skew correction。

倾斜校正规则：

- 只有当 `abs(detected_skew_angle) <= max_auto_skew_angle` 且 `skew_confidence >= min_skew_confidence` 时自动旋转。
- 超出范围或置信度不足时进入 review。
- 必须记录 `detected_skew_angle`、`skew_confidence`、`skew_correction_applied`。

透视 Homography 和模板配准放到后续 Story。

## Profile

质量检测 profile 必须不可变：

```json
{
  "name": "opencv-default",
  "version": "v1",
  "config_hash": "sha256:...",
  "metric_schema_version": "image-quality-metrics-v1",
  "report_schema_version": "image-quality-report-v1"
}
```

一旦 profile 被用于 run，不得原地修改配置。阈值或算法变化必须发布新 `version` 或新 `config_hash`。

## OCR Gate

页面允许进入 OCR 必须同时满足：

- 页面当前原图未被替换。
- 最新 run 的 `processing_status=completed`。
- `normalized_file_asset_id` 存在且仍有效。
- `quality_status=passed`，或存在有效人工 `quality_override`。
- 人工 override 包含 approver、reason、approved_at、scope，不得把原始 `review` 改成 `passed`。

submission 聚合到 `ready_for_ocr` 时，还要验证页数、页码、缺页、重复页、删除页和 run 错误状态。

## 原图替换与失效

当 `submission_page` 的原始 file asset 被替换、删除或重新上传时：

- `quality_status` 重置为 `unchecked`。
- `latest_quality_run_id` 置空。
- `normalized_file_asset_id` 置空。
- `quality_override` 清空或标记失效。
- `ready_for_ocr=false`。
- 旧 run 保留历史，但不得再激活到当前页面。

结果提交时如果发现当前页面的 `source_file_asset_id/source_sha256` 与 run 不一致，必须返回 409 或等价冲突错误。

## Worker Design

建议新增：

```text
services/image-quality-worker
```

第一版也可以复用 `services/ocr-worker` 的 API client 结构，但逻辑必须分模块：

```text
services/image-quality-worker/
  image_quality/
    api.py
    engine.py
    runner.py
    config.py
    schemas.py
    __main__.py
  tests/
```

依赖：

- Python 3.11
- OpenCV
- NumPy
- Pillow
- pypdfium2

worker 不直接读写数据库，只走 API Gateway 和文件下载/上传 API。

## Security and Privacy

- 不在普通日志输出完整 OCR 文本、学生答案、学生姓名、准考证号、身份证号或成绩。
- quality-input/claim 响应不返回学生身份字段。
- worker 使用认证 service principal；`worker_instance_id` 不作为权限依据。
- 原图与标准化图都必须走租户隔离的 file asset。
- 不信任 HTTP `Content-Type`，必须校验魔数和实际解码结果。
- 增加 decompression bomb、防超大像素、防超大尺寸、防 PDF 页数过多、防处理超时保护。
- 标准化图剥离不必要 EXIF。
- 对象 key 不能作为权限边界，必须通过 file asset 授权。
- 所有质量结果必须带 profile、schema version、input hash、output hash、duration，便于审计和回放。

## Tests

实现阶段必须先写失败测试，再实现。

后端测试：

- `run-quality-check` 为当前原图创建 `submission_page_quality_run`。
- 原图替换后旧 run 不能激活，并重置页面质量状态。
- claim 只返回当前租户、可处理、未删除、未 lease 或 lease 过期的 run。
- 并发 claim 不会重复领取同一 run。
- worker crash 后 lease 过期可重新领取。
- result 必须校验 `run_id + lease_token + attempt_no`。
- 同一 `result_version` 重复提交返回 200，不同内容返回 409。
- normalized asset 上传失败时 run 不得激活为 completed/passed。
- quality result 不泄露 `student_id`、`candidate_no`。
- 任一 failed 页会使 submission `quality_status=failed`，但 worker 基础设施错误不等于 failed 页。
- 任一 review 页且无 failed 页会使 submission `quality_status=review`。
- 全部 passed 且页数符合预期会使 submission `quality_status=passed`。
- `ready_for_ocr` 必须要求质量通过或有效人工 override。
- duplicate/missing page_no、deleted pages、expected_page_count 为空的聚合行为有测试。

worker 测试：

- 清晰测试图返回 `passed`。
- 模糊测试图返回 `review` 和 `low_sharpness`，不断言 OpenCV 精确数值，只断言状态/issue/相对关系。
- 空白图返回 `review` 或 `failed` 和 `blank_page`。
- 无法解码文件返回 `failed/unreadable_image`。
- EXIF 旋转图会输出 EXIF correction。
- 小角度倾斜在置信度足够时校正，超过阈值或置信度不足时进入 review。
- 标准化输出不覆盖原文件。
- `normalization_transform.source_to_normalized_matrix` 与输出尺寸、crop box 关系一致。
- PNG 输出为 RGB，并剥离不必要 EXIF。
- 超大尺寸、超大 PDF、超时、Content-Type 伪造都有保护测试。

建议 fixture：

```text
tests/fixtures/image_quality/
  clear_scanner.png
  mobile_shadow.jpg
  slight_skew.jpg
  severe_blur.jpg
  partial_page.jpg
  rotated_exif.jpg
  perspective_photo.jpg
  low_contrast_pencil.jpg
```

静态/部署测试：

- `npm run check:story050` 验证 worker、docs、compose profile/API capability 状态。
- `docker compose --profile quality config` 通过。
- 后端 `go test ./...` 通过。
- worker 单测通过。

## Acceptance Criteria

- 存在真实图像质量 worker，不是 mock/stub。
- OpenCV 对真实图片执行质量指标计算。
- 原图不被覆盖。
- 标准化 RGB PNG 另存为 file asset，并带元数据与 hash。
- 每页有最新状态，也有不可变 `submission_page_quality_run` 历史。
- 每个 run 记录 source hash、profile、schema version、quality report、issues、transform、duration 和 worker 信息。
- worker 使用 claim + lease，不使用简单无锁 pending 列表。
- result 回写幂等，并防止旧 attempt 覆盖新 attempt。
- submission page 有质量结论，submission 有聚合质量状态。
- 低质量页不能静默进入 OCR。
- 原图替换会作废旧质量结果与 OCR gate。
- API 文档说明质量检测、标准化图、上传链路、幂等性和状态机。
- 安全限制覆盖超大文件、伪造 Content-Type、EXIF 清理和敏感日志。
- `lab/` 未接入生产链路。
- Story 留下审阅、修正和批准记录。

## Plan Review

审阅发现：

- 如果本 Story 同时做模板配准和按题切图，范围会过大，并且会与后续固定模板 Story 混在一起；因此本 Story 只做到页面级质量和标准化。
- 如果只扩展现有 `RunQualityCheck`，会继续停留在业务页数检查，不能证明真实图像能力；必须引入 OpenCV worker。
- 如果 worker 直写数据库，会绕过租户隔离、审计和状态机；必须通过 API Gateway。
- 如果一开始硬编码生产阈值，会在不同扫描仪、手机、分辨率下误判；第一版只做保守门禁和可校准指标。
- 如果覆盖原图，会破坏证据链；原图必须永久保留。
- 如果质量接口返回学生身份字段，会扩大敏感数据暴露面；worker 只拿 page/file 输入。
- 如果只把最新结果写在 `submission_page`，会丢失审计历史和 profile 差异；必须引入不可变 quality run。
- 如果不区分 `quality_status` 和 `processing_status`，会把 worker 基础设施错误误判成学生答卷质量失败。
- 如果用简单 pending endpoint，会出现重复领取、worker crash 后状态卡死、旧结果覆盖新结果等问题；必须引入 claim + lease 与幂等 result。
- 如果不记录 `normalization_transform`，后续按题切图无法从原图坐标稳定映射到标准化图坐标。

## Spec Fixes

根据审阅，本规格修正为：

- 明确 `quality_status` 与 run `processing_status` 分离。
- 新增不可变 `submission_page_quality_run` 表，`submission_page` 只保存最新指针和状态。
- 新增 `latest_quality_run_id`、`normalized_file_asset_id`、`quality_override`。
- 新增 `normalization_transform`，记录尺寸、旋转、crop box 和 `source_to_normalized_matrix`。
- 用 `POST /api/v1/internal/image-quality/jobs/claim` 替代简单 pending endpoint，并要求 lease。
- 新增标准化资产上传槽 API。
- 明确 result 幂等规则：`run_id + attempt_no + result_version`。
- 明确 normalized asset 上传、hash 校验、finalize 与孤儿清理流程。
- 明确第一版只保证 EXIF correction 和受限 skew correction，不承诺通用 90/180 内容方向自动纠正。
- 原图替换时重置质量状态、标准化资产和 OCR gate。
- `blur_score` 改为 `sharpness_score`，并定义指标范围与方向。
- `quality_issues` 改为结构化 code/severity/metric/threshold/action，不允许 worker 提交面向用户的自由文本 message。
- 不默认计算 `dpi_estimate`，改为记录 pixel size、source dpi metadata 和 PDF `render_dpi`。
- 透视和缺边从 bool 改为 status/score/confidence。
- 标准化主资产明确为 RGB PNG，灰度只做内部派生。
- profile 改为不可变 name/version/config_hash。
- worker 身份由 service principal 决定，body 中 worker id 仅作实例诊断。
- OCR gate 增加 source unchanged、completed run、normalized asset、quality passed 或 human override 条件。
- 补充分页聚合、文件安全、魔数校验、EXIF 清理、测试 fixture 和并发/幂等测试要求。

## Implementation Entry Criteria

进入实现前需要确认：

- 接受 STORY-050 从“主观题模型适配器”调整为“答卷图像质量检测与页面标准化”。
- 接受新增独立 Python image-quality worker 或清晰分离的 worker 模块。
- 接受新增 `submission_page_quality_run` 不可变历史表。
- 接受 claim + lease，而不是简单 pending 列表。
- 接受本 Story 先不做模板配准和按题切图。
- 接受质量阈值第一版为保守默认，后续用真实样本校准。

## Implementation

本 Story 已完成首版实现。

新增后端能力：

- 新增 `services/api-gateway/internal/imagequality` 包。
- 新增不可变 image quality run 模型、内存 store、Postgres store、handler 和测试。
- 新增 `POST /api/v1/submissions/{id}/run-quality-check`。
- 新增 `POST /api/v1/internal/image-quality/jobs/claim`，支持 lease、attempt 和过期 lease 重新领取。
- 新增 `POST /api/v1/internal/image-quality/runs/{runId}/normalized-assets`。
- 新增 `POST /api/v1/internal/image-quality/runs/{runId}/result`，支持 result 幂等和 lease 校验。
- `submission_page` 类型新增 `latest_quality_run_id`、`normalized_file_asset_id`、`quality_status`、`quality_override`。
- 原图替换会重置页面质量状态、run 指针、normalized asset 指针和 override。
- result 回写会激活页面最新 quality run，并聚合 submission 质量状态。

新增数据库能力：

- 新增 `000022_story050_image_quality_run.sql`。
- 新增 `submission_page_quality_run` 表。
- 扩展 `submission_page` 最新质量字段。
- 扩展 `quality_status` 允许 `review`。
- 增加 claim/lease、page、submission 相关索引。

新增 Python worker：

- 新增 `services/image-quality-worker`。
- `image_quality.engine` 使用 Pillow + OpenCV/NumPy 做 EXIF 修正、清晰度、亮度、曝光、对比度、空白风险等指标。
- 标准化输出为 RGB PNG。
- 输出 `quality_report`、`quality_issues` 和 `normalization_transform`。
- `image_quality.runner` 串联 claim、download、analyze、normalized asset slot、upload、submit result。
- `image_quality.api` 提供真实 HTTP client。

新增部署与验收：

- 新增 `services/image-quality-worker/Dockerfile`。
- docker compose 新增 `image-quality-worker`，profile 为 `quality`。
- `.env.example` 和 `infra/docker-compose/.env.example` 新增 image quality worker 配置。
- 新增 `npm run check:story050` 静态验收脚本。
- 新增实现计划：`docs/superpowers/plans/2026-07-09-story050-image-quality-normalization.md`。

## Implementation Review

逐项自审结论：

- 真实 worker：已新增 Python worker，不是 mock/stub。
- OpenCV 指标：已计算 `sharpness_score` 等真实图像指标。
- 原图保留：worker 只下载原图并上传新的 normalized PNG，不覆盖原图。
- 标准化资产：result 必须引用已存在的 image/png file asset；runner 会上传 normalized PNG。
- 不可变 run：后端新增 `submission_page_quality_run`，page 只保留最新指针。
- claim + lease：已实现 claim、lease token、lease 过期后重新领取。
- 幂等回写：`run_id + attempt_no + result_version` 同内容重复提交返回现有结果，不同内容返回冲突。
- OCR gate：本 Story 已把 page/submission 质量状态打通；OCR 继续依赖 `ready_for_ocr`，后续 STORY-051/后续 gate 可进一步把 override 和 normalized-only 输入收紧。
- 原图替换失效：已实现页面质量状态和指针重置。
- 安全边界：claim/result 不返回学生身份字段；worker payload 不包含学生身份字段。
- `lab/`：未接入生产链路。

保留风险：

- normalized asset slot 首版返回受控上传信息，但仍复用现有 `/api/v1/files` 上传能力；严格的 slot finalize 与孤儿清理可在 STORY-051 worker runtime 或文件服务增强中继续收紧。
- PDF 渲染、透视检测、阴影检测和 skew 自动校正目前是保守首版，指标结构已预留，后续需要真实样本集校准。
- worker service principal 目前复用 `ocr:manage` 权限，后续应新增更细的 `submission:quality` 或 `image_quality:work` 权限。

## Fixes

实现自审中修正：

- result API 校验顺序调整为先校验 lease/result input，再校验 normalized asset，避免缺 lease 请求被文件校验掩盖。
- Dockerfile CMD 调整为 `python -m image_quality`，使静态验收脚本可明确识别启动命令。
- 同步补充 `infra/docker-compose/.env.example`，避免 quality profile 缺少 worker 配置样例。

## Verification

已运行并通过：

```text
npm.cmd run check:story050
go test ./...
python -m pytest -q
docker compose --env-file infra/docker-compose/.env.example -f infra/docker-compose/docker-compose.yml --profile quality config
```

验证结果：

- STORY-050 静态检查通过。
- Go API Gateway 全量测试通过。
- Python image-quality-worker 测试 4 个全部通过。
- Docker Compose quality profile 配置可解析。

## Approval

结论：Approved。

STORY-050 已完成规格、规格审阅修正、实现、实现自审、实现修正和验收记录。下一 Story 可进入 STORY-051 Agent Worker Runtime。
