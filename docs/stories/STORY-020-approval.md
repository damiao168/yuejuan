# STORY-020 自审审批记录

## Story

STORY-020 最终成绩与成绩发布

## 审批结论

Approved

## 验收项

| 验收项 | 证据 | 结论 |
| --- | --- | --- |
| `final_grade` 支持题目最终分和发布状态 | `000015_final_grades_publishing.sql` | 通过 |
| 每个 answer_segment 可生成最终题目分 | `FinalizeExam`、score store 测试 | 通过 |
| 每个 submission 可汇总总分 | `submission_grade`、score store 测试 | 通过 |
| 支持全部成绩状态 | migration check、`Statuses()`、测试 | 通过 |
| 支持成绩确认 | `ConfirmGrades`、route 测试 | 通过 |
| 支持成绩发布 | `PublishGrades`、route 测试 | 通过 |
| 未完成阅卷不能发布 | `TestPublishBlocksUntilGradesConfirmedAndQualityPasses` | 通过 |
| 未完成仲裁不能发布 | `TestPublishBlocksForArbitrationOCRAndMissingFinalGrade` | 通过 |
| OCR 失败不能发布 | 同上 | 通过 |
| 异常分未确认或缺失 final_grade 不能发布 | 同上 | 通过 |
| 发布动作要求权限 | `TestScoreRoutesRequirePermission` | 通过 |
| 发布后学生可查询 | `TestScoreRoutesFinalizeConfirmPublishStudentLookupAndExport` | 通过 |
| 发布后核心分数锁定 | route/store 测试断言 `locked=true` | 通过 |
| CSV 导出 | `ExportGradesCSV`、route 测试 | 通过 |
| 导出水印字段 | CSV 测试断言 `watermark`、`EduGrade export` | 通过 |
| 发布和导出写审计 | route 测试断言 audit action | 通过 |
| 不越界 | 未实现 Web 页面、学生端 UI、申诉、Excel `.xlsx` | 通过 |

## 运行命令与结果

```powershell
Push-Location .\services\api-gateway
gofmt -w internal
go test ./...
Pop-Location
docker compose --env-file .env.example -f infra\docker-compose\docker-compose.yml config
Push-Location .\services\api-gateway
go build -o ..\..\bin\api-gateway.exe .\cmd\api-gateway
Pop-Location
```

结果：

```text
gofmt -w internal -> passed
go test ./... -> passed
docker compose config -> passed
go build -> passed
```

## 剩余风险

- 学生端查询是后端 API，学生账号与学生档案的强绑定和学生端 UI 尚未实现。
- 导出为 CSV，不是 `.xlsx`。
- 发布后改分申请和申诉流程尚未实现。
- 成绩排名、统计分析和报表留给后续 Story。

## 下一步

进入 `STORY-021 申诉流程`。
