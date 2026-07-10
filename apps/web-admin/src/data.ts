import type { AgentStage, AuditItem, ExamRow, Metric, RubricPoint, SubmissionRow } from "./types";

export const metrics: Metric[] = [
  { label: "今日待阅卷", value: "1,248", trend: "+18%", status: "warning" },
  { label: "OCR 待处理", value: "328", trend: "4 个队列", status: "info" },
  { label: "AI 阅卷中", value: "7,604", trend: "92.4% 有证据", status: "processing" },
  { label: "待人工复核", value: "186", trend: "低置信度自动转入", status: "danger" },
  { label: "待仲裁", value: "43", trend: "双评分差超阈值", status: "danger" },
  { label: "待发布成绩", value: "5", trend: "需年级主任确认", status: "success" },
  { label: "待处理申诉", value: "17", trend: "平均 6.2 小时", status: "warning" },
  { label: "异常预警", value: "29", trend: "同类答案分差", status: "danger" }
];

export const pipeline = [
  { name: "上传", done: 12400, total: 12400 },
  { name: "预处理", done: 12186, total: 12400 },
  { name: "OCR", done: 11720, total: 12400 },
  { name: "切分", done: 11342, total: 12400 },
  { name: "AI 初评", done: 9360, total: 12400 },
  { name: "人工复核", done: 5210, total: 12400 },
  { name: "发布", done: 0, total: 12400 }
];

export const scoreDistribution = [
  { band: "0-39", count: 42 },
  { band: "40-49", count: 96 },
  { band: "50-59", count: 218 },
  { band: "60-69", count: 386 },
  { band: "70-79", count: 512 },
  { band: "80-89", count: 421 },
  { band: "90-100", count: 148 }
];

export const questionAverages = [
  { question: "Q1", avg: 4.6, full: 5 },
  { question: "Q2", avg: 6.8, full: 8 },
  { question: "Q3", avg: 7.2, full: 10 },
  { question: "Q4", avg: 5.9, full: 8 },
  { question: "Q5", avg: 11.3, full: 15 },
  { question: "Q6", avg: 13.1, full: 20 }
];

export const trend = [
  { day: "07-01", ocr: 93.2, ai: 61.8, human: 18.4 },
  { day: "07-02", ocr: 94.6, ai: 65.3, human: 17.6 },
  { day: "07-03", ocr: 95.1, ai: 68.9, human: 16.2 },
  { day: "07-04", ocr: 95.7, ai: 72.4, human: 14.8 },
  { day: "07-05", ocr: 96.2, ai: 75.2, human: 13.9 },
  { day: "07-06", ocr: 96.5, ai: 76.1, human: 13.4 }
];

export const agentStages: AgentStage[] = [
  { name: "版面解析 Agent", state: "done", count: 12400, detail: "题号、页码、答题区已识别" },
  { name: "OCR Agent", state: "running", count: 11720, detail: "680 页低置信度等待校正" },
  { name: "答案切分 Agent", state: "running", count: 11342, detail: "按学生与题目拆分" },
  { name: "Rubric 解析 Agent", state: "done", count: 18, detail: "v3 已审批" },
  { name: "简答题 Agent", state: "running", count: 7620, detail: "AI 建议分不可直接终审" },
  { name: "证据校验 Agent", state: "queued", count: 3860, detail: "检查每个采分点证据" },
  { name: "一致性 Agent", state: "queued", count: 1920, detail: "同类答案分差检测" },
  { name: "审计 Agent", state: "done", count: 38214, detail: "过程日志不可关闭" }
];

export const rubric: RubricPoint[] = [
  { id: "p1", title: "受力分析完整", score: 2, state: "hit", evidence: "第 2 行列出重力、支持力、合外力" },
  { id: "p2", title: "正确写出 F=ma", score: 2, state: "hit", evidence: "第 3 行出现 ΣF=ma" },
  { id: "p3", title: "代入数据正确", score: 2, state: "partial", evidence: "代入质量正确，加速度单位遗漏" },
  { id: "p4", title: "结果和单位正确", score: 2, state: "missing", evidence: "最终结果缺少 N，触发扣分" }
];

export const submissions: SubmissionRow[] = [
  { id: "ANS-22041", question: "Q18", status: "等待人工复核", score: "6 / 8", confidence: "0.81", risk: "单位缺失", owner: "物理组 A" },
  { id: "ANS-22042", question: "Q18", status: "已完成", score: "8 / 8", confidence: "0.94", risk: "无", owner: "物理组 A" },
  { id: "ANS-22043", question: "Q20", status: "等待仲裁", score: "15 / 20", confidence: "0.77", risk: "双评分差 5", owner: "仲裁池" },
  { id: "ANS-22044", question: "Q12", status: "OCR 失败", score: "-", confidence: "0.42", risk: "图片模糊", owner: "采集复核" },
  { id: "ANS-22045", question: "Q06", status: "AI 阅卷中", score: "-", confidence: "-", risk: "队列中", owner: "Agent Runtime" }
];

export const exams: ExamRow[] = [
  { name: "高二物理期末考试", subject: "物理", status: "人工复核中", papers: 1864, progress: 72, mode: "双评 + 仲裁" },
  { name: "初三数学联考", subject: "数学", status: "等待 OCR", papers: 3240, progress: 28, mode: "AI 初评 + 教师确认" },
  { name: "英语写作专项测评", subject: "英语", status: "评分标准审批", papers: 980, progress: 12, mode: "盲评 + 仲裁" },
  { name: "企业培训合规考试", subject: "培训", status: "待发布", papers: 420, progress: 96, mode: "自动阅卷" }
];

export const auditItems: AuditItem[] = [
  { time: "14:22:18", actor: "李老师", action: "覆盖 AI 建议分", target: "ANS-22041 / Q18", result: "6 -> 5.5，理由：单位遗漏" },
  { time: "14:19:03", actor: "系统", action: "触发人工复核", target: "ANS-22041 / Q18", result: "OCR 0.84，AI 0.81" },
  { time: "14:16:44", actor: "周主任", action: "审批 Rubric v3", target: "高二物理 Q18", result: "通过" },
  { time: "14:10:08", actor: "扫描工作站-03", action: "上传答卷批次", target: "BATCH-7791", result: "1200 页，17 页低质量" },
  { time: "14:08:51", actor: "审计 Agent", action: "冻结日志链", target: "EXAM-2026-PHY", result: "hash: 9f32...a18c" }
];

export const qualityRows = [
  { metric: "Exact Agreement", value: "71.4%", target: ">= 68%", status: "达标" },
  { metric: "Adjacent Agreement", value: "94.2%", target: ">= 92%", status: "达标" },
  { metric: "MAE", value: "0.38", target: "<= 0.45", status: "达标" },
  { metric: "QWK", value: "0.86", target: ">= 0.82", status: "达标" },
  { metric: "Bias Gap", value: "1.8%", target: "<= 3%", status: "达标" },
  { metric: "Review Trigger Recall", value: "91.7%", target: ">= 90%", status: "达标" }
];
