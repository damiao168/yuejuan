# STORY-068：AI 阅卷准确率基准与错误归因

状态：Implemented（第一阶段）

## 产品结论

本故事不把“AI 准确率”压缩成一个总百分比。每条脱敏对齐观察同时记录最终评分差异和上游链路证据，从而回答错误发生在图像、页面匹配、答题区域裁切、文字/公式转录、答案结构化、Rubric、模型评分、分值计算还是系统层。

评测组件仍然只保存脱敏键、指纹、指标和人工裁决元数据，不保存答题原图、学生身份、答案文本或 Gold 内容。

## 论文到实现的映射

- 手写数学评分研究发现最佳模型的大多数错误来自转录而非 Rubric 应用，因此观察新增 `transcription_cer`、`formula_exact` 与分层 `error_source`，避免把视觉错误归咎于评分模型：[Automated Grading of Handwritten Mathematics Using Vision-Capable LLMs](https://arxiv.org/abs/2605.19043)。
- HITL 数学评分采用标准化扫描、细粒度评分标准、多次评分、一致性检查和人工验证。本阶段先固化页面匹配、Crop IoU、Rubric 评分点一致率和人工路由证据；多次评分将在独立故事中接入：[Human-in-the-Loop LLM Grading for Handwritten Mathematics Assessments](https://arxiv.org/abs/2603.13083)。
- LLM 评委存在重复运行自不一致，因此单次模型分不能作为稳定性证明，后续评测必须记录重复评分分布：[Rating Roulette](https://aclanthology.org/2025.findings-emnlp.1361/)。
- 校准应基于冻结验证集，而不是模型自报置信度；现有 calibration 子系统继续承担路由阈值标定：[Beyond the Score](https://aclanthology.org/2025.emnlp-main.992/)。
- 教师已评分案例和 RAG 可以提高短答案评分效果，但案例只能来自人工确认的 Gold 数据；案例检索不在本阶段伪造实现：[Language Models are Few-Shot Graders](https://arxiv.org/abs/2502.13337)。
- 中文手写与公式分别评测，不把公式当普通文字 OCR：[PP-OCRv5](https://github.com/PaddlePaddle/PaddleOCR/blob/main/docs/version3.x/algorithm/PP-OCRv5/PP-OCRv5.en.md)、[PP-FormulaNet](https://github.com/PaddlePaddle/PaddleOCR/blob/main/docs/version3.x/pipeline_usage/formula_recognition.en.md)。

## 新增观察证据

每条 `grading_evaluation_observation` 可记录：

- `page_match_correct`
- `crop_iou`
- `transcription_cer`
- `formula_exact`
- `rubric_criterion_agreement`
- `error_source`
- `needs_human_review`
- `reference_reviewer_count`
- `reference_adjudicated`

人工裁决真值必须至少两名复核者并完成裁决。旧调用未提供错误来源时，分数一致自动归为 `none`，分数不一致自动归为 `unattributed`，不允许错误静默消失。

## 质量摘要

`GET /api/v1/grading-evaluations/{runId}/quality-summary` 返回：

- 页面匹配准确率及覆盖样本数
- 平均 Crop IoU 及覆盖样本数
- 平均转录 CER 及覆盖样本数
- 公式精确率及覆盖样本数
- Rubric 评分点平均一致率及覆盖样本数
- 人工复核率
- 已知风险错误的人工路由召回率
- 各错误来源数量、占比、严重错误数和人工路由召回率

指标带覆盖样本数，未采集的层级显示“未采集”，不会用零伪装成真实结果。

## 冻结集抽样方案

第一阶段 1,000 份真实授权答题样本按学科分层：数学 250、语文 160、英语 160、物理 120、化学 100、生物 70、历史 50、地理 45、道德与法治/思想政治 45。5,000 份扩展集保持同一比例。

每个学科内部继续按学段、题型、零分/部分分/满分、OCR 质量、答案长度和 Rubric 复杂度分层；低质量图像、复杂公式、非常规正确解法和历史人机分歧必须过采样。报告同时展示自然分布指标与加权还原后的总体指标，防止过采样改变生产估计。

## 自动评分边界

- 没有完成且未失效的授权冻结集评测：转人工。
- 没有对应学科、学段、题型、风险等级的校准证据：转人工。
- 页面匹配、Crop、转录、公式或评分点出现已知错误：转人工。
- `error_source != none` 但没有路由人工：计入风险漏放。
- R3 长作答和正式高风险考试继续保留人工最终控制。
- 本故事不写死 0.8、0.9 等经验阈值；阈值必须由冻结集按切片标定。

## 后续故事

1. 评分尝试表与三次/五次重复评分，记录中位数、方差和极差。
2. 只接收教师确认案例的 `grading_examples` 与按题目/Rubric 版本隔离的 RAG。
3. 中文手写 PP-OCR 与公式识别引擎的可替换适配器和真实集对比。
4. 对公式结构进行安全解析和符号等价验证。

