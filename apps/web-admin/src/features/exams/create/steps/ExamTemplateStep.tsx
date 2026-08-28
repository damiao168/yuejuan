import { Select } from "antd";
import { Check, FileSliders } from "lucide-react";
import type { ExamTemplate } from "../../../../api/examTemplates";
import type { Grade } from "../../../../api/org";
import { examSubjectOptions, examSubjectLabel } from "../../../../constants/examStatus";
import { createSubjectDraft, subjectDraftFromTemplate } from "../createExamDraft";
import type { CreateExamDraft } from "../types";

export function ExamTemplateStep({ draft, grade, templates, onChange }: { draft: CreateExamDraft; grade?: Grade; templates: ExamTemplate[]; onChange: (patch: Partial<CreateExamDraft>) => void }) {
  const visibleTemplates = templates.filter((template) => !grade || template.education_stage === grade.education_stage);
  const selectTemplate = (template: ExamTemplate) => onChange({ templateId: template.id, subjects: template.subjects.map(subjectDraftFromTemplate) });
  const selectCustom = () => onChange({ templateId: "custom", subjects: draft.templateId && draft.templateId !== "custom" ? [] : draft.subjects });
  const changeCustomSubjects = (subjects: string[]) => onChange({ subjects: subjects.map((subject) => draft.subjects.find((item) => item.subject === subject) ?? createSubjectDraft(subject)) });

  return <div className="exam-template-step">
    <div className="exam-template-options">
      {visibleTemplates.map((template) => <button type="button" key={template.id} className={`exam-template-option ${draft.templateId === template.id ? "selected" : ""}`} onClick={() => selectTemplate(template)}>
        <span className="exam-template-icon"><FileSliders size={18} /></span>
        <span className="exam-template-copy"><strong>{template.name}{template.recommended ? <em>推荐</em> : null}</strong><small>{template.description}</small><span>{template.subjects.map((subject) => examSubjectLabel(subject.subject)).join(" / ")} · 版本 {template.version}</span></span>
        {draft.templateId === template.id ? <Check size={18} /> : null}
      </button>)}
      <button type="button" className={`exam-template-option ${draft.templateId === "custom" ? "selected" : ""}`} onClick={selectCustom}>
        <span className="exam-template-icon"><FileSliders size={18} /></span><span className="exam-template-copy"><strong>完全自定义</strong><small>从空白科目结构开始，适合特殊考试或临时测验。</small><span>不套用系统题型规则</span></span>{draft.templateId === "custom" ? <Check size={18} /> : null}
      </button>
    </div>
    {draft.templateId === "custom" ? <label className="exam-create-field"><span>考试科目 *</span><Select mode="multiple" value={draft.subjects.map((item) => item.subject)} placeholder="选择本次考试包含的科目" options={examSubjectOptions} onChange={changeCustomSubjects} /></label> : null}
  </div>;
}
