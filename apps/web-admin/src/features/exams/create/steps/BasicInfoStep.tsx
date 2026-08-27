import { Input, InputNumber, Select } from "antd";
import type { Grade, School } from "../../../../api/org";
import { examSubjectOptions } from "../../../../constants/examStatus";
import { examTypeOptions } from "../presentation";
import type { CreateExamDraft } from "../types";

export function BasicInfoStep({ draft, schools, onChange }: { draft: CreateExamDraft; schools: School[]; grades: Grade[]; onChange: (patch: Partial<CreateExamDraft>) => void }) {
  const school = schools.find((item) => item.id === draft.schoolId);
  return (
    <div className="exam-create-fields">
      <label className="exam-create-field"><span>考试名称 *</span><Input value={draft.name} maxLength={100} placeholder="例如：2026-2027学年第一学期高二数学期中考试" onChange={(event) => onChange({ name: event.target.value })} /></label>
      <div className="exam-create-field-grid">
        <label className="exam-create-field"><span>考试类型 *</span><Select value={draft.examType} options={examTypeOptions} onChange={(examType) => onChange({ examType })} /></label>
        <label className="exam-create-field"><span>考试科目 *</span><Select value={draft.subject} options={examSubjectOptions} onChange={(subject) => onChange({ subject })} /></label>
      </div>
      <div className="exam-create-field-grid">
        <label className="exam-create-field"><span>满分 *</span><InputNumber min={1} max={1000} precision={1} value={draft.totalScore} onChange={(value) => onChange({ totalScore: value ?? 100 })} /></label>
        <label className="exam-create-field"><span>学校</span>{schools.length > 1 ? <Select value={draft.schoolId || undefined} placeholder="选择学校" options={schools.map((item) => ({ label: item.name, value: item.id }))} onChange={(schoolId) => onChange({ schoolId, gradeId: "", classIds: [] })} /> : <Input value={school?.name ?? ""} readOnly suffix="自动确定" />}</label>
      </div>
    </div>
  );
}
