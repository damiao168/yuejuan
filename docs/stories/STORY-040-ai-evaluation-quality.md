# STORY-040 AI 评估集与评分质量测试

## Plan

### 目标

为 EduGrade Enterprise 建立第一版 AI 阅卷评估框架，用可复现的 synthetic evaluation set 衡量 AI 建议分与人工分的一致性，避免凭主观感觉判断模型或 Prompt 质量。

### 范围

1. 创建并填充 `tests/ai-evaluation`。
2. 定义评估样本格式，必须包含：
   - `question`
   - `rubric`
   - `answer`
   - `human_score`
   - `human_rationale`
   - `expected_points`
3. 定义预测结果格式，包含：
   - `sample_id`
   - `model_version`
   - `prompt_version`
   - `suggested_score`
   - `confidence`
   - `needs_human_review`
   - `risk_flags`
   - `matched_points`
4. 实现离线评估脚本：
   - 不调用真实 AI 服务。
   - 从 JSONL synthetic 样本与 JSONL 预测结果计算指标。
   - 输出 JSON 和 Markdown 报告。
5. 计算指标：
   - MAE
   - RMSE
   - exact agreement
   - adjacent agreement
   - score bias
6. 支持按 `model_version` 对比。
7. 支持按 `prompt_version` 对比。
8. 统计低置信度转人工策略：
   - 低置信度数量与比例。
   - 已转人工数量与比例。
   - 低置信度且转人工覆盖率。
   - 需要人工但未转人工的风险项。
9. 添加小型 synthetic evaluation set，不包含真实学生隐私数据。
10. 文档说明如何扩展评估集和接入真实模型输出。

### 非范围

1. 不实现真实模型调用、真实 OCR、真实 LLM worker 或在线评测服务。
2. 不把 synthetic/mock 预测结果宣传为真实模型能力。
3. 不引入外部 Python 依赖，避免增加私有化部署复杂度。
4. 不进入 STORY-041 E2E 测试。
5. 不把本评估结果作为上线准入的唯一依据；当前只是质量评估框架与样例集。

### 预计新增文件

1. `tests/ai-evaluation/README.md`
2. `tests/ai-evaluation/evaluate.py`
3. `tests/ai-evaluation/samples/synthetic_subjective_v1.jsonl`
4. `tests/ai-evaluation/predictions/synthetic_mock_predictions.jsonl`
5. `tests/ai-evaluation/test_evaluate.py`
6. `docs/stories/STORY-040-ai-evaluation-quality.md`
7. `docs/stories/STORY-040-approval.md`

### 预计修改文件

1. `docs/stories/README.md`
2. `tests/README.md`

### 验收标准

1. `tests/ai-evaluation` 下存在样本、预测、评估脚本、测试和说明文档。
2. 样本文件显式标注 `synthetic: true`，不含真实学生身份、姓名、学号或隐私数据。
3. 评估脚本能生成 JSON 与 Markdown 报告。
4. 报告包含 MAE、RMSE、exact agreement、adjacent agreement、score bias。
5. 报告能按 `model_version` 与 `prompt_version` 分组对比。
6. 报告包含低置信度转人工策略统计。
7. 单元测试覆盖指标计算、分组对比、低置信度统计和报告生成。
8. 文档说明如何扩展评估集，且明确样例预测不代表真实模型能力。

## Plan Review

1. 范围没有越过 STORY-040：只做评估框架、synthetic 数据和离线报告，不做 E2E 和真实模型服务。
2. 原始功能要求全部覆盖：目录、样本格式、脚本、五类指标、JSON/Markdown 报告、model/prompt 对比、低置信度转人工统计、synthetic evaluation set 均已纳入。
3. 与当前仓库匹配：`tests/ai-evaluation` 已存在为空目录，AI 服务仍是 placeholder；采用离线脚本最符合“不要假装真实 AI 能力”的项目原则。
4. 风险控制：样例预测文件会标记为 synthetic/mock baseline，报告中也会输出能力边界，避免夸大模型能力。

## Implementation

### 评估目录与文档

1. 在 `tests/ai-evaluation` 下新增完整离线评估框架。
2. 新增 `README.md`，说明：
   - 所有样本与预测均为 synthetic。
   - 不允许使用真实学生姓名、学号、答卷或学校隐私数据。
   - 如何扩展 evaluation set。
   - 如何接入真实模型输出文件。
   - 指标定义和运行命令。

### Synthetic Evaluation Set

新增 `samples/synthetic_subjective_v1.jsonl`，包含 5 条 synthetic 样本：

1. 科学短答：光合作用。
2. 历史短答：工业革命因果。
3. 数学计算：矩形面积。
4. 写作短文：校园花园论证。
5. 安全风险样本：包含 Prompt 注入式句子，但不包含真实学生数据。

每条样本均包含：

1. `sample_id`
2. `synthetic: true`
3. `question`
4. `rubric`
5. `answer.synthetic: true`
6. `human_score`
7. `human_rationale`
8. `expected_points`

### Synthetic Prediction Fixtures

新增 `predictions/synthetic_mock_predictions.jsonl`，包含 15 条 synthetic/mock 预测：

1. `mock-llm-v1 / prompt-v1`
2. `mock-llm-v1 / prompt-v2-guarded`
3. `mock-llm-v2 / prompt-v2-guarded`

预测文件用于验证评估框架的模型版本与 Prompt 版本对比能力，不代表真实模型能力。

### 评估脚本

新增 `evaluate.py`，无第三方依赖，支持：

1. 读取 JSONL 样本与预测。
2. 校验样本和预测必须显式 `synthetic: true`。
3. 校验必要字段、分数区间、置信度区间和预测引用的样本 ID。
4. 计算：
   - MAE
   - RMSE
   - exact agreement
   - adjacent agreement
   - score bias
5. 按以下维度分组：
   - overall
   - `model_version + prompt_version`
   - `model_version`
   - `prompt_version`
6. 统计低置信度转人工策略：
   - 低置信度数量与比例。
   - 低置信度转人工覆盖率。
   - 总体转人工比例。
   - 高置信度转人工数量。
   - 风险标记但未转人工样本 ID。
7. 输出：
   - `reports/ai_evaluation_report.json`
   - `reports/ai_evaluation_report.md`

### 单元测试

新增 `test_evaluate.py`，覆盖：

1. 五类质量指标计算。
2. 低置信度转人工统计。
3. 风险标记但未转人工样本识别。
4. 按模型版本和 Prompt 版本分组。
5. JSON/Markdown 报告生成。
6. 非 synthetic 样本拒绝。

## Implementation Review

1. `tests/ai-evaluation` 已存在并填充脚本、样本、预测、报告和测试。
2. 样本格式覆盖原始要求中的 `question`、`rubric`、`answer`、`human_score`、`human_rationale`、`expected_points`。
3. 评估脚本已实现并生成 JSON/Markdown 报告。
4. 报告包含 MAE、RMSE、exact agreement、adjacent agreement、score bias。
5. 报告按 `model_version` 与 `prompt_version` 分组输出。
6. 报告包含低置信度转人工策略统计。
7. Synthetic 样本和预测均显式标记 `synthetic: true`。
8. 报告与 README 均明确说明样例预测可能是 mock baseline，不代表真实模型能力。
9. 不调用真实模型、不访问真实学生数据、不提前进入 STORY-041。

## Fixes

1. 实现审阅时发现 `prompt-v2-guarded` 的 mock fixture 过于完美，容易被误读为真实能力，因此将其中一条预测调整为轻微误差，报告变为更克制的 synthetic baseline。
2. Python 单元测试初次运行生成 `__pycache__`，已清理，并将验证命令改为设置 `PYTHONDONTWRITEBYTECODE=1`。
3. 报告保留强提示：Synthetic evaluation only，不可作为真实模型生产能力证明。

## Verification

1. `$env:PYTHONDONTWRITEBYTECODE='1'; python -m unittest discover -s .\tests\ai-evaluation`：通过，4 个测试通过。
2. `python .\tests\ai-evaluation\evaluate.py --samples .\tests\ai-evaluation\samples\synthetic_subjective_v1.jsonl --predictions .\tests\ai-evaluation\predictions\synthetic_mock_predictions.jsonl --out-dir .\tests\ai-evaluation\reports`：通过。
3. 报告生成：
   - `tests/ai-evaluation/reports/ai_evaluation_report.json`
   - `tests/ai-evaluation/reports/ai_evaluation_report.md`
4. 检查报告包含：
   - `Synthetic evaluation only`
   - MAE/RMSE 表格
   - model version 对比
   - prompt version 对比
   - low confidence routing 统计
5. 检查 `tests/ai-evaluation/__pycache__`：不存在。

## Approval

批准。

STORY-040 的显式要求均已完成：`tests/ai-evaluation` 已包含 synthetic 样本格式、离线评估脚本、JSON/Markdown 报告、模型版本和 Prompt 版本对比、低置信度转人工统计、小型 synthetic evaluation set、单元测试和扩展说明。

### 剩余风险

1. 当前 prediction fixture 是 synthetic/mock baseline，不代表真实模型能力。
2. 指标只衡量建议分与人工分的一致性，不覆盖公平性、鲁棒性、学科泛化或 Prompt 注入红队能力。
3. 当前评估集规模很小，只适合作为框架样例和 CI smoke test。
4. 后续接入真实模型输出时，需要增加数据治理流程，确保不混入学生隐私数据。
