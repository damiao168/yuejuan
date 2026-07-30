import { Alert, Button, Empty, Input, Select, Space, Tag, type TableColumnsType } from "antd";
import { BarChart3, Database, Eye, Search, ShieldCheck, TriangleAlert } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import type {
  EvaluationCandidate,
  EvaluationEvidenceClass,
  EvaluationRun,
  EvaluationRunStatus,
  ModelDeployment
} from "../../api/modelGovernance";
import { ResponsiveTable } from "../ResponsiveTable";

const statusLabels: Record<EvaluationRunStatus, string> = {
  draft: "采集中",
  completed: "已冻结",
  invalidated: "已失效"
};

const evidenceLabels: Record<EvaluationEvidenceClass, string> = {
  protocol_fixture: "协议样例",
  authorized_frozen_set: "授权冻结集"
};

function statusColor(status: EvaluationRunStatus) {
  if (status === "completed") return "success";
  if (status === "invalidated") return "default";
  return "processing";
}

function formatTime(value?: string) {
  if (!value) return "-";
  const parsed = new Date(value);
  return Number.isNaN(parsed.getTime()) ? value : parsed.toLocaleString("zh-CN", { hour12: false });
}

function formatPercent(value: number) {
  return `${(value * 100).toFixed(1)}%`;
}

function formatAverageCost(value: number) {
  return value === 0 ? "¥0" : `¥${(value / 1_000_000).toFixed(6)}`;
}

export function ModelEvaluationWorkspace({
  runs,
  deployments: _deployments,
  loading,
  canManage: _canManage,
  onRefresh: _onRefresh
}: {
  runs: EvaluationRun[];
  deployments: ModelDeployment[];
  loading: boolean;
  canManage: boolean;
  onRefresh: () => Promise<void>;
}) {
  const [keyword, setKeyword] = useState("");
  const [status, setStatus] = useState<EvaluationRunStatus | "all">("all");
  const [evidenceClass, setEvidenceClass] = useState<EvaluationEvidenceClass | "all">("all");
  const [selectedRunID, setSelectedRunID] = useState<string>();

  const filteredRuns = useMemo(() => {
    const normalizedKeyword = keyword.trim().toLocaleLowerCase();
    return runs.filter((run) => {
      const matchesKeyword = !normalizedKeyword || [
        run.display_name,
        run.run_key,
        run.dataset_reference,
        run.subject,
        run.grade,
        run.question_type
      ].some((value) => value.toLocaleLowerCase().includes(normalizedKeyword));
      return matchesKeyword
        && (status === "all" || run.status === status)
        && (evidenceClass === "all" || run.evidence_class === evidenceClass);
    });
  }, [evidenceClass, keyword, runs, status]);

  useEffect(() => {
    if (selectedRunID && runs.some((run) => run.id === selectedRunID)) return;
    setSelectedRunID(runs[0]?.id);
  }, [runs, selectedRunID]);

  const selectedRun = runs.find((run) => run.id === selectedRunID);

  const bestMetrics = useMemo(() => {
    if (!selectedRun?.candidates.length) return undefined;
    const metrics = selectedRun.candidates.map((candidate) => candidate.metrics);
    return {
      teacherAcceptance: Math.max(...metrics.map((item) => item.teacher_acceptance_rate)),
      seriousError: Math.min(...metrics.map((item) => item.serious_error_rate)),
      evidenceValidity: Math.max(...metrics.map((item) => item.evidence_validity_rate)),
      stability: Math.max(...metrics.map((item) => item.stability_rate)),
      latency: Math.min(...selectedRun.candidates.map((item) => item.p95_latency_ms)),
      cost: Math.min(...metrics.map((item) => item.average_cost_micros))
    };
  }, [selectedRun]);

  const columns: TableColumnsType<EvaluationRun> = [
    {
      title: "评测批次",
      dataIndex: "display_name",
      render: (_value, run) => (
        <div className="model-evaluation-run">
          <span><Database size={15} /></span>
          <div>
            <strong>{run.display_name}</strong>
            <small>{run.run_key}</small>
          </div>
        </div>
      )
    },
    {
      title: "适用范围",
      key: "scope",
      render: (_value, run) => (
        <div className="model-governance-stack">
          <span>{run.subject} · {run.grade}</span>
          <small>{run.question_type} · {run.modality === "image" ? "图片" : "文本"}</small>
        </div>
      )
    },
    {
      title: "证据",
      dataIndex: "evidence_class",
      render: (value: EvaluationEvidenceClass, run) => (
        <div className="model-governance-stack">
          <span>{evidenceLabels[value]}</span>
          <small>{run.dataset_reference}</small>
        </div>
      )
    },
    {
      title: "样本 / 重复",
      key: "samples",
      width: 130,
      render: (_value, run) => `${run.sample_count} / ${run.repeat_count}`
    },
    {
      title: "候选",
      dataIndex: "candidates",
      width: 88,
      render: (candidates: EvaluationRun["candidates"]) => candidates.length
    },
    {
      title: "状态",
      dataIndex: "status",
      width: 105,
      render: (value: EvaluationRunStatus) => <Tag color={statusColor(value)}>{statusLabels[value]}</Tag>
    },
    {
      title: "创建时间",
      dataIndex: "created_at",
      width: 178,
      render: (value: string) => formatTime(value)
    },
    {
      title: "操作",
      key: "action",
      width: 90,
      render: (_value, run) => (
        <Button
          type={selectedRunID === run.id ? "primary" : "link"}
          size="small"
          icon={<Eye size={14} />}
          onClick={(event) => {
            event.stopPropagation();
            setSelectedRunID(run.id);
          }}
        >
          对比
        </Button>
      )
    }
  ];

  const candidateColumns: TableColumnsType<EvaluationCandidate> = [
    {
      title: "候选部署",
      dataIndex: "deployment_key",
      render: (_value, candidate) => (
        <div className="model-governance-stack">
          <span>{candidate.deployment_key}</span>
          <small>{candidate.provider_key} · {candidate.model_version}</small>
        </div>
      )
    },
    {
      title: "提示词 / Rubric",
      key: "versions",
      render: (_value, candidate) => (
        <div className="model-governance-stack">
          <span>{candidate.prompt_version}</span>
          <small>{candidate.rubric_version}</small>
        </div>
      )
    },
    {
      title: "教师接受率",
      dataIndex: ["metrics", "teacher_acceptance_rate"],
      width: 120,
      render: (value: number) => selectedRun?.evidence_class === "protocol_fixture"
        ? <span className="muted">不适用</span>
        : <strong className={value === bestMetrics?.teacherAcceptance ? "model-evaluation-best" : ""}>{formatPercent(value)}</strong>
    },
    {
      title: "严重错误率",
      dataIndex: ["metrics", "serious_error_rate"],
      width: 118,
      render: (value: number) => (
        <strong className={value === bestMetrics?.seriousError ? "model-evaluation-best" : value > 0 ? "model-evaluation-risk" : ""}>
          {formatPercent(value)}
        </strong>
      )
    },
    {
      title: "证据有效率",
      dataIndex: ["metrics", "evidence_validity_rate"],
      width: 118,
      render: (value: number) => <strong className={value === bestMetrics?.evidenceValidity ? "model-evaluation-best" : ""}>{formatPercent(value)}</strong>
    },
    {
      title: "稳定性",
      dataIndex: ["metrics", "stability_rate"],
      width: 105,
      render: (value: number, candidate) => candidate.repeat_comparisons === 0
        ? <span className="muted">未重复</span>
        : <strong className={value === bestMetrics?.stability ? "model-evaluation-best" : ""}>{formatPercent(value)}</strong>
    },
    {
      title: "P95 时延",
      dataIndex: "p95_latency_ms",
      width: 105,
      render: (value: number) => <span className={value === bestMetrics?.latency ? "model-evaluation-best" : ""}>{value} ms</span>
    },
    {
      title: "平均成本",
      dataIndex: ["metrics", "average_cost_micros"],
      width: 116,
      render: (value: number) => <span className={value === bestMetrics?.cost ? "model-evaluation-best" : ""}>{formatAverageCost(value)}</span>
    }
  ];

  return (
    <div className="model-evaluation-workspace">
      <div className="model-evaluation-filterbar">
        <Input
          allowClear
          prefix={<Search size={15} />}
          placeholder="搜索批次、数据集或适用范围"
          value={keyword}
          onChange={(event) => setKeyword(event.target.value)}
        />
        <Space wrap>
          <Select
            aria-label="评测状态"
            value={status}
            onChange={setStatus}
            options={[
              { value: "all", label: "全部状态" },
              { value: "draft", label: statusLabels.draft },
              { value: "completed", label: statusLabels.completed },
              { value: "invalidated", label: statusLabels.invalidated }
            ]}
          />
          <Select
            aria-label="证据类别"
            value={evidenceClass}
            onChange={setEvidenceClass}
            options={[
              { value: "all", label: "全部证据" },
              { value: "protocol_fixture", label: evidenceLabels.protocol_fixture },
              { value: "authorized_frozen_set", label: evidenceLabels.authorized_frozen_set }
            ]}
          />
        </Space>
      </div>
      <div className="model-evaluation-count">
        <span>显示 {filteredRuns.length} / {runs.length} 个批次</span>
        <small>这里只记录离线对比证据，不改变当前评分路由。</small>
      </div>
      <div className="model-governance-table">
        <ResponsiveTable
          rowKey="id"
          columns={columns}
          dataSource={filteredRuns}
          loading={loading}
          pagination={{ pageSize: 10, hideOnSinglePage: true }}
          locale={{ emptyText: <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无符合条件的评测批次" /> }}
          onRow={(run) => ({ onClick: () => setSelectedRunID(run.id) })}
          rowClassName={(run) => run.id === selectedRunID ? "model-evaluation-selected-row" : ""}
        />
      </div>
      {selectedRun ? (
        <section className="model-evaluation-detail" aria-label="评测候选对比">
          <header>
            <div>
              <span className="model-evaluation-detail-kicker"><BarChart3 size={15} /> 候选并排对比</span>
              <h2>{selectedRun.display_name}</h2>
              <p>
                {selectedRun.subject} · {selectedRun.grade} · {selectedRun.question_type}
                <span>数据集 {selectedRun.dataset_reference}</span>
              </p>
            </div>
            <Tag color={statusColor(selectedRun.status)}>{statusLabels[selectedRun.status]}</Tag>
          </header>

          <div className="model-evaluation-evidence">
            {selectedRun.status === "invalidated" ? (
              <Alert
                type="error"
                showIcon
                icon={<TriangleAlert size={17} />}
                message="这组评测证据已失效"
                description={`失效时间：${formatTime(selectedRun.invalidated_at)}。历史结果保留用于审计，不应再用于质量判断。`}
              />
            ) : selectedRun.evidence_class === "authorized_frozen_set" ? (
              <Alert
                type="success"
                showIcon
                icon={<ShieldCheck size={17} />}
                message="授权冻结集证据"
                description={`授权引用：${selectedRun.authorization_reference}；SHA-256：${selectedRun.dataset_sha256}。这类证据可进入后续质量评审，但不等于模型已批准。`}
              />
            ) : (
              <Alert
                type="info"
                showIcon
                message="协议样例证据"
                description={`SHA-256：${selectedRun.dataset_sha256}。只验证协议、结构和产品流程，不代表真实教师接受率或生产评分效果。`}
              />
            )}
          </div>

          <div className="model-evaluation-candidate-table">
            <ResponsiveTable
              rowKey="id"
              columns={candidateColumns}
              dataSource={selectedRun.candidates}
              pagination={false}
              mobilePrimaryCount={4}
              locale={{
                emptyText: <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="该批次还没有候选结果" />
              }}
            />
          </div>
          <footer>
            <span><i className="model-evaluation-legend-best" /> 同批次内的相对最优值</span>
            <small>不同数据集、题型或 Rubric 版本之间不可直接横向比较。</small>
          </footer>
        </section>
      ) : null}
    </div>
  );
}
