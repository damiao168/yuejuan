import type { Grade, SchoolClass } from "../../../../api/org";
import { examSubjectLabel } from "../../../../constants/examStatus";
import { examTypeLabel, gradingLabel, publishPolicyLabel } from "../presentation";
import type { CreateExamDraft } from "../types";

export function ConfirmStep({ draft, grades, classes, studentCountByClass }: { draft: CreateExamDraft; grades: Grade[]; classes: SchoolClass[]; studentCountByClass: Map<string, number> }) {
  const grade = grades.find((item) => item.id === draft.gradeId);
  const selectedClasses = classes.filter((item) => draft.classIds.includes(item.id));
  const students = selectedClasses.reduce((total, item) => total + (studentCountByClass.get(item.id) ?? 0), 0);
  return (
    <div className="exam-confirmation">
      <section><h3>基本信息</h3><dl>
        <div><dt>考试名称</dt><dd>{draft.name}</dd></div>
        <div><dt>考试类型</dt><dd>{examTypeLabel(draft.examType)}</dd></div>
        <div><dt>考试科目</dt><dd>{examSubjectLabel(draft.subject)}</dd></div>
        <div><dt>满分</dt><dd>{draft.totalScore} 分</dd></div>
      </dl></section>
      <section><h3>学生范围</h3><dl>
        <div><dt>年级</dt><dd>{grade?.name ?? "-"}</dd></div>
        <div><dt>参考班级</dt><dd>{selectedClasses.map((item) => item.name).join("、") || "-"}</dd></div>
        {selectedClasses.length > 0 && selectedClasses.every((item) => studentCountByClass.has(item.id)) ? <div><dt>学生人数</dt><dd>{students} 人</dd></div> : null}
      </dl></section>
      <section><h3>阅卷设置</h3><dl>
        <div><dt>阅卷方式</dt><dd>{gradingLabel(draft.gradingMode)}</dd></div>
        <div><dt>成绩发布</dt><dd>{publishPolicyLabel(draft.publishPolicy)}</dd></div>
        <div><dt>申诉</dt><dd>{draft.appealEnabled ? "开启" : "关闭"}</dd></div>
      </dl></section>
      <div className="exam-confirm-next"><strong>创建后进入“考试准备”</strong><span>系统会展示学生、试卷、答案、答题卡模板等真实 readiness 检查项。</span></div>
    </div>
  );
}
