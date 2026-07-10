# 多智能体阅卷工作流

## 原则

Agent 是受控工作流节点，不是聊天角色。每次输出必须可回放、可审计、可解释，并被策略引擎约束。

## 状态机

```text
CREATED
 -> UPLOADED
 -> PREPROCESSED
 -> OCR_DONE
 -> SEGMENTED
 -> RUBRIC_READY
 -> AI_GRADED
 -> VERIFIED
 -> HUMAN_REVIEWING
 -> FINALIZED
 -> PUBLISHED
 -> APPEALING
 -> ARCHIVED
```

## Agent 列表

- 版面解析 Agent：识别题号、页码、答题区域，不自动终审。
- OCR Agent：识别印刷体、手写体、公式，低置信度转人工。
- 答案切分 Agent：按学生和题目切分答案段。
- 脱敏 Agent：去除姓名、学号、班级等身份信息。
- Rubric 解析 Agent：结构化评分标准，需审批。
- 客观题 Agent：低风险题型可自动通过。
- 简答题/计算题/作文 Agent：只提供建议分与证据。
- 证据校验 Agent：检查每个得分点是否有证据位置。
- 一致性 Agent：检测同类答案分数是否一致。
- 异常 Agent：识别空白、答错区域、疑似模板答案等风险。
- 审计 Agent：记录过程日志，不可关闭。

## 人工复核触发

- OCR 置信度低于 0.85。
- AI 评分置信度低于 0.80。
- AI 与规则冲突。
- 与同类答案分差异常。
- 双评分差超过阈值。
- 学生答案过短但得分高，或过长但得分低。
- 命中敏感词、疑似作弊或疑似答错区域。
