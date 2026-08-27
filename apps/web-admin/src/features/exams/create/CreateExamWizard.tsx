import { Button, Steps } from "antd";
import { AnimatePresence, motion } from "framer-motion";
import { ArrowLeft, ArrowRight } from "lucide-react";
import type { Grade, School, SchoolClass } from "../../../api/org";
import { BasicInfoStep } from "./steps/BasicInfoStep";
import { ConfirmStep } from "./steps/ConfirmStep";
import { GradingPolicyStep } from "./steps/GradingPolicyStep";
import { StudentScopeStep } from "./steps/StudentScopeStep";
import type { CreateExamDraft } from "./types";

const steps = [
  { title: "基本信息", description: "考试名称、类型、科目和满分" },
  { title: "学生范围", description: "选择年级和参考班级" },
  { title: "阅卷设置", description: "阅卷、发布和申诉策略" },
  { title: "确认创建", description: "核对考试信息" }
];

export function CreateExamWizard({ step, draft, schools, grades, classes, studentCountByClass, submitting, onChange, onStepChange, onSubmit, onCancel }: {
  step: number;
  draft: CreateExamDraft;
  schools: School[];
  grades: Grade[];
  classes: SchoolClass[];
  studentCountByClass: Map<string, number>;
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
      <header className="exam-create-page-header"><h1>新建考试</h1><Steps current={step} responsive={false} items={steps.map(({ title }) => ({ title }))} /></header>
      <section className="exam-create-stage">
        <header><h2>{current.title}</h2><p>{current.description}</p></header>
        <AnimatePresence mode="wait" initial={false}>
          <motion.div key={step} className="exam-create-step-content" initial={{ opacity: 0, x: 10 }} animate={{ opacity: 1, x: 0 }} exit={{ opacity: 0, x: -8 }} transition={{ duration: .16 }}>
            {step === 0 ? <BasicInfoStep draft={draft} schools={schools} grades={grades} onChange={onChange} /> : null}
            {step === 1 ? <StudentScopeStep draft={draft} grades={grades} classes={classes} studentCountByClass={studentCountByClass} onChange={onChange} /> : null}
            {step === 2 ? <GradingPolicyStep draft={draft} onChange={onChange} /> : null}
            {step === 3 ? <ConfirmStep draft={draft} grades={grades} classes={classes} studentCountByClass={studentCountByClass} /> : null}
          </motion.div>
        </AnimatePresence>
        <footer>
          <div><Button disabled={submitting} onClick={onCancel}>取消</Button>{step > 0 ? <Button disabled={submitting} onClick={() => onStepChange(step - 1)}>上一步</Button> : null}</div>
          {step < steps.length - 1 ? <Button type="primary" onClick={() => onStepChange(step + 1)}>下一步 <ArrowRight size={15} /></Button> : <Button type="primary" loading={submitting} onClick={onSubmit}>创建考试 <ArrowRight size={15} /></Button>}
        </footer>
      </section>
    </div>
  );
}
