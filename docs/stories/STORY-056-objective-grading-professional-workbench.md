# STORY-056 客观题评分引擎和专业人工阅卷工作台

## Status

Approved (2026-07-15)

## First-principles decision

STORY-055 已经把原始扫描文件可靠地转化为按题、可追溯的 `answer_segment` 图片。此时离用户获得价值只差一件最直接的事：把每个答案变成可解释、可复核、不会静默丢失的评分结果。

因此下一个 Story 不先做更复杂的 AI、答案聚类、质量中心或账号治理，而是完成最短生产闭环：

```text
可信答题区域
-> 题型专用答案提取
-> 版本化确定性规则评分
-> 不确定结果转人工
-> 阅卷员连续、可靠地完成评分
-> 形成可供 STORY-058 质量控制使用的评分事实
```

产品的第一原则不是“尽量自动给分”，而是“每一分都有来源，不能确定时交给人，并让人高效且不易出错地完成”。

## Goal

将现有按单个 segment 手工触发“记录答案/规则判分”的技术页面，升级为考试负责人和阅卷员可以独立使用的生产流程：批量为当前考试生成评分任务；选择题/判断题使用真实 OMR；多选、填空和数字题使用版本化规则；所有歧义、低置信、缺少配置和处理失败自动进入人工队列；阅卷员在匿名、按题、可自动保存且支持冲突恢复的工作台中连续阅卷。

完成后的主流程：

```text
采集批次移交 OCR/评分
-> 按题型路由答案提取
-> OMR/OCR/人工答案形成不可变候选
-> 评分规则快照与确定性评分
-> 明确结果自动确认或待负责人抽查
-> 歧义结果创建人工阅卷任务
-> 阅卷员领取/恢复任务
-> 自动保存草稿
-> 提交并进入下一份
-> 生成带版本和证据的题目评分事实
```

## User outcomes

- 考试负责人只需在考试工作区点击一次“开始评分”，系统即可根据当前有效 segment、题型和规则批量推进，不需要逐题调用 API。
- 负责人看到按题目的总数、待处理、自动评分、人工待阅、失败和阻断原因，并可从问题直接进入修复位置。
- 阅卷员进入“我的阅卷”即可继续上次位置，默认按同一题连续阅卷，学生身份默认隐藏。
- 每份答案都展示真实裁图、题干、满分和 Rubric；OCR、规则结果以后续 STORY-057 的 AI 结果都只是辅助证据。
- 草稿自动保存；网络中断、刷新或误关闭后可以恢复；并发修改不会最后写入静默覆盖。
- 单选/判断明确涂卡可自动评分；空白、多涂、浅涂、擦除、污染和低置信必须进入人工确认。
- 分数来源、答案候选、规则版本、人工修改、撤销和提交均可审计和重现。

## Existing capability and real gaps

### Reuse

- STORY-054 已锁定题目、答案、Rubric、题目区域和开考 readiness。
- STORY-055 已生成真实 crop asset，并保留模板、配准、题目和文件 hash。
- `grading.Engine` 已支持 single choice、true/false、multiple choice、fill blank、numeric 的基础规则。
- `answer_segment_answer`、`ai_grade`、`review_task`、`human_grade`、`final_grade` 已提供初始数据边界。
- `GradingWorkbenchPage` 已能展示页面、OCR、Rubric、提交人工分和基础快捷键。
- PostgreSQL Worker Runtime 已具备 claim、lease、retry、dead-letter、取消和审计语义。

### Gaps that block real use

- 当前评分必须由用户逐个 segment 点击“记录答案”和“规则判分”，没有考试级编排、进度或自动建人工任务。
- 选择题答案仍依赖文本或 payload，没有从 STORY-055 crop 图片真实执行 OMR。
- 当前规则版本是代码常量 `objective-rules-v1`，多选只有简单比例部分分；填空规范化、数字单位/相对误差/舍入等不足以表达真实考试规则。
- `ai_grade` 被复用于确定性规则结果，语义混杂；缺少独立的评分 run、输入 hash、规则快照和当前有效版本。
- 基础引擎可能把 OCR 低置信文本匹配结果标记 `auto_pass`，没有把上游置信度、图像质量、配准状态纳入自动确认门禁。
- 工作台跨多个接口拼上下文，存在 N+1；显示原始整页而不是优先显示真实 segment crop；没有正式任务领取、租约、草稿版本或冲突恢复。
- “主观题 AI”按钮仍明确调用 mock，STORY-056 的正式界面不能把它呈现为可用生产能力。
- 当前任务列表不以考试、题目、本人、优先级和状态形成生产队列，也不能稳定支持下一份预加载。

## Supported scope

第一版正式支持：

- 固定版式答卷中的单选、判断、多选、填空、数字答案和主观题人工评分。
- 单选、判断和规范涂卡多选题的 OpenCV OMR 候选提取。
- 填空题精确/等价答案匹配和显式规范化规则。
- 数字题整数、小数、科学计数法、百分数、分数、绝对/相对误差、有效数字和受控单位换算。
- 题目级评分规则版本、发布、锁定和考试快照。
- 考试级评分启动、幂等重跑、进度聚合、问题队列和受影响项重算。
- 匿名、按题连续人工阅卷，草稿自动保存、快捷键、冲突处理、退回处理和下一份预加载。
- 规则自动确认与人工评分形成统一但来源明确的题目评分事实。

明确不承诺：

- 真实主观题 AI、视觉语言模型、答案分组和模型门禁，留给 STORY-057。
- 校准卷、种子卷、抽检、双评运营和仲裁质量中心，留给 STORY-058；本 Story 只保证已有双评数据模型不被破坏。
- 复杂数学表达式等价判定。SymPy 路线先写入后续候选，不能在没有沙箱、超时和专项评测时进入自动评分。
- 任意样式 OMR。正式支持范围限于模板中已定义选项区域或可从题目区域稳定推导的标记区域。
- 依赖模糊字符串相似度自动给分；不能精确证明的答案进入人工复核。
- AI 自动写最终成绩或自动发布成绩。

## Mature product and open-source references

- Gradescope 的固定模板、按题阅卷、Rubric、Next Ungraded 和 Answer Groups 证明：减少上下文切换、让人工确认建议分组，比追求无人评分更贴合真实阅卷。STORY-056 借鉴按题连续阅卷和 Rubric 快捷操作，不复制其分组能力，分组留给 STORY-057。
- Moodle 的 Rubric/marking guide 和 marking workflow 说明评分状态、反馈和 Rubric 必须是一个工作流，而非单个分数输入框。
- OMRChecker 等 OpenCV 项目验证灰度、阈值、轮廓/连通域、填充率和模板化区域是可行起点；本项目只借鉴算法管线，不直接嵌入其 CLI、模板或评分模型。
- OpenCV 继续作为 OMR 技术底座，复用 STORY-055 的配准图和模板坐标，避免再次做页面定位。
- Go 标准库负责确定性文本与数字规则；单位换算第一版采用项目内受控单位表和维度校验，不引入重量级运行时。
- SymPy 只作为以后数学表达式候选。其通用表达式解析不能直接接收学生不可信输入，未经允许符号、AST 限制、隔离进程和超时验证不得上线。

## Technical route

### Architecture

- 保持 Go 模块化单体、PostgreSQL、MinIO、现有 Python page-processing worker、React/TypeScript/Ant Design 和 Worker Runtime。
- OMR 是图像推理，放入现有 `page-processing-worker` 的新任务类型 `omr_extract`，复用 OpenCV、registered page/crop、模板版本和 worker 协议，不新增服务、消息队列或数据库。
- 确定性评分保留在 Go `grading` 模块中，便于事务、版本快照、权限和审计；不为了几个规则引入 Python RPC。
- 评分编排由 Go API 在同一事务内创建不可变 scoring run 和 worker tasks；浏览器不负责逐份排队。
- 前端新增考试级聚合上下文 API，不继续由工作台请求任务、segment、page、question、rubric、OCR、grade 等多个资源自行拼接。

### OMR extraction

输入必须是 STORY-055 当前有效的 registered crop，不使用原始歪斜页：

```text
验证 crop/template/hash
-> 灰度与局部对比度检查
-> Otsu + 自适应阈值双路候选
-> 形态学去噪
-> 按模板选项区域计算墨迹/填充特征
-> 与空白模板或邻域背景做差
-> 判定 single / blank / multiple / ambiguous
-> 输出每个选项证据与 overlay
```

- 每个题目模板应显式保存选项标记区域；缺失时必须阻止正式 OMR，不凭均分网格猜位置。
- 每个选项保存 fill ratio、foreground delta、阈值、轮廓/连通域摘要和 bbox。
- 保存二值化/差分 overlay 作为受保护证据资产；不覆盖 segment crop。
- 阈值来自版本化 `omr_profile`，按试卷模板可覆盖；生产阈值必须由项目 fixture 和后续授权样本校准。
- 自动确认只允许唯一明确选项、上游质量/配准通过、margin 达标且无擦除/污染风险；其余创建人工确认任务。
- 多选题先提取候选集合，再由 Go 评分规则计算分数；图像识别与计分策略严格分离。

### Versioned scoring rules

新增题目级 `scoring_rule` 版本，不再只依赖代码常量和自由形态 tolerance：

- `single_choice` / `true_false`：精确答案；blank/multiple/ambiguous 不自动判零，转人工。
- `multiple_choice`：完全正确分、少选计分模式、每项分值、错选扣分、最低分和上限均显式配置。
- `fill_blank`：Unicode NFKC、全半角、大小写、空白、标点规则均默认保守并逐题开启；多个允许答案和同义答案显式维护；不使用模糊匹配自动评分。
- `numeric`：Decimal 语义解析整数、小数、科学计数法、百分数和分数；支持绝对/相对误差、有效数字、舍入模式、单位必填和受控同维单位换算。
- `manual`：简答/主观题直接创建人工任务，不制造 0 分建议。

每次评分固化：`question_version`、`answer_version`、`rubric_version`、`scoring_rule_version/hash`、`segment/crop hash`、`answer_candidate_id`、引擎版本和时间。规则或输入变化只使受影响结果失效，并生成新 run，不覆盖历史。

### Confidence and decision policy

系统不把来源模型给出的一个 0-1 数字直接当成自动评分依据。自动确认必须同时满足：

- capture page、quality、registration、segment 均是当前有效且通过门禁的版本。
- OMR 或 OCR 输出结构合法、来源可追溯、达到题型 profile 阈值。
- 答案唯一、无冲突、无空白/多选/擦除/越界风险。
- 当前规则已发布且锁定，输入题型与规则类型一致。
- 评分可以被确定性规则证明，分数在 `[0, max_score]` 内。

任何一项不满足都产生业务可读的 review reason，不静默判错，不阻塞其他答案继续处理。

## Data model

迁移顺序接续 STORY-055 的 `000035`，新增 `000036_story056_grading_workbench.sql`，不得修改已应用迁移。

### `scoring_rule`

- `id`, `tenant_id`, `exam_id`, `question_id`, `version`, `rule_type`, `config`
- `status` (`draft/published/retired`), `content_hash`, `created_by`, `published_by`, timestamps
- 发布后不可修改；考试开始评分时保存当前规则快照。

### `answer_candidate`

- 不可变候选：`answer_segment_id`, `source` (`omr/ocr/manual`), `payload`, `display_text`
- `confidence`, `decision`, `evidence`, `engine/profile/version`, `input_hash`
- `is_current` 只由服务端在新 run 提交时事务切换；人工修正创建新候选而非覆盖机器结果。

### `omr_run`

- `answer_segment_id`, `crop_file_asset_id/hash`, `template_id/hash`, `profile_version/hash`
- 状态、attempt/result version、选项 measurements、decision、confidence、risk flags
- overlay asset、worker、耗时、错误分类和时间。

### `scoring_run` and `question_grade`

- `scoring_run` 是考试或题目级编排 run，保存 scope、规则快照、状态和聚合计数。
- `question_grade` 是统一评分事实，来源为 `rule_confirmed` 或 `human`，关联 segment、submission、question、candidate、rule、review task。
- 保存 score/max、status、source、evidence、version、supersedes、created/confirmed by；历史不可覆盖。
- 现有 `ai_grade` 继续承载建议结果，后续由 STORY-057 使用；不再承载新的确定性最终评分事实。

### `review_draft` and task upgrade

- 草稿绑定 `review_task_id + reviewer_id`，保存 score、Rubric selections、feedback、viewer state、version 和 updated_at。
- `review_task` 增加 claim/lease 或等价的短期所有权、revision、last_opened_at、reason_code 和 current_grade_id。
- 任务分配与题目/考试授权同时检查；匿名码必须是考试域内稳定随机值，不可推导学生 ID。

## API contract

### Scoring orchestration

- `POST /api/v1/exams/{examId}/scoring-runs`：按当前有效 segments 幂等创建 run。
- `GET /api/v1/exams/{examId}/scoring-summary`：一次返回总进度、按题进度和问题计数。
- `GET /api/v1/scoring-runs/{id}`、`POST /api/v1/scoring-runs/{id}/retry-failed`、`POST /api/v1/scoring-runs/{id}/cancel`。
- `POST /api/v1/answer-segments/{id}/reprocess-score`：仅重跑受影响 segment。

### Rule configuration

- `GET/POST /api/v1/questions/{id}/scoring-rules`
- `GET/PATCH /api/v1/scoring-rules/{id}`（仅 draft，revision 乐观锁）
- `POST /api/v1/scoring-rules/{id}/publish`
- STORY-054 准备检查增加“客观题已发布评分规则/主观题已配置 Rubric”。

### Workbench

- `POST /api/v1/review-tasks/next`：按本人授权、考试、题目和过滤器原子领取下一份。
- `GET /api/v1/review-tasks/{id}/workspace`：聚合任务、segment crop、题干、Rubric、候选、建议和历史摘要。
- `PUT /api/v1/review-tasks/{id}/draft`：幂等自动保存，携带 revision。
- `POST /api/v1/review-tasks/{id}/submit`：提交时验证任务 revision、规则/Rubric 版本和权限。
- `POST /api/v1/review-tasks/{id}/return`：结构化原因 + 说明。
- `POST /api/v1/review-tasks/{id}/release`：退出时主动释放；超时自动回收。

所有写接口拒绝未知字段，验证 tenant/exam/question/submission/segment/task/asset 同域归属，使用请求幂等键并写审计。

## Frontend product design

### Exam grading overview

- 考试工作区“阅卷”先显示业务进度，不直接进入某个随机 review task。
- 顶部显示待开始/处理中/阅卷中/有问题/已完成，以及“开始评分”“继续处理问题”“进入我的阅卷”。
- 主体使用按题高密度表格：题号、类型、答卷数、自动确认、待人工、已阅、退回、失败、阻断原因。
- 从阻断原因直接导航到缺失规则、异常 segment 或待处理任务。
- 技术 task id、lease、worker 名称仅在运维诊断中显示。

### Professional workbench

桌面端是正式主体验，最小支持 1280px；窄屏提供只读/应急操作，不承诺手机长时间阅卷。

```text
顶部 56px：考试 / 题目 / 进度 / 保存状态 / 快捷键 / 退出
左侧 240px：题目与任务导航、过滤、状态
中间自适应：segment 裁图为主，原图/配准图按需切换
右侧 380px：Rubric、答案候选、规则/AI 辅助、分数和反馈
底部固定：上一份、退回、保存、保存并下一份
```

- 默认显示 segment crop 并预加载下一份；原图、标准化图和配准图用于上下文核查。
- 支持缩放、拖动、旋转、适应宽度、实际大小、全屏、OCR/OMR 证据 overlay。
- 任务默认按题组织，减少 Rubric 上下文切换；允许按学生查看但不作为默认方式。
- 自动保存显示“正在保存/已保存/离线待保存/冲突”；不得用 toast 冒充持久化成功。
- 冲突时提供查看新版本、保留本地草稿和重新应用，禁止最后写入覆盖。
- 快捷键支持 Rubric、保存、保存并下一份、导航、适应宽度和帮助；输入框聚焦时不得误触，用户可关闭。
- 生产界面移除 mock 主观题 AI 操作；只有 STORY-057 真实模型通过门禁后才作为辅助面板启用。

## Security, privacy and audit

- 阅卷员只可领取其考试和题目授权范围内任务；前端过滤不是权限边界。
- 默认匿名阅卷，姓名、学号、班级和文件名不进入 workspace 响应；解除匿名留给授权管理流程并必须审计。
- segment 和 evidence 图片继续通过受保护 API/短时受控响应提供，不暴露公开 MinIO URL。
- 草稿、私密备注和学生反馈区分权限与发布可见性。
- OMR/OCR 原始结果、人工修正、规则评分和最终人工分全部保留，禁止覆盖机器证据。
- 记录评分 run 启动/取消、规则发布、自动确认、任务领取/释放、草稿冲突、人工提交、退回和重算审计。

## Error, recovery and consistency

- Worker 不可用显示“等待答案识别”，不误判为空白或 0 分。
- 单个 OMR 失败只影响该 segment；达到重试上限进入人工队列并保留 crop。
- 用户离线时草稿保存在浏览器受控缓存并显示未同步；恢复连接后使用 revision 提交，冲突不自动覆盖。
- 浏览器崩溃或任务 lease 到期后，服务端草稿仍可由同一用户恢复。
- 规则、答案 key、segment、模板或人工答案变更时，只失效依赖它的当前 grade，历史结果继续可查。
- scoring run 和任务创建使用数据库事务/outbox，不能出现“答案已识别但永远没有评分任务”。
- 自动评分批量重试保持幂等，不生成多个当前有效 question grade。

## Acceptance criteria

- [x] 考试负责人可从 UI 一次启动整场考试评分，刷新后进度可恢复。
- [x] STORY-055 当前有效 segment 自动进入题型路由，不需逐个点击 API。
- [x] 规范单选、判断和多选 crop 真实执行 OpenCV OMR，并保存选项级证据和 overlay。
- [x] blank/multiple/ambiguous/low-confidence 不自动判错，全部进入人工任务。
- [x] 多选、填空和数字题规则可在 UI 配置、校验、发布和版本化。
- [x] 填空不使用模糊字符串自动给分；数字规则覆盖约定格式、误差和单位边界。
- [x] 自动确认同时检查图像、配准、答案和规则门禁；低置信上游不能获得 `auto_pass`。
- [x] 规则结果写入独立 versioned scoring fact，不继续把新规则结果混入 `ai_grade`。
- [x] 主观题和不确定客观题自动创建真实 review task。
- [x] 阅卷员可按本人授权领取下一份，并按同一题连续阅卷。
- [x] 工作台优先显示真实 segment crop，支持原图/配准图和 OMR/OCR overlay 核查。
- [x] 草稿自动保存、刷新恢复、离线状态和 revision 冲突处理真实有效。
- [x] 匿名模式不向阅卷 workspace 返回学生身份；对象级越权请求全部拒绝。
- [x] 提交后自动进入下一份，分数/Rubric 合计和版本均由服务端验证。
- [x] 单项失败、worker 重启和重复 scoring 请求均可恢复且保持幂等。
- [x] Go/Python/Web 测试、PostgreSQL migration、Compose E2E 和 Playwright 真实流程通过。
- [x] 100 名 synthetic/脱敏学生混合题型 fixture 无丢任务、重复当前分或静默 0 分。
- [x] 桌面 1280/1440/1920 视口无关键重叠；390px 明确降级且无数据误操作。

## Verification plan

```text
Go: scoring rules, decimal/numeric/unit, task orchestration, idempotency, invalidation,
    authorization, anonymous workspace, draft revision and submission tests
Python: OMR normal/blank/multiple/light/erased/polluted/skew fixtures,
        threshold profile, overlay, limits, retry and deterministic output tests
Database: 000036 clean apply/rollback, composite FK, current-version uniqueness,
          concurrent task claim and grade commit tests
Web: rule editor and workbench component tests, typecheck/build
Compose: segment -> OMR/OCR -> rule -> review task -> human grade real flow
Playwright: start scoring, resolve ambiguous OMR, autosave/reload, conflict,
            save-and-next, return, permission denial and anonymous identity check
Acceptance: >=100 synthetic/authorized submissions; report OMR confusion matrix,
            ambiguous recall, manual routing rate, throughput and zero silent-score count
```

## Plan review

### Why this is the correct next Story

1. STORY-055 ends at reliable question crops. Without STORY-056, those assets do not become grades and the system still requires technical operators; this is the nearest broken user outcome.
2. STORY-057 AI requires a trustworthy human baseline and a production workbench for mandatory verification. Building AI first would optimize a suggestion before the system can reliably accept or correct it.
3. STORY-058 quality control requires real reviewer actions, timing, task status and versioned grades. STORY-056 supplies those facts without prematurely implementing quality policy.
4. Account governance is still production-critical, but it does not close the exam value chain at this point and remains scheduled for STORY-060 alongside service accounts and operations.

### Scope corrections from previous planning

1. The old production roadmap assigned different meanings to STORY-054/055/056. The V1.0 master delivery plan is authoritative for numbering: STORY-056 is objective grading plus professional workbench.
2. OMR is included because OCR is the wrong instrument for mark recognition and current code has no real image-to-choice path.
3. The existing `grading.Engine` is retained but treated as a foundation, not proof of production readiness. Its code constant, simplistic policy and missing orchestration must be replaced by versioned exam facts.
4. The existing workbench is retained and incrementally productized; a frontend rewrite would add risk without improving the core user outcome.
5. SymPy and broad formula grading are excluded because secure expression parsing and semantic equivalence require a separate bounded subsystem and real evaluation data.
6. Answer grouping is excluded even though Gradescope demonstrates its value; it depends on OCR/embedding/model governance and belongs in STORY-057.
7. Double marking and arbitration APIs are not expanded here. STORY-056 must remain compatible, while assignment, calibration, sampling and quality intervention belong in STORY-058.

### Production risks to resolve during implementation review

- Thresholds cannot be claimed accurate from synthetic fixtures alone; external authorized answer-sheet evaluation remains an explicit acceptance dependency.
- Existing question/answer-key JSON must be migrated or adapted without invalidating exams already created in STORY-054.
- Current `final_grade` semantics overlap automatic and human outcomes; implementation must prove one current grade per segment without deleting history.
- Offline browser draft storage may contain sensitive feedback; it needs bounded lifetime, per-user isolation and cleanup on logout.
- Automatically confirmed 0 scores are especially high risk; the acceptance report must separately measure false-zero rate and require zero known silent ambiguous-to-zero cases.

## Decision

Proceed with STORY-056 after specification review. Do not begin STORY-057 until STORY-056 has completed implementation review, fixes and approval.

## Final implementation review and approval (2026-07-15)

Implementation review fixes are complete:

- Migrations `000036` through `000042` add the STORY-056 schema incrementally. Applied migrations `000033` through `000035` remain unchanged, and the clean PostgreSQL chain recorded the expected `000036` SHA-256 `14c1f6e21f966afb23a4168fcc981888586899f1741db484c88188f280648d15`.
- Scoring-run detail, cancellation, failed-item retry and segment-only reprocessing are wired to Worker Runtime cancellation/requeue semantics and covered by recovery and lease tests.
- Browser draft fallback is bounded, keyed by reviewer and task, revision-aware, and removed on submission or logout. Server drafts remain authoritative after reconnect.
- Human submission now increments `human_confirmed_count` instead of the automatic counter. The 100-case PostgreSQL acceptance proves 67 automatic confirmations plus 33 human confirmations without duplicate current grades or silent zeroes.
- The production workbench uses the exam-scoped route, refreshes scoring progress after submission, and requests the next task only when a pending task exists. An empty queue is a completed state, not a browser-visible 404.
- Desktop layout is constrained to the remaining viewport; mobile metadata and topbar controls wrap or truncate without character-by-character UUID rendering, overlap, or horizontal overflow.

Independent acceptance evidence:

- A clean isolated Docker project applied all 42 migrations and passed the real PostgreSQL server suite. The suite completed 100 durable OMR result transitions in 9.406 seconds.
- The isolated seed created an unapproved-calibration review case, the real Python Worker claimed and processed one task, and the verification phase passed private overlay/hash/permission and durable writeback checks.
- Playwright used the rebuilt Docker Web image and real UI to move `total=1, auto=0, human=0, review=1` to `completed, human=1, review=0`. PostgreSQL confirmed a current `question_grade` with `source=human`, `status=confirmed`, and score `1.00`.
- The authenticated fresh browser tab recorded zero console errors and only 200/201 business responses. At 320px and 390px controls remain inside the viewport with no overlap or horizontal overflow; at 1440x1000 the immersive grading shell ends exactly at the viewport boundary. Screenshots are retained under ignored `output/playwright/`.
- Go tests, 31 Python Worker tests, Web production build, production-route checks, migration checks and `git diff --check` pass.
- External governance covers SurveySet (privacy/domain-gap review only) and the CC BY 4.0 Tamaulipas v2 subset (20 sheets, 1,800 items). The production gate auto-confirmed 4/4 correct items, routed all 24 unsafe X/M labels to human review, and produced zero silent unsafe-to-zero cases.

Approval decision:

**Approved for the STORY-056 scope.** The evaluated external template remains `manual_only`: automatic precision was 100% on only four gated items and coverage is insufficient for template approval. A future production template still requires a locked blank reference, at least 100 governed calibration samples, per-option coverage, explicit approval, and revocation. This Story approval does not approve STORY-057 through STORY-059 and does not declare the overall product Production Ready.
