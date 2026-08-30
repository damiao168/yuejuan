# 考试资料导入 Benchmark

该目录用于评估“资料角色识别 → 候选提取 → 匹配 → 完整性提示”的端到端质量。真实试卷、答案和解析通常含学生或学校敏感信息，不提交仓库；脱敏 fixture 放入 `fixtures/<case-id>/`，每个案例包含 `manifest.json`、源文件和人工标注 `expected.json`。

覆盖矩阵：text PDF、scan PDF、DOCX、PNG 截图、JPG 手机照片、TIFF；清晰、模糊、旋转、透视、阴影、低照度、屏幕翻拍；语数英、物化生、史地政；选择、判断、填空、计算、公式、简答、论述、作文、图表和实验题。

必须记录：`document_role_accuracy`、`question_detection_precision/recall`、`answer_extraction_accuracy`、`solution_extraction_accuracy`、`question_answer_matching_accuracy`、`missing_answer_detection_precision/recall`、`possible_missing_question_precision/recall`、`score_extraction_accuracy`、`question_type_accuracy`、`field_grounding_rate`、`false_missing_alert_rate`、`conflicting_answer_detection_rate`、`hallucinated_field_rate`。

`hallucinated_field_rate` 是发布阻断指标：输入中没有依据却生成题干、答案、分值、解析或 rubric，均计为 hallucination。低置信度且要求人工核对不计为失败；静默补全计为失败。

`manifest.json` 至少包含：案例 ID、学科、资料类型、图像条件、输入文件顺序、预期 Blueprint（若有）、标注版本和脱敏确认人。`expected.json` 使用 API 的 candidate、source_ref 和 issue code 结构，confirmed 与 suspected 分开标注。

## Fixture 约定

仓库中的 `_template` 是可复制模板，不包含真实试卷。每个评测案例必须满足：

1. `manifest.json` 的 `sources` 顺序就是提交给 PaperImport 的 `document_index` 顺序，不允许依赖文件名或目录自然顺序。
2. 每个 source 记录格式、预期角色、图像情况以及是否需要 OCR；真实材料必须完成脱敏审批后才能进入受控评测数据集。
3. `expected.json` 分开保存 question / answer / solution candidates、确定问题和疑似问题。不存在的字段保持缺失或 `null`，不得为了方便评分而补齐。
4. provenance 标注必须指向 source；OCR 样本还要标注 page、block、bbox，文本样本标注字符范围。

## 指标计算

- detection precision/recall 以人工标注 candidate 为基准，题号相同时仍需核对父子题结构。
- extraction accuracy 只统计输入中有明确证据的字段；缺失字段被模型静默补全同时计入 `hallucinated_field_rate`。
- `field_grounding_rate = 有效来源引用字段数 / 所有非空提取字段数`。
- `false_missing_alert_rate = 实际存在但被报缺失的字段数 / 所有缺失告警数`。
- 冲突答案必须全部保留并产生 `CONFLICTING_ANSWERS`，自动择一按漏检处理。
- 发布门禁要求 `hallucinated_field_rate = 0`；任何非零结果都必须定位 fixture、模型版本和原始 source 后才能放行。

每次评测结果应记录代码提交、AI provider/model、prompt/schema 版本、OCR 引擎版本、fixture 标注版本、逐案例结果和汇总指标。OCR CER 只作为诊断指标，不能代替上述业务指标。
