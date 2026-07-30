import { Empty, Input, Select, Space, Tag, type TableColumnsType } from "antd";
import { Database, Search } from "lucide-react";
import { useMemo, useState } from "react";
import type {
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
        />
      </div>
    </div>
  );
}
