import { Button, Checkbox, Input, Select } from "antd";
import type { Grade, School, SchoolClass } from "../../../../api/org";
import { examTypeOptions } from "../../../../constants/examCatalog";
import { examSubjectOptions } from "../../../../constants/examStatus";
import { createSubjectDraft } from "../createExamDraft";
import type { CreateExamDraft } from "../types";

export function ExamScopeStep({ draft, schools, grades, classes, onChange }: { draft: CreateExamDraft; schools: School[]; grades: Grade[]; classes: SchoolClass[]; onChange: (patch: Partial<CreateExamDraft>) => void }) {
  const school = schools.find((item) => item.id === draft.schoolId);
  const availableClasses = classes.filter((item) => item.grade_id === draft.gradeId && item.status === "active");
  const selected = new Set(draft.classIds);
  const allSelected = availableClasses.length > 0 && availableClasses.every((item) => selected.has(item.id));
  const changeSubjects = (subjects: string[]) => onChange({ subjects: subjects.map((subject) => draft.subjects.find((item) => item.subject === subject) ?? createSubjectDraft(subject)) });
  return (
    <div className="exam-scope-form">
      <div className="exam-create-field-grid">
        <label className="exam-create-field"><span>学校 *</span>{schools.length > 1 ? <Select value={draft.schoolId || undefined} options={schools.map((item) => ({ label: item.name, value: item.id }))} onChange={(schoolId) => onChange({ schoolId, gradeId: "", classIds: [], subjects: [] })} /> : <Input value={school?.name ?? ""} readOnly />}</label>
        <label className="exam-create-field"><span>年级 *</span><Select value={draft.gradeId || undefined} placeholder="选择年级" options={grades.map((grade) => ({ label: `${grade.name} · ${grade.academic_year}`, value: grade.id }))} onChange={(gradeId) => onChange({ gradeId, classIds: [], subjects: draft.subjects.map((item) => ({ ...item, classIds: [] })) })} /></label>
      </div>
      <div className="exam-create-field-grid">
        <label className="exam-create-field"><span>考试名称 *</span><Input value={draft.name} maxLength={100} placeholder="例如：2026—2027学年高二第一学期期中考试" onChange={(event) => onChange({ name: event.target.value })} /></label>
        <label className="exam-create-field"><span>考试类型 *</span><Select value={draft.examType || undefined} placeholder="选择考试类型" options={examTypeOptions} onChange={(examType) => onChange({ examType })} /></label>
      </div>
      <label className="exam-create-field"><span>考试科目 * <small>可多选；系统将为每个科目创建独立阅卷工作区</small></span><Select mode="multiple" value={draft.subjects.map((item) => item.subject)} placeholder="选择本次考试包含的科目" options={examSubjectOptions} onChange={changeSubjects} /></label>
      <div className="exam-scope-toolbar"><span>统一参考班级 *</span><Button type="link" disabled={!availableClasses.length} onClick={() => onChange({ classIds: allSelected ? [] : availableClasses.map((item) => item.id) })}>{allSelected ? "取消全选" : "全选当前年级"}</Button></div>
      <div className="exam-class-list compact">
        {availableClasses.map((schoolClass) => <button type="button" key={schoolClass.id} className={selected.has(schoolClass.id) ? "selected" : ""} onClick={() => onChange({ classIds: selected.has(schoolClass.id) ? draft.classIds.filter((id) => id !== schoolClass.id) : [...draft.classIds, schoolClass.id] })}><Checkbox checked={selected.has(schoolClass.id)} tabIndex={-1} /><span><strong>{schoolClass.name}</strong><small>{schoolClass.code}</small></span></button>)}
        {!availableClasses.length ? <div className="exam-create-inline-empty">选择年级后，可在这里确定参考班级。</div> : null}
      </div>
      <div className="exam-scope-total">已选择 <strong>{draft.classIds.length}</strong> 个班级 · <strong>{draft.subjects.length}</strong> 个科目</div>
    </div>
  );
}
