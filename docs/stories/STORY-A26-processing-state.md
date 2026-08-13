# STORY-A26：统一 Processing State 与异常中心

状态：Implemented（当前工作树）

## 范围

- 将接收、质量、配准、身份、OCR、分割等既有事实投影为页面处理状态和可运营异常。
- 异常支持查询、重试、指派和解决；不通过异常页面直接覆盖原始处理事实。
- 专用 parser quality 可作为 A14 Eligibility 输入，缺失时保守转人工。

## 接线

- `internal/processing` 提供考试摘要、异常列表及处置接口，并在真实 PostgreSQL 组装中注册。
- 管理端答卷采集页展示处理摘要与异常动作；A14 Subjective 链路读取 parser quality 提供者。

## 验证

- Processing/Subjective/Server 定向 Go 测试、Web 类型检查和生产构建已执行。

## 外部边界

- 公式、表格、图表等专用 parser 的真实质量与复杂版面吞吐尚未验证；没有质量证据时系统不得将其视作自动评分通过。
