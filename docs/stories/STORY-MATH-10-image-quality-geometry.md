# STORY-MATH-10：Image Quality Geometry / Register Foundation

## Problem

`image-quality-worker` 已能输出 skew、page border、perspective 和 shadow 测量，但 normalized PNG 未执行安全 deskew；发生 EXIF transpose 或旋转时，`source_to_normalized_matrix` 仍是恒等矩阵，无法可靠追溯源图坐标。

## Scope

- 复用现有 deterministic geometry detectors，并限制几何分析图的最大尺寸。
- 仅对高置信、0.5°～7° 的小角度倾斜执行保守 deskew。
- 扩大白色画布避免旋转裁边，并输出真实的 3×3 source-to-normalized transform。
- 组合 EXIF orientation 与 deskew transform，保持源图完整 crop box。
- page border、perspective、shadow 只测量和报告，不据此新增质量门禁。

## Non-Scope

不自动裁页、不做 perspective rectify、shadow removal、二值化、锐化或 Answer Perception；不新增模型、依赖、服务、API、数据库、前端或 Docker 改动。

## Implementation

在现有 `analyze_and_normalize` 调用链中加入安全 deskew 决策和扩大画布的 OpenCV affine rotation。矩阵按列向量语义组合 EXIF 与实际 deskew；`content_rotation_degrees`、输出尺寸和 `skew_correction_applied` 均来自真实执行结果。无法可靠检测的 border/perspective 返回 `unknown` 并继续处理。

## Tests

```text
cd services/image-quality-worker
python -m pytest tests/test_engine.py -q
python -m pytest tests/test_runner.py -q
python -m pytest -q

cd ../..
python -m ruff check services/image-quality-worker

git diff --check
```

## Evidence

Synthetic fixtures 覆盖正常页、4° 小角度倾斜、低置信页面、22° 异常旋转、完整/缺失边框、矩形/梯形页面、均匀/渐变照明以及 EXIF 1/3/6/8。测试验证 deskew 后残余角度降低、源图四角仍在 normalized canvas 内、Runner 上报尺寸与实际 PNG 一致。

这些 synthetic 证据仅证明确定性行为和坐标变换一致，不代表真实学校图像准确率或 OCR 效果提升。

## Remaining Risks

复杂背景、弯曲纸张、严重透视、弱边框、非文本主导页面和真实扫描设备分布尚未验证。Heuristic confidence 不是校准概率；不安全或不确定场景保持原图方向。

## Approval

Implemented：deterministic geometry measurement 与可追踪的安全小角度 deskew 已接入现有 Worker；教师最终控制和原图审计证据边界不变。
