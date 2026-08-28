import { Checkbox, Radio, Select } from "antd";
import type { Grade, SchoolClass } from "../../../../api/org";
import { examSubjectLabel } from "../../../../constants/examStatus";
import { subjectScore } from "../createExamDraft";
import { examTypeLabel, gradingPresentation, publishPolicyOptions } from "../presentation";
import type { CreateExamDraft, GradingChoice } from "../types";

export function ReviewCreateStep({ draft, grades, classes, onChange }: { draft: CreateExamDraft; grades: Grade[]; classes: SchoolClass[]; onChange: (patch: Partial<CreateExamDraft>) => void }) {
  const grade = grades.find((item) => item.id === draft.gradeId);
  const selectedClasses = classes.filter((item) => draft.classIds.includes(item.id));
  return <div className="exam-review-create">
    <section className="exam-review-summary"><h3>创建内容</h3><dl><div><dt>考试</dt><dd>{draft.name} · {examTypeLabel(draft.examType)}</dd></div><div><dt>范围</dt><dd>{grade?.name ?? "-"} · {selectedClasses.map((item) => item.name).join("、")}</dd></div><div><dt>科目工作区</dt><dd>{draft.subjects.length} 个</dd></div></dl>
      <div className="exam-review-subjects">{draft.subjects.map((subject) => <div key={subject.subject}><strong>{examSubjectLabel(subject.subject)}</strong><span>{subject.totalScore} 分 · {subject.durationMinutes} 分钟 · {subject.sections.reduce((sum, item) => sum + item.questionCount, 0)} 题</span><em>{subjectScore(subject) === subject.totalScore ? "分值已校验" : "分值不一致"}</em></div>)}</div>
    </section>
    <section className="exam-review-policy"><h3>阅卷与发布</h3><Radio.Group className="exam-choice-list grading-policy-list" value={draft.gradingMode} onChange={(event) => onChange({ gradingMode: event.target.value as GradingChoice })}>{(["ai_assisted", "auto_objective_only", "human_review_required"] as GradingChoice[]).map((mode) => <Radio key={mode} value={mode}><span><strong>{gradingPresentation[mode].title}{mode === "ai_assisted" ? <em>推荐</em> : null}</strong><small>{gradingPresentation[mode].description}</small></span></Radio>)}</Radio.Group>
      <label className="exam-create-field"><span>成绩发布</span><Select value={draft.publishPolicy} options={publishPolicyOptions} onChange={(publishPolicy) => onChange({ publishPolicy })} /></label><Checkbox checked={draft.appealEnabled} onChange={(event) => onChange({ appealEnabled: event.target.checked })}>成绩发布后允许申诉</Checkbox>
      <details className="exam-create-policy-details"><summary>高级阅卷方式</summary><Radio.Group value={draft.gradingMode} onChange={(event) => onChange({ gradingMode: event.target.value as GradingChoice })}><Radio value="double_mark">双评</Radio><Radio value="blind_double_mark">盲双评</Radio></Radio.Group></details>
    </section>
  </div>;
}
