import type { StudentQuestion, StudentResult } from "../../api";

const subjectNames: Record<string, string> = {
  math: "数学", chinese: "语文", english: "英语", physics: "物理",
  chemistry: "化学", biology: "生物", history: "历史", geography: "地理", politics: "政治"
};

export function displaySubject(subject?: string) {
  if (!subject) return "考试";
  return subjectNames[subject.toLowerCase()] ?? subject;
}

export function ScoreHero({ result }: { result: StudentResult }) {
  const rate = Math.round((result.score_rate ?? (result.max_score > 0 ? result.total_score / result.max_score : 0)) * 1000) / 10;
  const overallScore = result.overall_total_score ?? result.total_score;
  const overallMax = result.overall_max_score ?? result.max_score;
  return <section className="score-hero" aria-label="总分">
    <Metric label="单科试卷得分" value={`${formatScore(result.total_score)} / ${formatScore(result.max_score)}`} note={`得分率 ${rate}%`} />
    <Metric label="总分" value={`${formatScore(overallScore)} / ${formatScore(overallMax)}`} />
    <Metric label="班级排名" value={rank(result.rankings?.class_rank, result.rankings?.class_size)} />
    <Metric label="年级排名" value={rank(result.rankings?.grade_rank, result.rankings?.grade_size)} />
  </section>;
}

function Metric({ label, value, note }: { label: string; value: string; note?: string }) {
  return <div className="score-metric"><span>{label}</span><strong>{value}</strong>{note ? <small>{note}</small> : null}</div>;
}

function rank(value?: number, size?: number) {
  return value && size ? `${value} / ${size}` : "—";
}

export function ExamDiagnosis({ questions, onStart }: { questions: StudentQuestion[]; onStart: () => void }) {
  const missed = questions.filter((item) => item.score < item.max_score);
  if (missed.length === 0) return null;
  const partial = missed.filter((item) => item.score > 0);
  const biggest = [...missed].sort((a, b) => (b.max_score - b.score) - (a.max_score - a.score)).slice(0, 3);
  const knowledge = [...new Set(missed.flatMap((item) => item.knowledge_points ?? []))].slice(0, 3);
  return <section className="diagnosis" aria-labelledby="diagnosis-title">
    <div><h2 id="diagnosis-title">重点复盘</h2><p>{missed.length} 道失分题 · {partial.length} 道部分得分</p></div>
    <div className="diagnosis-list"><span>主要失分</span>{biggest.map((item) => <p key={item.question_id}>第 {item.question_no} 题 <strong>−{formatScore(item.max_score - item.score)} 分</strong></p>)}</div>
    {knowledge.length > 0 ? <div className="diagnosis-list"><span>相关知识</span>{knowledge.map((item) => <p key={item}>{item}</p>)}</div> : null}
    <button type="button" className="primary-action" onClick={onStart}>查看失分题 <span>→</span></button>
  </section>;
}

export function formatScore(value: number) {
  return Number.isInteger(value) ? String(value) : value.toFixed(1).replace(/\.0$/, "");
}
