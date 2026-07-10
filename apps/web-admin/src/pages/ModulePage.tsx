import { Button, Progress, Space, TableColumnsType } from "antd";
import { ArrowRight, Plus } from "lucide-react";
import { exams, submissions } from "../data";
import type { ExamRow, SubmissionRow } from "../types";
import type { AppRoute } from "../router/routes";
import { DataTable } from "../components/DataTable";
import { FormShell } from "../components/FormShell";
import { MockBadge } from "../components/MockBadge";
import { StatusTag } from "../components/StatusTag";

export function ModulePage({ route }: { route: AppRoute }) {
  const examColumns: TableColumnsType<ExamRow> = [
    { title: "名称", dataIndex: "name" },
    { title: "学科", dataIndex: "subject", width: 90 },
    { title: "状态", dataIndex: "status", render: (value: string) => <StatusTag tone="processing">{value}</StatusTag> },
    { title: "答卷", dataIndex: "papers", width: 90 },
    { title: "进度", dataIndex: "progress", render: (value: number) => <Progress percent={value} size="small" /> },
    { title: "模式", dataIndex: "mode" }
  ];
  const submissionColumns: TableColumnsType<SubmissionRow> = [
    { title: "答卷", dataIndex: "id" },
    { title: "题号", dataIndex: "question", width: 80 },
    { title: "状态", dataIndex: "status", render: (value: string) => <StatusTag tone={value.includes("失败") ? "danger" : value.includes("复核") ? "warning" : "info"}>{value}</StatusTag> },
    { title: "分数", dataIndex: "score", width: 90 },
    { title: "风险", dataIndex: "risk" },
    { title: "处理池", dataIndex: "owner" }
  ];
  const isExamLike = ["考试管理", "试卷管理", "成绩管理", "学情报告"].includes(route.title);

  return (
    <div className="page-stack">
      <section className="page-heading">
        <div>
          <Space>
            <h1>{route.title}</h1>
            <MockBadge />
          </Space>
          <p>{route.title} 的基础列表、筛选、状态和操作入口。</p>
        </div>
        <Space wrap>
          <Button icon={<Plus size={16} />}>新建</Button>
          <Button type="primary" icon={<ArrowRight size={16} />}>打开详情</Button>
        </Space>
      </section>

      {isExamLike ? (
        <DataTable title={`${route.title}列表`} rows={exams} columns={examColumns} searchPlaceholder="搜索名称" filterLabel="状态" filterOptions={["人工复核中", "待发布", "等待 OCR"]} />
      ) : (
        <DataTable title={`${route.title}队列`} rows={submissions} columns={submissionColumns} searchPlaceholder="搜索答卷" filterLabel="状态" filterOptions={["等待人工复核", "等待仲裁", "OCR 失败"]} />
      )}

      <FormShell title={`${route.title}表单`} />
    </div>
  );
}
