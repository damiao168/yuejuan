# 答题卡图像质量检测 V2

## 范围

本阶段只完成答题卡图像质量检测、质量结论、标准化证据与管理员处置闭环。OCR、模板配准、ROI 切题和自动评分仅保留门禁与接口边界，不在本阶段宣称完成。

管理员入口不新增侧栏或独立页面，继续位于现有“答卷导入”批次详情中，通过“图像质检”标签页查看。

## 处理顺序

```text
上传/拆页
  -> Fast Gate（解码、有效分辨率、严重模糊、空白风险）
  -> 页面边界与有限小角度校正
  -> Detailed Assessment（七类指标 + 局部 Focus Map）
  -> Hard Gate
  -> weighted-rules-v2（后续可替换为 XGBoost OCR 失败概率模型）
  -> PASS / PASS_WITH_ENHANCEMENT / LOW_QUALITY / REJECT
```

模板配准后的必需 ROI 可见率、定位点数量与 Homography RMSE 目前明确返回 `not_evaluated` / `awaiting_template_registration`。这些字段已经进入报告和 Hard Gate 结构，待页面配准阶段接入真实模板坐标后填充；未评估不等于通过。

## 七类质量维度

| 维度 | 当前实现 | 关键证据 |
| --- | --- | --- |
| 页面完整性 | 页面轮廓、四边支持、边界置信度 | `border_completeness_score`、`page_border_status/confidence` |
| 有效分辨率 | 优先使用检测到的页面四边形实际短边，缺少边界时使用图像短边 | `effective_short_edge_px`、`estimated_a4_dpi` |
| 清晰度 | 8×12 局部含墨迹 patch；Laplacian、Tenengrad、Gradient Energy 和方向性 | mean/median/P20/min、`bad_focus_patch_ratio`、`blur_pattern`、Focus Map |
| 几何质量 | Hough 残余倾斜、页面四边形透视风险、有限小角度自动校正 | skew angle/confidence、perspective status/score/confidence |
| 光照与对比度 | 分块背景估计、阴影、曝光、局部前景/背景对比度、反光风险 | `shadow_risk_score`、`low_contrast_area_ratio`、`glare_risk_score` |
| 遮挡与反光 | 大面积深色连通域启发式 + 置信度；模板关键 ROI 遮挡预留 | risk/confidence/candidate area |
| 噪声与压缩 | 平坦区残差噪声、JPEG 8px 边界块效应；摩尔纹预留 | noise sigma/risk、blockiness/compression risk、moire status |

所有阈值均属于 `opencv-default/v2` 启动阈值，必须用学校授权、脱敏的真实答题卡以及实际 OCR/公式识别失败率做 ROC/PR 校准。当前报告明确记录 `calibration_status=awaiting_real_answer_sheet_samples`，不得将启发式分数宣传为已校准概率。

## Hard Gate 与四档结论

完整性不参与平均分。以下已实现的致命项会直接阻断：

- 检测到的页面有效短边小于 1200 px；
- 局部清晰度、Laplacian 中位数和 Tenengrad 共同表明严重模糊；
- 在边界置信度足够时，页面主体完整度严重不足。

必需 ROI 完整性和关键 ROI 遮挡在模板配准前保持 `not_evaluated`，以后接入后仍作为 Hard Gate，不进入平均分。

| 报告结论 | 后端兼容状态 | 行为 |
| --- | --- | --- |
| `PASS` | `passed` | 可进入后续门禁 |
| `PASS_WITH_ENHANCEMENT` | `passed` | 已记录有限校正，再进入后续门禁 |
| `LOW_QUALITY` | `review` | 必须人工复核，不自动进入后续处理 |
| `REJECT` | `failed` | 重扫、重拍或人工处置 |

综合分沿用可解释权重：清晰度 25%，有效分辨率 15%，几何 15%，光照 15%，对比度 15%，遮挡 10%，噪声/压缩 5%。综合分永远不能覆盖失败的 Hard Gate。

## 后端与审计

- `submission_page_quality_run` 保存不可变历史；页面只保存最新 run、标准化资产和兼容质量状态。
- Worker 通过 claim + lease 获取任务，通过幂等 result 回写；旧租约不能覆盖新 attempt。
- 标准化输出为新的 RGB PNG file asset，原图不覆盖；报告保存输入/输出、profile、schema、transform、耗时与 worker 信息。
- `GET /api/v1/submission-pages/{id}/quality-runs` 返回当前租户内该页的倒序质检历史，不暴露 lease token 或 result payload hash。
- `POST /api/v1/submissions/{id}/run-quality-check` 创建重检任务；现有批次页使用它重检选中页面所属整份答卷。
- `POST /api/v1/submission-pages/{id}/quality-override` 保留原问题、操作人、理由与时间，仅恢复后续处理，不篡改原 run 结论。

## 管理员工作台

现有批次页内的“图像质检”标签提供：

- 全部、需处理、合格、待检测的页面级筛选与计数；
- 原图/标准化图切换，以及局部 Focus Map 覆盖层；
- 七项维度、综合分、A/B/C/D 结论、结构化问题和 Hard Gate 明细；
- 不可变历史 run 切换、整份答卷重检和已有的人工放行流程。

页面配准、ROI 完整性与深度模型 fallback 的状态会显示为等待后续阶段，不伪造成可操作的完成能力。
