import type { Dispatch, SetStateAction } from "react";
import { Button, Checkbox, Input, Popconfirm, Space, Tooltip } from "antd";
import { Award, Check, CheckCircle2, Flag } from "lucide-react";
import type { RubricPoint } from "../../../../api/papers";
import type { AiGrade } from "../../../../api/review";
import { StatusTag } from "../../../../components/StatusTag";
import { SharedScoreControl } from "../../SharedScoreControl";
import { requiresExplicitSecondOpinion } from "../reviewContext";
import { commentPresets } from "../gradingWorkbench.model";
import type { ScoreDraft, WorkbenchContext } from "../gradingWorkbench.types";

export interface ScoreEditorProps {
  context: WorkbenchContext;
  draft: ScoreDraft;
  setDraft: Dispatch<SetStateAction<ScoreDraft>>;
  maxScore: number;
  rubricPoints: RubricPoint[];
  selectedGrade?: AiGrade;
  canEditDraft: boolean;
  canSubmit: boolean;
  canReturn: boolean;
  canManageTasks: boolean;
  actioning: string | null;
  onAdoptAiScore: () => void;
  onMarkDispute: () => Promise<void>;
  onSubmit: (nominateAsGold?: boolean) => Promise<void>;
  quickSubmit?: boolean;
  onQuickSubmitChange?: (value: boolean) => void;
}

export function ScoreEditor({
  context,
  draft,
  setDraft,
  maxScore,
  rubricPoints,
  selectedGrade,
  canEditDraft,
  canSubmit,
  canReturn,
  canManageTasks,
  actioning,
  onAdoptAiScore,
  onMarkDispute,
  onSubmit,
  quickSubmit = false,
  onQuickSubmitChange
}: ScoreEditorProps) {
  return (
    <section className="score-panel">
      <div className="panel-head">
        <div>
          <h2>最终评分</h2>
          <p>满分 {maxScore || "—"}</p>
        </div>
        {selectedGrade && !requiresExplicitSecondOpinion(context.reviewContext) ? (
          <Button icon={<Check size={14} />} disabled={!canEditDraft} onClick={onAdoptAiScore}>采纳 AI 建议</Button>
        ) : null}
      </div>

      <SharedScoreControl
        disabled={!canEditDraft}
        maxScore={maxScore}
        rubricPoints={rubricPoints}
        value={{ score: draft.score, rubricSelections: draft.rubricSelections }}
        onChange={(value) => setDraft((current) => ({ ...current, ...value }))}
      />

      {rubricPoints.length > 0 && draft.score !== null && Math.abs(draft.score - rubricPoints.reduce((sum, point) => sum + (draft.rubricSelections[point.id] ?? 0), 0)) > 0.001 ? (
        <div className="grading-field">
          <label htmlFor="score-adjustment-reason">人工调整原因（必填）</label>
          <Input.TextArea id="score-adjustment-reason" disabled={!canEditDraft} value={draft.reason} rows={2} placeholder="说明最终分与评分点合计不同的依据" onChange={(event) => setDraft((current) => ({ ...current, reason: event.target.value }))} />
        </div>
      ) : null}

      <details className="grading-more-fields">
        <summary>评语与备注（可选）</summary>
        <div className="grading-more-fields-body">
          <div className="grading-comment-presets">
            <span className="muted">常用评语（点击追加）</span>
            <Space wrap size={4}>
              {commentPresets.map((item) => (
                <Button
                  key={item}
                  size="small"
                  disabled={!canEditDraft}
                  onClick={() => setDraft((current) => ({ ...current, comments: current.comments ? `${current.comments}；${item}` : item }))}
                >
                  {item}
                </Button>
              ))}
            </Space>
          </div>
          <div className="grading-field">
            <label className="grading-field-label muted" htmlFor="grading-comments-input">教师评语（阅卷记录）</label>
            <Input.TextArea id="grading-comments-input" disabled={!canEditDraft} rows={2} placeholder="记录评分依据（可选）" value={draft.comments} onChange={(event) => setDraft((current) => ({ ...current, comments: event.target.value }))} />
          </div>
          <div className="grading-field">
            <div className="grading-field-label">
              <label className="muted" htmlFor="grading-student-feedback-input">学生可见反馈（将展示给学生）</label>
              <StatusTag tone="warning">学生可见</StatusTag>
            </div>
            <Input.TextArea id="grading-student-feedback-input" disabled={!canEditDraft} rows={2} placeholder="这段文字会展示给学生（可选）" value={draft.studentFeedback} onChange={(event) => setDraft((current) => ({ ...current, studentFeedback: event.target.value }))} />
          </div>
          <div className="grading-field">
            <label className="grading-field-label muted" htmlFor="grading-private-note-input">内部备注（阅卷教师及授权管理员可见）</label>
            <Input.TextArea id="grading-private-note-input" disabled={!canEditDraft} rows={2} placeholder="仅教师与管理端可见（可选）" value={draft.privateNote} onChange={(event) => setDraft((current) => ({ ...current, privateNote: event.target.value }))} />
          </div>
          {canReturn ? <Input disabled={!canEditDraft} placeholder="争议原因（可选，退回重评时会一并记录）" prefix={<Flag size={14} />} value={draft.disputeReason} onChange={(event) => setDraft((current) => ({ ...current, disputeReason: event.target.value }))} /> : null}
        </div>
      </details>

      <div className="grading-submit-note">提交后进入质检，不会直接发布成绩</div>
      <Checkbox checked={quickSubmit} onChange={(event) => onQuickSubmitChange?.(event.target.checked)}>连续阅卷：允许 Enter 提交并下一份</Checkbox>
      <details className="grading-shortcut-help"><summary>键盘帮助 · {quickSubmit ? "Enter" : "Ctrl / ⌘ + Enter"} 提交</summary><p>{rubricPoints.length ? "1–9 仅切换对应评分点并合计；没有对应评分点的数字键不生效。" : "1–9 直接打分，0分或小数请在最终分输入框填写。"} A 采纳建议 · F 标记异常 · R 退回 · Z 撤销本次评分编辑 · +/- 缩放。输入文字或打开弹窗时快捷键暂停。</p></details>

      <Space wrap className="grading-submit-bar">
        {canReturn ? (
          <Popconfirm
            title="标记争议并退回重评？"
            description="该答卷将退出你的队列，进入重评流程"
            okText="确认退回"
            cancelText="再想想"
            okButtonProps={{ danger: true }}
            onConfirm={() => void onMarkDispute()}
          >
            <Button danger icon={<Flag size={16} />} disabled={!context} loading={actioning === "return"}>标记争议</Button>
          </Popconfirm>
        ) : null}
        {canManageTasks ? (
          <Button icon={<Award size={16} />} disabled={!canSubmit || context.task.status === "submitted"} loading={actioning === "submit"} onClick={() => void onSubmit(true)}>
            提交并提名标准卷
          </Button>
        ) : null}
        <Tooltip title={quickSubmit ? "Enter（连续阅卷）" : "Ctrl / ⌘ + Enter"}>
          <Button type="primary" icon={<CheckCircle2 size={16} />} disabled={!canSubmit || context.task.status === "submitted"} loading={actioning === "submit"} onClick={() => void onSubmit()}>
            提交并下一份
          </Button>
        </Tooltip>
      </Space>
    </section>
  );
}
