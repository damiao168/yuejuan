import { Checkbox, Radio, Select } from "antd";
import { gradingPresentation, publishPolicyOptions } from "../presentation";
import type { CreateExamDraft, GradingChoice } from "../types";

export function GradingPolicyStep({ draft, onChange }: { draft: CreateExamDraft; onChange: (patch: Partial<CreateExamDraft>) => void }) {
  return (
    <div className="exam-create-fields">
      <Radio.Group className="exam-choice-list grading-policy-list" value={draft.gradingMode} onChange={(event) => onChange({ gradingMode: event.target.value as GradingChoice })}>
        <Radio value="ai_assisted"><span><strong>{gradingPresentation.ai_assisted.title} <em>推荐</em></strong><small>{gradingPresentation.ai_assisted.description}</small></span></Radio>
        <Radio value="auto_objective_only"><span><strong>{gradingPresentation.auto_objective_only.title}</strong><small>{gradingPresentation.auto_objective_only.description}</small></span></Radio>
        <Radio value="human_review_required"><span><strong>{gradingPresentation.human_review_required.title}</strong><small>{gradingPresentation.human_review_required.description}</small></span></Radio>
      </Radio.Group>
      <div className="exam-create-subsection"><h3>成绩发布</h3><label className="exam-create-field"><span>发布方式</span><Select value={draft.publishPolicy} options={publishPolicyOptions} onChange={(publishPolicy) => onChange({ publishPolicy })} /></label><Checkbox checked={draft.appealEnabled} onChange={(event) => onChange({ appealEnabled: event.target.checked })}>成绩发布后允许申诉</Checkbox></div>
      <details className="exam-create-policy-details"><summary>高级阅卷设置</summary><Radio.Group className="exam-choice-list" value={["double_mark", "blind_double_mark"].includes(draft.gradingMode) ? draft.gradingMode : undefined} onChange={(event) => onChange({ gradingMode: event.target.value as GradingChoice })}><Radio value="double_mark"><span><strong>{gradingPresentation.double_mark.title}</strong><small>{gradingPresentation.double_mark.description}</small></span></Radio><Radio value="blind_double_mark"><span><strong>{gradingPresentation.blind_double_mark.title}</strong><small>{gradingPresentation.blind_double_mark.description}</small></span></Radio></Radio.Group><p>具体分配规则在考试准备阶段设置。</p></details>
    </div>
  );
}
