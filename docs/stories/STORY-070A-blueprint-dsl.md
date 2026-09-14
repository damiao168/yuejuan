# STORY-070A：版本化考试蓝图 DSL

状态：Planned。优先级：P0。父 Story：[070](STORY-070-assessment-blueprint-smart-assembly.md)。依赖：069B/C。

## 问题与范围

把教师的考试规格保存为可验证的版本事实，为确定性选题提供唯一输入。Blueprint 持有稳定身份，BlueprintVersion 固定 schema、核心规格、约束与版权/用途默认策略；修改已确认版本只能派生新 draft。

- 核心：学科/学段/年级、目标 Bank 范围、item_count、total_score_units、score_scale、duration_minutes、schema_version。
- 约束类型：question_type、assessment_archetype、knowledge_point、cognitive_level、difficulty_band、suggested_time、required/excluded_item、item_family、source、copyright/intended_use；曝光策略版本引用到 070C 落地。
- 明确 hard/soft、count/score/time/ratio 的单位与 min/max，比例必须声明 denominator=count 或 score；非整数题量上下限按 ceil/floor 计算，不能静默四舍五入。
- 第一阶段知识点采用完整覆盖计分，同题可能满足多个知识点约束，界面声明覆盖分不能相加视作总分。题目建议用时单位秒且非负，缺失如何处理由约束显式规定。
- difficulty_source 为 teacher_label 或 observed_score_rate；观测值绑定同 ItemVersion 的统计快照、N、群体、算法与 as_of，缺观测值可拒绝或由显式 fallback 使用标签，不自动填零。
- required/excluded 默认逻辑 Item 级，再固定候选池中的 published Version；同 Item 不得重复入卷。显式指定 Version 时仍检查其归属与发布状态。
- 校验结构、未知字段/枚举、min>max、总分与题型分值冲突、必选禁用交集与确定的容量必要条件；返回 constraint_id 与字段级原因。

## DSL 示例

以下是计划契约示例，所有约束均有稳定 ID；不是当前已上线 API。

```json
{
  "schema_version": 1,
  "score_scale": 100,
  "total_score_units": 12000,
  "item_count": 24,
  "duration_minutes": 120,
  "knowledge_score_mode": "full_coverage",
  "constraints": [
    {
      "id": "choice-count",
      "kind": "question_type",
      "value": "single_choice",
      "strength": "hard",
      "count": {"min": 12, "max": 12}
    },
    {
      "id": "linear-coverage",
      "kind": "knowledge_point",
      "value": "linear_function",
      "strength": "hard",
      "score_units": {"min": 2000, "max": 2500}
    },
    {
      "id": "hard-share",
      "kind": "difficulty_band",
      "value": "hard",
      "strength": "hard",
      "difficulty_source": "teacher_label",
      "ratio": {"min": "0.15", "max": "0.25", "denominator": "count"}
    }
  ]
}
```

既有 float 分值转成 score_units 必须验证可精确表示，超出 score_scale 的输入拒绝；solver 不依赖 float 求精确总分。比例字符串解析为有理数或精确十进制，不用浮点比较边界。

## 数据与 API 规划

新增 assessment_blueprint、assessment_blueprint_version；固定核心列 + constraint DSL JSON。拟定 `GET|POST /api/v1/assessment-blueprints`、`GET|POST /api/v1/assessment-blueprints/{id}/versions`、版本草稿更新/validate/confirm。版本和 schema/hash 不可原地替换，固定 schema_version 的旧蓝图按对应解释器读取，未知版本拒绝。

## 非范围

不实现 solver、LLM 蓝图生成或外部优化 Worker，不拆几十张约束表，不因校验通过宣称存在可行解，不在未授权题库上显示容量。

## 预计修改文件

新增 `internal/testassembly/` DSL types/validator/blueprint store/handlers、蓝图迁移、`docs/api/test-assembly.md`；管理端蓝图编辑/约束反馈、OpenAPI/SDK 与定向测试。

## 测试方式与验收标准

Go table tests 验证分值精度、比例边界、多知识点、矛盾约束与未知 schema；真实 PostgreSQL 验证 revision、确认后不可变、权限与版本重放；OpenAPI/SDK 和 Web 类型构建验证 typed contract。

1. 120 分可无误差表示为 12000 units；count 比例边界计算固定，未知 schema/字段拒绝。
2. 多知识点贡献可解释，覆盖分不是总分；difficulty 标签与观测来源显式且缺值有策略。
3. 每条校验错误带 constraint_id、位置与原因；结构合法与可行性判断分开。
4. 已确认版本不可改，派生新 draft 保留历史；同命令/revision 重试不产生重复版本。
5. Bank 范围、必选项与容量提示都先校验授权，不能借 validate 泄露保密题。

## 规划审阅与实施记录

规划已固定整数分值、比例分母、多知识点和未知 schema 语义。实现、实现审阅、修正与 Approval 待证据；本切片验收后才执行 070B。
