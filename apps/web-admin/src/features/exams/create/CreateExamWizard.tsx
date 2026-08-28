import { Button, Steps } from "antd";
import { AnimatePresence, motion } from "framer-motion";
import { ArrowLeft, ArrowRight, Check, Cloud } from "lucide-react";
import type { Grade, School, SchoolClass } from "../../../api/org";
import type { ExamTemplate } from "../../../api/examTemplates";
import { ExamScopeStep } from "./steps/ExamScopeStep";
import { ExamTemplateStep } from "./steps/ExamTemplateStep";
import { PaperStructureStep } from "./steps/PaperStructureStep";
import { ReviewCreateStep } from "./steps/ReviewCreateStep";
import type { CreateExamDraft } from "./types";

const steps = [
  { title: "考试范围", description: "确定考试名称、年级和参考班级" },
  { title: "考试方案", description: "从学校或系统方案开始，也可以完全自定义" },
  { title: "试卷结构", description: "按本场考试调整科目、分值和题目分区" },
  { title: "检查并创建", description: "完成分值校验，确认阅卷与发布策略" }
];

export function CreateExamWizard({ step, draft, schools, grades, classes, templates, scopeLocked, savedAt, submitting, onChange, onStepChange, onSubmit, onCancel }: {
  step: number;
  draft: CreateExamDraft;
  schools: School[];
  grades: Grade[];
  classes: SchoolClass[];
  templates: ExamTemplate[];
  scopeLocked: boolean;
  savedAt: string;
  submitting: boolean;
  onChange: (patch: Partial<CreateExamDraft>) => void;
  onStepChange: (step: number) => void;
  onSubmit: () => void;
  onCancel: () => void;
}) {
  const current = steps[step];
  return (
    <div className="exam-create-wizard">
      <button type="button" className="exam-create-back" onClick={onCancel}><ArrowLeft size={16} /> 返回考试列表</button>
      <header className="exam-create-page-header"><div><h1>新建考试</h1><span className="exam-draft-status"><Cloud size={14} /> {savedAt ? `草稿已保存 ${savedAt}` : "草稿将自动保存"}</span></div><Steps current={step} responsive={false} items={steps.map(({ title }) => ({ title }))} /></header>
      <section className="exam-create-stage">
        <header><h2>{current.title}</h2><p>{current.description}</p></header>
        <AnimatePresence mode="wait" initial={false}>
          <motion.div key={step} className="exam-create-step-content" initial={{ opacity: 0, x: 10 }} animate={{ opacity: 1, x: 0 }} exit={{ opacity: 0, x: -8 }} transition={{ duration: .16 }}>
            {step === 0 ? <ExamScopeStep draft={draft} schools={schools} grades={grades} classes={classes} scopeLocked={scopeLocked} onChange={onChange} /> : null}
            {step === 1 ? <ExamTemplateStep draft={draft} grade={grades.find((item) => item.id === draft.gradeId)} templates={templates} onChange={onChange} /> : null}
            {step === 2 ? <PaperStructureStep draft={draft} classes={classes} onChange={onChange} /> : null}
            {step === 3 ? <ReviewCreateStep draft={draft} grades={grades} classes={classes} onChange={onChange} /> : null}
          </motion.div>
        </AnimatePresence>
        <footer>
          <div><Button disabled={submitting} onClick={onCancel}>取消</Button>{step > 0 ? <Button disabled={submitting} onClick={() => onStepChange(step - 1)}>上一步</Button> : null}</div>
          {step < steps.length - 1 ? <Button type="primary" onClick={() => onStepChange(step + 1)}>下一步 <ArrowRight size={15} /></Button> : <Button type="primary" loading={submitting} icon={<Check size={15} />} onClick={onSubmit}>创建 {draft.subjects.length} 个科目工作区</Button>}
        </footer>
      </section>
    </div>
  );
}
