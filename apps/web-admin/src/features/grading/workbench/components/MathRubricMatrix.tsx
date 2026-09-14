import { Alert, Button, Empty, Spin } from "antd";
import type { RubricPoint } from "../../../../api/papers";
import type { AiGrade } from "../../../../api/review";
import { StatusTag } from "../../../../components/StatusTag";
import { mathDecisionSteps, mathSuggestionState, type MathWorkbenchEvidence } from "../mathWorkbenchEvidence";

export interface MathRubricMatrixProps {
  evidence: MathWorkbenchEvidence;
  grades: AiGrade[];
  points: RubricPoint[];
  dirty: boolean;
  requesting: boolean;
  canRequest: boolean;
  onRequest: () => void;
  onSelectStep: (stepId: string) => void;
}

export function MathRubricMatrix({ evidence, grades, points, dirty, requesting, canRequest, onRequest, onSelectStep }: MathRubricMatrixProps) {
  const { score, understanding } = evidence;
  return <section className="math-rubric-matrix" aria-label="数学评分点与步骤证据">
    <div className="math-evidence-head"><div><h3>评分点 × 步骤</h3><p>分值由服务端计算，待确认项不会记为零分。</p></div></div>
    {evidence.phase === "loading" ? <Spin size="small" aria-label="正在核对数学证据" /> : null}
    {evidence.message ? <Alert type={evidence.phase === "ready" ? "info" : "warning"} showIcon message={evidence.message} /> : null}
    {dirty ? <Alert type="warning" showIcon message="校正未保存，建议暂不可采纳。" /> : null}
    {score && understanding ? <>
      <div className="math-score-summary" aria-label="服务端数学评分区间">
        <div><span>已验证分值</span><strong>{score.verified_score}</strong></div>
        <div><span>待确认分值</span><strong>{score.unresolved_score}</strong></div>
        <div><span>{score.suggested_score === null ? "建议区间" : "服务端建议"}</span><strong>{score.suggested_score === null ? `${score.score_range.min}–${score.score_range.max}` : score.suggested_score}<small> / {score.max_score}</small></strong></div>
      </div>
      <div className="math-matrix-scroll"><table className="math-decision-table">
        <caption>当前数学证据 v{score.artifact_version} · 校正谱系 #{score.verified_correction_revision + score.correction_revision} · {evidence.phase === "pending" ? "待验证" : "教师复核"}</caption>
        <thead><tr><th scope="col">评分点</th><th scope="col">分值</th><th scope="col">步骤 / 验证</th></tr></thead>
        <tbody>{score.criterion_decisions.map((decision) => {
          const steps = mathDecisionSteps(decision, score, understanding);
          const checks = (understanding.effective_artifact?.verifications ?? understanding.artifact.verifications)?.filter((item) => decision.verification_ids.includes(item.id)) ?? [];
          const uncertain = decision.awarded_score === null || decision.status === "uncertain";
          return <tr key={decision.rubric_point_id}>
            <th scope="row"><span>{points.find((point) => point.id === decision.rubric_point_id)?.description || decision.rubric_point_id}</span><StatusTag tone={uncertain ? "warning" : decision.status === "supported" ? "success" : "danger"}>{uncertain ? "待确认" : decision.status === "supported" ? "已验证" : "已验证不成立"}</StatusTag></th>
            <td>{uncertain ? "待确认" : decision.awarded_score} / {decision.max_score}</td>
            <td><div className="math-matrix-steps">{steps.length ? steps.map((step) => <Button key={step.id} type="link" size="small" disabled={dirty || evidence.phase === "conflict"} onClick={() => onSelectStep(step.id)} aria-label={`定位数学步骤 ${step.id}`}>{step.id}</Button>) : <span className="muted">暂无已绑定步骤</span>}</div>
              <small title={decision.reason_code}>{checks.length ? checks.map((check) => `${check.engine === "sympy" ? "SymPy" : "规则"} · ${check.status === "verified" ? "已验证" : check.status === "contradicted" ? "不成立" : "待确认"}`).join("；") : decision.decision_source === "model_candidate" ? "语义候选 · 人工确认" : "规则 / 语义待确认"}</small>
            </td>
          </tr>;
        })}</tbody>
      </table></div>
    </> : evidence.phase !== "loading" && !evidence.message ? <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无可用的服务端数学判定" /> : null}
    <Button size="small" loading={requesting} disabled={!canRequest || dirty || evidence.phase !== "ready"} onClick={onRequest}>重新生成数学建议</Button>
    <p className="math-suggestion-note">仅在点击后请求受治理模型；未完成验证或未开放服务时，请继续人工阅卷。</p>
    {grades.length ? <details className="math-suggestion-history"><summary>建议版本记录（{grades.length}）</summary><ul>{grades.map((grade) => {
      const state = mathSuggestionState(grade, evidence, dirty);
      return <li key={grade.id}><div><strong>{grade.math_artifact_version ? `数学证据 v${grade.math_artifact_version} · 校正 #${grade.math_correction_revision ?? 0}` : "旧版 / 未绑定数学证据"}</strong><small>{grade.suggested_score} / {grade.max_score} · {new Date(grade.created_at).toLocaleString("zh-CN")}</small></div><StatusTag tone={state.current ? "success" : "warning"}>{state.label}</StatusTag></li>;
    })}</ul></details> : null}
  </section>;
}
