import { Alert, Button, Input, InputNumber, Select } from "antd";
import { Plus, Trash2 } from "lucide-react";
import type { SchoolClass } from "../../../../api/org";
import { questionTypeOptions } from "../../../../constants/examCatalog";
import { examSubjectLabel } from "../../../../constants/examStatus";
import { createBlankSection, subjectScore } from "../createExamDraft";
import type { CreateExamDraft, SubjectExamDraft } from "../types";

export function PaperStructureStep({ draft, classes, onChange }: { draft: CreateExamDraft; classes: SchoolClass[]; onChange: (patch: Partial<CreateExamDraft>) => void }) {
  const updateSubject = (subjectCode: string, patch: Partial<SubjectExamDraft>) => onChange({ subjects: draft.subjects.map((item) => item.subject === subjectCode ? { ...item, ...patch } : item) });
  const selectedClasses = classes.filter((item) => draft.classIds.includes(item.id));
  return <div className="paper-blueprint-list">{draft.subjects.map((subject) => {
    const configuredScore = subjectScore(subject);
    const mismatch = Math.abs(configuredScore - subject.totalScore) > .001;
    return <section className="paper-blueprint" key={subject.subject}>
      <header><div><span className="paper-blueprint-index">{examSubjectLabel(subject.subject)}</span><h3>试卷结构</h3></div><span className={mismatch ? "score-mismatch" : "score-valid"}>{configuredScore} / {subject.totalScore} 分</span></header>
      <div className="paper-blueprint-meta">
        <label className="exam-create-field"><span>科目满分</span><InputNumber min={1} max={1000} precision={1} value={subject.totalScore} onChange={(value) => updateSubject(subject.subject, { totalScore: value ?? 0 })} /></label>
        <label className="exam-create-field"><span>考试时长（分钟）</span><InputNumber min={10} max={600} value={subject.durationMinutes} onChange={(value) => updateSubject(subject.subject, { durationMinutes: value ?? 0 })} /></label>
        <label className="exam-create-field"><span>参考规则</span><Select value={subject.candidateRule} options={[{ label: "沿用统一班级范围", value: "all_selected_classes" }, { label: "本学科单独指定班级", value: "subject_selected_classes" }]} onChange={(candidateRule) => updateSubject(subject.subject, { candidateRule, classIds: candidateRule === "all_selected_classes" ? [] : subject.classIds })} /></label>
      </div>
      {subject.candidateRule === "subject_selected_classes" ? <label className="exam-create-field"><span>本学科参考班级 *</span><Select mode="multiple" value={subject.classIds} options={selectedClasses.map((item) => ({ label: item.name, value: item.id }))} onChange={(classIds) => updateSubject(subject.subject, { classIds })} /></label> : null}
      <div className="paper-section-head"><span>分区</span><span>题型</span><span>题数</span><span>每题分值</span><span>小计</span><span /></div>
      {subject.sections.map((section) => <div className="paper-section-row" key={section.id}>
        <Input value={section.title} onChange={(event) => updateSubject(subject.subject, { sections: subject.sections.map((item) => item.id === section.id ? { ...item, title: event.target.value } : item) })} />
        <Select value={section.questionType} options={questionTypeOptions} onChange={(questionType) => updateSubject(subject.subject, { sections: subject.sections.map((item) => item.id === section.id ? { ...item, questionType } : item) })} />
        <InputNumber min={1} max={200} value={section.questionCount} onChange={(questionCount) => updateSubject(subject.subject, { sections: subject.sections.map((item) => item.id === section.id ? { ...item, questionCount: questionCount ?? 1 } : item) })} />
        <InputNumber min={.5} max={200} precision={1} value={section.scorePerQuestion} onChange={(scorePerQuestion) => updateSubject(subject.subject, { sections: subject.sections.map((item) => item.id === section.id ? { ...item, scorePerQuestion: scorePerQuestion ?? 1 } : item) })} />
        <strong>{section.questionCount * section.scorePerQuestion}</strong>
        <Button type="text" danger aria-label={`删除${section.title}`} disabled={subject.sections.length === 1} icon={<Trash2 size={15} />} onClick={() => updateSubject(subject.subject, { sections: subject.sections.filter((item) => item.id !== section.id) })} />
      </div>)}
      <Button type="dashed" icon={<Plus size={15} />} onClick={() => updateSubject(subject.subject, { sections: [...subject.sections, createBlankSection()] })}>添加分区</Button>
      {mismatch ? <Alert type="warning" showIcon message={`当前题目合计 ${configuredScore} 分，与科目满分相差 ${Math.abs(subject.totalScore - configuredScore)} 分`} /> : null}
    </section>;
  })}</div>;
}
