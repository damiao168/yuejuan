import { useCallback, useEffect, useMemo, useState } from "react";
import {
  Alert,
  App,
  Button,
  Descriptions,
  Divider,
  Drawer,
  Empty,
  Input,
  InputNumber,
  List,
  Modal,
  Popconfirm,
  Progress,
  Select,
  Space,
  Spin,
  Tag,
  Tooltip
} from "antd";
import type { AnswerGroup, AnswerGroupMetrics, TeacherReferenceCase } from "@edugrade/sdk";
import { ApiClientError, getUserErrorMessage } from "../../api/client";
import { listQuestions, type Question } from "../../api/papers";
import {
  buildAnswerGroups,
  confirmAnswerGroup,
  getAnswerGroupMetrics,
  listAnswerGroups,
  reviewAnswerGroupSample,
  rollbackAnswerGroup,
  saveAnswerGroupDecision
} from "../../api/answerGroups";
import { confirmBlockReason, sampleRoleLabels } from "./answerGroupingPresentation";

function errorMessage(error: unknown) {
  if (error instanceof ApiClientError) {
    if (error.code === "answer_group_sampling_incomplete") return "抽检未满足策略：请完成最低抽检数量并检查全部异常样本。";
    if (error.code === "answer_group_revision_conflict") return "候选已被其他人更新，请刷新后再操作。";
    if (error.code === "answer_group_no_eligible_answers") return "该题没有可可靠文本化的短答或精确文本答案，不能分组。";
    return getUserErrorMessage(error, "答案分组操作失败");
  }
  return getUserErrorMessage(error, "答案分组操作失败");
}

const statusLabels: Record<string, string> = {
  sampling: "抽检中",
  ready_for_confirmation: "可确认",
  confirmed: "已生成候选",
  rolled_back: "已回滚"
};

export function AnswerGroupingDrawer({ open, examId, initialQuestionId = "", canManage, onClose }: {
  open: boolean;
  examId: string;
  initialQuestionId?: string;
  canManage: boolean;
  onClose: () => void;
}) {
  const { message } = App.useApp();
  const [questions, setQuestions] = useState<Question[]>([]);
  const [questionId, setQuestionId] = useState(initialQuestionId);
  const [groups, setGroups] = useState<AnswerGroup[]>([]);
  const [references, setReferences] = useState<TeacherReferenceCase[]>([]);
  const [metrics, setMetrics] = useState<AnswerGroupMetrics>();
  const [selectedGroupId, setSelectedGroupId] = useState("");
  const [loading, setLoading] = useState(false);
  const [acting, setActing] = useState(false);
  const [loadError, setLoadError] = useState("");
  const [score, setScore] = useState<number | null>(null);
  const [rubricPoints, setRubricPoints] = useState<string[]>([]);
  const [rollbackOpen, setRollbackOpen] = useState(false);
  const [rollbackReason, setRollbackReason] = useState("");

  const selectedGroup = useMemo(() => groups.find((group) => group.id === selectedGroupId) ?? groups[0], [groups, selectedGroupId]);
  const blockReason = confirmBlockReason(selectedGroup);

  const load = useCallback(async (nextQuestionId = questionId) => {
    if (!examId || !nextQuestionId) {
      setGroups([]); setReferences([]); setMetrics(undefined); setLoadError("");
      return;
    }
    setLoading(true);
    setLoadError("");
    try {
      const [groupResponse, metricResponse] = await Promise.all([
        listAnswerGroups(examId, nextQuestionId),
        getAnswerGroupMetrics(examId, nextQuestionId)
      ]);
      setGroups(groupResponse.answer_groups);
      setReferences(groupResponse.teacher_reference_cases);
      setMetrics(metricResponse.metrics);
      setSelectedGroupId((current) => groupResponse.answer_groups.some((group) => group.id === current) ? current : groupResponse.answer_groups[0]?.id ?? "");
    } catch (error) {
      const text = errorMessage(error);
      setLoadError(text);
      setGroups([]); setReferences([]); setMetrics(undefined);
    } finally {
      setLoading(false);
    }
  }, [examId, questionId]);

  useEffect(() => {
    if (!open || !examId) return;
    let active = true;
    setQuestionId(initialQuestionId);
    listQuestions(examId)
      .then((response) => {
        if (!active) return;
        setQuestions(response.questions);
        const next = initialQuestionId || response.questions[0]?.id || "";
        setQuestionId(next);
        void load(next);
      })
      .catch((error) => active && setLoadError(errorMessage(error)));
    return () => { active = false; };
  }, [examId, initialQuestionId, open]);

  useEffect(() => {
    const decisionScore = selectedGroup?.decision?.score_candidate.score;
    setScore(typeof decisionScore === "number" ? decisionScore : null);
    const selection = selectedGroup?.decision?.rubric_selection.rubric_points;
    setRubricPoints(Array.isArray(selection) ? selection.filter((item): item is string => typeof item === "string") : []);
  }, [selectedGroup?.id, selectedGroup?.decision?.revision]);

  const run = async (action: () => Promise<unknown>, success: string) => {
    setActing(true);
    try {
      await action();
      message.success(success);
      await load();
    } catch (error) {
      message.error(errorMessage(error));
    } finally {
      setActing(false);
    }
  };

  const build = () => run(() => buildAnswerGroups(examId, questionId), "答案分组候选已生成");
  const reviewSample = (segmentId: string, outcome: "accepted" | "rejected") => run(
    () => reviewAnswerGroupSample(selectedGroup!.id, segmentId, outcome),
    outcome === "accepted" ? "样本已接受" : "样本已拒绝并保留人工处理"
  );
  const saveDecision = () => {
    if (!selectedGroup || score === null || rubricPoints.length === 0) {
      message.warning("请填写候选分数并至少选择一个 Rubric 采分点");
      return;
    }
    void run(() => saveAnswerGroupDecision(selectedGroup.id, {
      score_candidate: { score },
      rubric_selection: { rubric_points: rubricPoints },
      expected_revision: selectedGroup.decision?.revision ?? 0
    }), "组评分候选已保存");
  };
  const confirm = () => selectedGroup?.decision && run(
    () => confirmAnswerGroup(selectedGroup.id, selectedGroup.decision!.revision),
    "已为组成员生成可追溯评分候选；最终成绩未被修改"
  );
  const rollback = () => {
    const rollbackReference = selectedGroup?.decision?.rollback_reference;
    if (!selectedGroup || !rollbackReference || !rollbackReason.trim()) {
      message.warning("请填写回滚原因");
      return;
    }
    void run(() => rollbackAnswerGroup(selectedGroup.id, rollbackReference, rollbackReason.trim()), "组候选已回滚")
      .then(() => { setRollbackOpen(false); setRollbackReason(""); });
  };

  return (
    <>
      <Drawer title="相似答案分组" width={1040} open={open} onClose={onClose} extra={<Button loading={loading} onClick={() => void load()}>刷新</Button>}>
        <Alert
          type="info"
          showIcon
          message="分组只生成评分候选，不会直接修改最终成绩"
          description="仅对可可靠文本化的精确文本与短构造题开放；代表、边界与异常样本必须按抽检策略由教师确认。"
          style={{ marginBottom: 16 }}
        />
        <Space wrap style={{ marginBottom: 16 }}>
          <Select
            style={{ minWidth: 300 }}
            value={questionId || undefined}
            placeholder="选择题目"
            options={questions.map((question) => ({ value: question.id, label: `${question.question_no} · ${question.question_type}` }))}
            onChange={(value) => { setQuestionId(value); void load(value); }}
          />
          {canManage ? <Button type="primary" disabled={!questionId} loading={acting} onClick={() => void build()}>构建分组</Button> : null}
        </Space>
        {loadError ? <Alert type="error" showIcon message="答案分组加载失败" description={loadError} style={{ marginBottom: 16 }} /> : null}
        {metrics ? (
          <Descriptions size="small" bordered column={5} style={{ marginBottom: 16 }}>
            <Descriptions.Item label="组数">{metrics.group_count}</Descriptions.Item>
            <Descriptions.Item label="组内同质性">{(metrics.group_homogeneity * 100).toFixed(1)}%</Descriptions.Item>
            <Descriptions.Item label="批量回滚率">{(metrics.batch_override_rate * 100).toFixed(1)}%</Descriptions.Item>
            <Descriptions.Item label="节省人工动作">{metrics.human_actions_saved}</Descriptions.Item>
            <Descriptions.Item label="事后审计误差">{metrics.post_audit_error_rate === undefined ? "尚未采集" : `${(metrics.post_audit_error_rate * 100).toFixed(1)}%`}</Descriptions.Item>
          </Descriptions>
        ) : null}

        <Spin spinning={loading}>
          {!groups.length ? <Empty description={questionId ? "暂无答案组，可构建分组候选" : "请先选择题目"} /> : (
            <div style={{ display: "grid", gridTemplateColumns: "280px minmax(0, 1fr)", gap: 16 }}>
              <List
                bordered
                dataSource={groups}
                renderItem={(group) => (
                  <List.Item onClick={() => setSelectedGroupId(group.id)} style={{ cursor: "pointer", background: group.id === selectedGroup?.id ? "#f0f7ff" : undefined }}>
                    <List.Item.Meta
                      title={<Space><span>{group.member_count} 份答案</span><Tag>{statusLabels[group.status] ?? "未知状态"}</Tag></Space>}
                      description={`同质性 ${(group.homogeneity * 100).toFixed(1)}% · 抽检 ${group.reviewed_sample_count}/${group.minimum_sample}`}
                    />
                  </List.Item>
                )}
              />
              {selectedGroup ? (
                <section>
                  <Space direction="vertical" style={{ width: "100%" }} size="middle">
                    <div>
                      <Space style={{ width: "100%", justifyContent: "space-between" }}>
                        <strong>抽检进度</strong><span>{selectedGroup.reviewed_sample_count} / 至少 {selectedGroup.minimum_sample}</span>
                      </Space>
                      <Progress percent={selectedGroup.minimum_sample ? Math.min(100, Math.round(selectedGroup.reviewed_sample_count / selectedGroup.minimum_sample * 100)) : 100} status={selectedGroup.can_confirm ? "success" : "active"} />
                    </div>
                    <List
                      size="small"
                      bordered
                      header="代表 / 边界 / 异常样本"
                      dataSource={[...selectedGroup.members].sort((left, right) => Number(right.representative) - Number(left.representative) || Number(right.boundary) - Number(left.boundary) || Number(right.outlier) - Number(left.outlier))}
                      renderItem={(member) => (
                        <List.Item actions={canManage && selectedGroup.status !== "confirmed" && selectedGroup.status !== "rolled_back" ? [
                          <Button key="accept" size="small" type={member.sample_status === "accepted" ? "primary" : "default"} onClick={() => void reviewSample(member.segment_id, "accepted")}>接受</Button>,
                          <Button key="reject" size="small" danger type={member.sample_status === "rejected" ? "primary" : "default"} onClick={() => void reviewSample(member.segment_id, "rejected")}>拒绝</Button>
                        ] : undefined}>
                          <List.Item.Meta
                            title={<Space wrap><span>答卷 {member.submission_id.slice(0, 10)}</span>{sampleRoleLabels(member).map((label) => <Tag key={label} color={label === "异常样本" ? "red" : label === "边界样本" ? "orange" : "blue"}>{label}</Tag>)}</Space>}
                            description={`相似度 ${(member.similarity * 100).toFixed(1)}% · 异常度 ${(member.outlier_score * 100).toFixed(1)}%${member.sample_status ? ` · ${member.sample_status === "accepted" ? "已接受" : "已拒绝"}` : " · 未抽检"}`}
                          />
                        </List.Item>
                      )}
                    />
                    {canManage && selectedGroup.status !== "confirmed" && selectedGroup.status !== "rolled_back" ? (
                      <section>
                        <Divider orientation="left">组评分候选</Divider>
                        <Space wrap>
                          <InputNumber min={0} value={score} onChange={(value) => setScore(value === null ? null : Number(value))} placeholder="候选分数" />
                          <Select mode="tags" style={{ minWidth: 320 }} value={rubricPoints} onChange={setRubricPoints} tokenSeparators={[","]} placeholder="评分细则采分点（输入后回车）" />
                          <Button loading={acting} onClick={saveDecision}>保存候选</Button>
                          <Tooltip title={blockReason ?? "抽检策略已满足"}>
                            <span>
                              <Popconfirm title="确认生成整组评分候选？" description="系统只写入可追溯候选，不会写最终成绩。" onConfirm={() => void confirm()}>
                                <Button type="primary" disabled={Boolean(blockReason)} loading={acting}>确认组候选</Button>
                              </Popconfirm>
                            </span>
                          </Tooltip>
                        </Space>
                        {blockReason ? <Alert type="warning" showIcon message="暂不能确认" description={blockReason} style={{ marginTop: 12 }} /> : null}
                      </section>
                    ) : null}
                    {selectedGroup.status === "confirmed" && selectedGroup.decision?.rollback_reference && canManage ? (
                      <Alert type="success" showIcon message="已生成自动化评分候选" description={<Space><span>候选仍需下游评分流程确认，不是最终成绩。</span><Button danger size="small" onClick={() => setRollbackOpen(true)}>回滚候选</Button></Space>} />
                    ) : null}
                  </Space>
                </section>
              ) : null}
            </div>
          )}
        </Spin>

        <Divider orientation="left">教师参考案例（仅已批准标准卷）</Divider>
        <List
          size="small"
          bordered
          dataSource={references}
          locale={{ emptyText: "当前题目暂无已批准标准卷参考案例" }}
          renderItem={(item) => <List.Item><List.Item.Meta title={`${item.reference_score} / ${item.max_score} 分 · 标准卷 v${item.version}`} description={<Space direction="vertical" size={2}><span>{item.explanation}</span>{item.error_tags.length ? <span>{item.error_tags.map((tag) => <Tag key={tag}>{tag}</Tag>)}</span> : null}</Space>} /></List.Item>}
        />
      </Drawer>
      <Modal title="回滚整组候选" open={rollbackOpen} confirmLoading={acting} okButtonProps={{ danger: true }} onOk={() => void rollback()} onCancel={() => setRollbackOpen(false)}>
        <Alert type="warning" showIcon message="回滚只撤销本次组候选，不会覆盖已形成的最终成绩事实。" style={{ marginBottom: 12 }} />
        <Input.TextArea rows={3} value={rollbackReason} onChange={(event) => setRollbackReason(event.target.value)} placeholder="填写回滚原因，便于审计追溯" />
      </Modal>
    </>
  );
}
