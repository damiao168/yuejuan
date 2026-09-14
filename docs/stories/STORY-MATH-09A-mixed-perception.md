# STORY-MATH-09A：Mixed Math Perception

## 状态

Implemented / Real-model Benchmark Pending

编号使用 `09A`，以保留既有 `STORY-MATH-09` Runtime Spatial 历史。

## 目标

数学、物理、化学的结构化答题区不再被当作一张完整公式图片。一个不可变 `answer_segment` 裁切图同时经过全文 OCR 和公式区域检测，公式 ROI 批量进入 FormulaNet，再合成为带归一化坐标、置信度和可审计候选的 mixed evidence。

## 实现

- `math_layout.py` 成为 Paper Import 与学生答题区共用的公式布局边界，统一承载 Paddle LayoutDetection、公式框合并、ROI padding、边缘墨迹检查和 adaptive recrop。
- `RegionKind.MIXED` 同时执行答题区 OCR 与公式布局检测；公式 ROI 使用 FormulaNet 批量识别。
- OCR 行与公式 ROI 相交时，按几何范围移除/拆分公式对应的 OCR 文本，避免同一图像区域同时成为 text 和 formula 证据。
- Worker 将像素 bbox 依据原图尺寸归一化到 `[0,1]`，修复旧 text 路径可能提交非法像素坐标的问题。
- `FormulaArtifact` 增加 `candidates` 与 `selected_candidate`；当前生产路由保存所选 FormulaNet 候选，结构允许后续接入 plus-L、UniMERNet 或受治理的 VLM challenger。
- detector、crop 或 FormulaNet 不可用/不完整时保留可见证据、写入稳定 reason code，并设置 `requires_human_review`，不静默降级成可信文本。
- migration `000145` 将新任务写为 `math-understanding-task-v2` / `region_kind=mixed`，并只升级尚未开始的 queued v1 formula 任务；leased、running 和历史结果不被改写。

## 边界

- 公式模型仍只允许数学、物理、化学；其他学科不会调用公式模型。
- 此 Story 不实现 connector/diagram learned detector、SolutionGraph v2 分步器、跨步骤 SymPy 验证或自动评分。
- 当前 overlap 拆分受限于 Paddle OCR 的行级 bbox；没有字符级 bbox 时按水平方向比例拆分并降低置信度。
- `Implemented` 仅代表软件链路和合成回归通过，不代表真实手写答卷、FormulaNet 准确率或学校试点门禁已完成。

## 验收

- mixed 路由在同一图片上同时执行 OCR、layout detector 和公式 batch recognition。
- 示例 `解：由题意得x2=2` 与右侧公式 ROI 合成后只保留文本 `解：由题意得` 和公式 `x=2`。
- mixed artifact 包含 text/formula blocks、归一化 bbox、候选、selected candidate、syntax verification 和人工复核信号。
- detector 缺失、无 ROI、裁切不完整及公式引擎失败均 fail closed。
- Paper Formula 原有 detector/crop 回归保持通过。
