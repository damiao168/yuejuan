import { FormEvent, PointerEvent, useCallback, useEffect, useMemo, useState } from "react";
import {
  createQuestionAppeal,
  currentUser,
  getQuestion,
  getResult,
  listQuestionAnnotations,
  listQuestionAppeals,
  listPublishedExams,
  login,
  logout,
  PortalApiError,
  studentQuestionAnswerImageURL,
  type AuthUser,
  type PublishedExam,
  type StudentQuestionAppeal,
  type StudentQuestionAnnotation,
  type StudentQuestion,
  type SelectedAppealRegion,
  type StudentResult
} from "./api";

type Page = { kind: "home" } | { kind: "exam"; examID: string };

function readPage(): Page {
  const value = window.location.hash.replace(/^#/, "");
  const match = value.match(/^\/exams\/([^/?#]+)$/);
  if (match) {
    try {
      return { kind: "exam", examID: decodeURIComponent(match[1]) };
    } catch {
      return { kind: "home" };
    }
  }
  return { kind: "home" };
}

function goHome() { window.location.hash = "/"; }
function goExam(examID: string) { window.location.hash = `/exams/${encodeURIComponent(examID)}`; }

export default function App() {
  const [user, setUser] = useState<AuthUser | null>(null);
  const [loadingUser, setLoadingUser] = useState(true);
  const [page, setPage] = useState<Page>(readPage);

  const restore = useCallback(async () => {
    setLoadingUser(true);
    try {
      const response = await currentUser();
      setUser(response.user);
    } catch {
      setUser(null);
    } finally {
      setLoadingUser(false);
    }
  }, []);

  useEffect(() => { void restore(); }, [restore]);
  useEffect(() => {
    const onChange = () => setPage(readPage());
    window.addEventListener("hashchange", onChange);
    return () => window.removeEventListener("hashchange", onChange);
  }, []);

  if (loadingUser) {
    return <main className="portal-loading">正在确认登录状态…</main>;
  }
  if (!user) {
    return <LoginScreen onLoggedIn={setUser} />;
  }
  const eligible = user.roles.includes("student") && user.permissions.includes("student:grade:read");
  if (!eligible) {
    return <AccessDenied user={user} onLogout={() => void logout().finally(() => setUser(null))} />;
  }
  return (
    <div className="portal-shell">
      <header className="portal-header">
        <button className="brand" type="button" onClick={goHome} aria-label="回到我的考试">
          <span className="brand-mark">E</span>
          <span><strong>EduGrade</strong><small>学生端</small></span>
        </button>
        <div className="account-area">
          <span>{user.display_name || user.username}</span>
          <button type="button" className="text-button" onClick={() => void logout().finally(() => setUser(null))}>退出登录</button>
        </div>
      </header>
      <main className="portal-content">
        {page.kind === "home" ? <ExamList onOpen={goExam} /> : <ResultDetail examID={page.examID} onBack={goHome} />}
      </main>
    </div>
  );
}

function LoginScreen({ onLoggedIn }: { onLoggedIn: (user: AuthUser) => void }) {
  const [tenantCode, setTenantCode] = useState("");
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [submitting, setSubmitting] = useState(false);

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setSubmitting(true);
    setError("");
    try {
      const response = await login({ tenant_code: tenantCode.trim(), username: username.trim(), password });
      onLoggedIn(response.user);
    } catch (reason) {
      setError(friendlyError(reason));
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <main className="login-screen">
      <section className="login-panel" aria-labelledby="login-title">
        <div className="brand login-brand"><span className="brand-mark">E</span><span><strong>EduGrade</strong><small>学生端</small></span></div>
        <h1 id="login-title">查看已发布成绩</h1>
        <p>使用学校发放的学生账号登录。未发布的成绩不会在此显示。</p>
        <form onSubmit={submit}>
          <label>学校代码<input required value={tenantCode} onChange={(event) => setTenantCode(event.target.value)} autoComplete="organization" /></label>
          <label>账号<input required value={username} onChange={(event) => setUsername(event.target.value)} autoComplete="username" /></label>
          <label>密码<input required type="password" value={password} onChange={(event) => setPassword(event.target.value)} autoComplete="current-password" /></label>
          {error ? <p className="form-error" role="alert">{error}</p> : null}
          <button className="primary-button" disabled={submitting} type="submit">{submitting ? "正在登录…" : "登录"}</button>
        </form>
      </section>
    </main>
  );
}

function AccessDenied({ user, onLogout }: { user: AuthUser; onLogout: () => void }) {
  return (
    <main className="portal-loading access-denied">
      <h1>此账号不是学生账号</h1>
      <p>学生端只显示本人已发布的成绩。请使用学校发放的学生账号登录。</p>
      <p className="muted">当前账号：{user.display_name || user.username}</p>
      <button className="primary-button" type="button" onClick={onLogout}>退出登录</button>
    </main>
  );
}

function ExamList({ onOpen }: { onOpen: (examID: string) => void }) {
  const [items, setItems] = useState<PublishedExam[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  const load = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      const response = await listPublishedExams();
      setItems(response.exams);
    } catch (reason) {
      setError(friendlyError(reason));
    } finally {
      setLoading(false);
    }
  }, []);
  useEffect(() => { void load(); }, [load]);

  return (
    <section className="exam-list-page" aria-labelledby="exam-list-title">
      <div className="page-heading">
        <div><p className="eyebrow">我的考试</p><h1 id="exam-list-title">已发布成绩</h1><p>只展示学校已正式发布的成绩版本。</p></div>
        <button type="button" className="secondary-button" onClick={() => void load()} disabled={loading}>刷新</button>
      </div>
      {error ? <ErrorNotice message={error} onRetry={load} /> : null}
      {loading ? <div className="inline-status">正在加载已发布考试…</div> : null}
      {!loading && !error && items.length === 0 ? <EmptyState /> : null}
      {!loading && !error && items.length > 0 ? <div className="exam-list">
        {items.map((exam) => <button className="exam-row" type="button" key={exam.exam_id} onClick={() => onOpen(exam.exam_id)}>
          <div><strong>{exam.name}</strong><span>{exam.subject || "考试"} · 已发布 {formatDate(exam.published_at)}</span></div>
          <div className="exam-row-action"><small>第 {exam.release_version} 版</small><span>查看成绩 →</span></div>
        </button>)}
      </div> : null}
    </section>
  );
}

function ResultDetail({ examID, onBack }: { examID: string; onBack: () => void }) {
  const [result, setResult] = useState<StudentResult | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [appealRevision, setAppealRevision] = useState(0);

  const load = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      const response = await getResult(examID);
      setResult(response.result);
    } catch (reason) {
      setError(friendlyError(reason));
    } finally {
      setLoading(false);
    }
  }, [examID]);
  useEffect(() => { void load(); }, [load]);

  if (loading) return <div className="inline-status">正在加载成绩详情…</div>;
  if (error || !result) return <ErrorNotice message={error || "暂时无法取得成绩"} onRetry={load} onBack={onBack} />;
  const percentage = result.max_score > 0 ? Math.round(result.total_score / result.max_score * 100) : 0;
  const questionsVisible = Array.isArray(result.questions);
  return (
    <section className="result-page" aria-labelledby="result-title">
      <button type="button" className="back-button" onClick={onBack}>← 返回我的考试</button>
      <div className="result-heading">
        <div><p className="eyebrow">已发布成绩 · 第 {result.release_version} 版</p><h1 id="result-title">本次考试成绩</h1><p>该成绩为学校当前正式发布版本。</p></div>
        <div className="release-state"><span>已发布</span><small>成绩如有更正，将以新版本显示</small></div>
      </div>
      <section className="score-summary" aria-label="总分">
        <div><span>总分</span><strong>{formatScore(result.total_score)}<small> / {formatScore(result.max_score)}</small></strong></div>
        <div className="score-rate"><span>得分率</span><strong>{percentage}%</strong><div><i style={{ width: `${Math.max(0, Math.min(100, percentage))}%` }} /></div></div>
      </section>
      <section className="question-section" aria-labelledby="questions-title">
        <header><div><h2 id="questions-title">题目反馈</h2><p>{questionsVisible ? "点击题目查看学校公开的反馈与评分要点。" : "本次考试仅公开总分，题目得分与反馈未对学生公开。"}</p></div></header>
        {questionsVisible && result.questions && result.questions.length > 0 ? <QuestionList examID={examID} releaseID={result.release_id} releaseVersion={result.release_version} appealWindow={result.appeal_window} questions={result.questions} onAppealSubmitted={() => setAppealRevision((current) => current + 1)} /> : null}
        {questionsVisible && result.questions?.length === 0 ? <p className="muted section-empty">学校暂未提供逐题结果。</p> : null}
      </section>
      <AppealWindow result={result} />
      <AppealStatus examID={examID} revision={appealRevision} />
    </section>
  );
}

function QuestionList({ examID, releaseID, releaseVersion, appealWindow, questions, onAppealSubmitted }: {
  examID: string;
  releaseID: string;
  releaseVersion: number;
  appealWindow: StudentResult["appeal_window"];
  questions: StudentQuestion[];
  onAppealSubmitted: () => void;
}) {
  const [expanded, setExpanded] = useState<string | null>(null);
  const [details, setDetails] = useState<Record<string, StudentQuestion>>({});
  const [annotations, setAnnotations] = useState<Record<string, StudentQuestionAnnotation[]>>({});
  const [loadingID, setLoadingID] = useState<string | null>(null);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const sorted = useMemo(() => [...questions].sort((left, right) => left.question_no.localeCompare(right.question_no, "zh-CN", { numeric: true })), [questions]);

  const toggle = async (question: StudentQuestion) => {
    if (expanded === question.question_id) { setExpanded(null); return; }
    setExpanded(question.question_id);
    if (details[question.question_id] || loadingID === question.question_id) return;
    setLoadingID(question.question_id);
    setErrors((current) => ({ ...current, [question.question_id]: "" }));
    try {
      const detailResponse = await getQuestion(examID, question.question_id);
      setDetails((current) => ({ ...current, [question.question_id]: detailResponse.question }));
      try {
        const annotationResponse = await listQuestionAnnotations(examID, question.question_id);
        setAnnotations((current) => ({ ...current, [question.question_id]: annotationResponse.annotations }));
      } catch {
        // Annotation availability must not hide an already-published question result.
        setAnnotations((current) => ({ ...current, [question.question_id]: [] }));
      }
    } catch (reason) {
      setErrors((current) => ({ ...current, [question.question_id]: friendlyError(reason) }));
    } finally {
      setLoadingID(null);
    }
  };

  return <div className="question-list">{sorted.map((question) => {
    const open = expanded === question.question_id;
    const detail = details[question.question_id] ?? question;
    const publicAnnotations = annotations[question.question_id] ?? [];
    return <article className={`question-row ${open ? "open" : ""}`} key={question.question_id}>
      <button type="button" className="question-toggle" onClick={() => void toggle(question)} aria-expanded={open}>
        <span className="question-number">{question.question_no}</span>
        <span className="question-score">{formatScore(question.score)} <small>/ {formatScore(question.max_score)} 分</small></span>
        <span className="chevron">{open ? "⌃" : "⌄"}</span>
      </button>
      {open ? <div className="question-detail">
        {loadingID === question.question_id ? <p className="muted">正在加载本题反馈…</p> : null}
        {errors[question.question_id] ? <p className="detail-error">{errors[question.question_id]}</p> : null}
        {!loadingID && !errors[question.question_id] ? <>
          {detail.feedback ? <div><h3>教师反馈</h3><p>{detail.feedback}</p></div> : null}
          {detail.rubric_summary && detail.rubric_summary.length > 0 ? <div><h3>评分要点</h3><ul>{detail.rubric_summary.map((item) => <li key={item}>{item}</li>)}</ul></div> : null}
          {publicAnnotations.length > 0 ? <div className="question-annotations"><h3>教师批注</h3><ul>{publicAnnotations.map((annotation) => <li key={annotation.id}><span className="annotation-type">{annotationTypeLabel(annotation.type)}</span><span>{annotation.content || "教师标注"}</span></li>)}</ul></div> : null}
          {!detail.feedback && !(detail.rubric_summary?.length) ? <p className="muted">本题未公开文字反馈。</p> : null}
          {appealWindow.open ? <QuestionAppealForm examID={examID} releaseID={releaseID} releaseVersion={releaseVersion} question={question} allowedReasonCodes={appealWindow.allowed_reason_codes ?? []} onSubmitted={onAppealSubmitted} /> : null}
        </> : null}
      </div> : null}
    </article>;
  })}</div>;
}

function annotationTypeLabel(type: StudentQuestionAnnotation["type"]) {
  switch (type) {
    case "highlight": return "重点标注";
    case "rectangle": return "圈画标注";
    case "freehand": return "手写标注";
    default: return "教师批注";
  }
}

const appealReasonLabels: Record<string, string> = {
  recognition_error: "答题内容识别不完整",
  missing_step_credit: "作答步骤疑似漏评",
  rubric_disagreement: "评分标准适用有异议",
  calculation_error: "得分记录或合计有误",
  annotation_issue: "批注与实际扣分不一致",
  other: "其他明确评分问题"
};

function QuestionAppealForm({ examID, releaseID, releaseVersion, question, allowedReasonCodes, onSubmitted }: {
  examID: string;
  releaseID: string;
  releaseVersion: number;
  question: StudentQuestion;
  allowedReasonCodes: string[];
  onSubmitted: () => void;
}) {
  const availableReasons = allowedReasonCodes.filter((code) => appealReasonLabels[code]);
  const [reasonCode, setReasonCode] = useState(availableReasons[0] ?? "");
  const [reason, setReason] = useState("");
  const [selectedRegion, setSelectedRegion] = useState<SelectedAppealRegion | undefined>();
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState("");
  const [existingAppeal, setExistingAppeal] = useState<StudentQuestionAppeal | null>(null);

  useEffect(() => {
    let active = true;
    void listQuestionAppeals(examID).then((response) => {
      if (active) setExistingAppeal(response.appeals.find((appeal) => appeal.source_release_id === releaseID && appeal.question_id === question.question_id) ?? null);
    }).catch(() => undefined);
    return () => { active = false; };
  }, [examID, question.question_id, releaseID]);

  if (existingAppeal) {
    return <div className="question-appeal-confirmation" role="status"><strong>{appealStatusLabel(existingAppeal.status)}</strong><span>本题已提交过复核申请，不能重复提交。处理结果会显示在页面下方。</span></div>;
  }
  if (availableReasons.length === 0) {
    return null;
  }
  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setSubmitting(true);
    setError("");
    try {
      const response = await createQuestionAppeal(examID, {
        source_release_id: releaseID,
        question_id: question.question_id,
        reason_code: reasonCode,
        reason: reason.trim(),
        ...(selectedRegion ? { selected_region: selectedRegion } : {})
      });
      setExistingAppeal(response.appeal);
      onSubmitted();
    } catch (failure) {
      setError(friendlyError(failure));
    } finally {
      setSubmitting(false);
    }
  };
  return <form className="question-appeal-form" onSubmit={submit}>
    <h3>申请复核</h3>
    <p>每道题只能提交一次。学校将按第 {releaseVersion} 版成绩核对原卷和评分标准。</p>
    <AppealRegionSelector
      imageURL={studentQuestionAnswerImageURL(examID, question.question_id)}
      value={selectedRegion}
      onChange={setSelectedRegion}
    />
    <label>复核原因<select value={reasonCode} onChange={(event) => setReasonCode(event.target.value)}>{availableReasons.map((code) => <option key={code} value={code}>{appealReasonLabels[code]}</option>)}</select></label>
    <label>具体说明<textarea required minLength={10} maxLength={500} value={reason} onChange={(event) => setReason(event.target.value)} placeholder="请具体说明哪一步、哪一评分点或哪段识别内容需要复核（10—500字）" /></label>
    {error ? <p className="detail-error" role="alert">{error}</p> : null}
    <button className="secondary-button" type="submit" disabled={submitting || reason.trim().length < 10}>{submitting ? "正在提交…" : "确认提交（仅一次）"}</button>
  </form>;
}

function AppealRegionSelector({ imageURL, value, onChange }: {
  imageURL: string;
  value?: SelectedAppealRegion;
  onChange: (value: SelectedAppealRegion | undefined) => void;
}) {
  const [imageState, setImageState] = useState<"loading" | "ready" | "unavailable">("loading");
  const [start, setStart] = useState<{ x: number; y: number } | null>(null);

  const point = (event: PointerEvent<HTMLDivElement>) => {
    const bounds = event.currentTarget.getBoundingClientRect();
    return {
      x: Math.max(0, Math.min(1, (event.clientX - bounds.left) / bounds.width)),
      y: Math.max(0, Math.min(1, (event.clientY - bounds.top) / bounds.height))
    };
  };
  const regionFrom = (origin: { x: number; y: number }, end: { x: number; y: number }): SelectedAppealRegion => ({
    coordinate_space: "canonical_image_normalized",
    x: Math.min(origin.x, end.x), y: Math.min(origin.y, end.y),
    width: Math.abs(end.x - origin.x), height: Math.abs(end.y - origin.y)
  });
  const begin = (event: PointerEvent<HTMLDivElement>) => {
    if (imageState !== "ready") return;
    event.currentTarget.setPointerCapture(event.pointerId);
    setStart(point(event));
  };
  const finish = (event: PointerEvent<HTMLDivElement>) => {
    if (!start) return;
    const region = regionFrom(start, point(event));
    setStart(null);
    if (region.width >= 0.01 && region.height >= 0.01) onChange(region);
  };
  return <section className="appeal-region-selector" aria-labelledby="appeal-region-title">
    <div><h4 id="appeal-region-title">圈选争议区域（可选）</h4><p>在本题答题图上拖动框选，帮助老师快速定位；未圈选也可以提交。</p></div>
    <div className="appeal-region-image-wrap">
      <img src={imageURL} alt="本题答题区域" onLoad={() => setImageState("ready")} onError={() => setImageState("unavailable")} />
      {imageState === "ready" ? <div
        className="appeal-region-canvas"
        onPointerDown={begin}
        onPointerUp={finish}
        onPointerCancel={() => setStart(null)}
        aria-label="在答题图上框选争议区域"
        role="application"
      >
        {value ? <span className="appeal-region-box" style={{ left: `${value.x * 100}%`, top: `${value.y * 100}%`, width: `${value.width * 100}%`, height: `${value.height * 100}%` }} /> : null}
      </div> : null}
      {imageState === "loading" ? <span className="appeal-region-loading">正在加载本题答题区域…</span> : null}
    </div>
    {imageState === "unavailable" ? <p className="detail-error">本题答题图暂时不可用，仍可通过文字说明提交复核。</p> : null}
    {value ? <div className="appeal-region-summary"><span>已圈选争议区域</span><button type="button" className="text-button" onClick={() => onChange(undefined)}>清除</button></div> : null}
  </section>;
}

function AppealWindow({ result }: { result: StudentResult }) {
  const windowState = result.appeal_window;
  if (windowState.open) {
    return <section className="appeal-notice open"><div><h2>成绩复核</h2><p>本次成绩的复核窗口已开启{windowState.closes_at ? `，截止至 ${formatDateTime(windowState.closes_at)}` : ""}。请在需要复核的题目下提交申请。</p></div><span>锚定第 {result.release_version} 版</span></section>;
  }
  return <section className="appeal-notice"><div><h2>成绩复核</h2><p>{windowState.closes_at ? `本次成绩的复核窗口已于 ${formatDateTime(windowState.closes_at)} 结束。` : "学校未开放本次成绩的复核窗口。"}</p></div></section>;
}

function AppealStatus({ examID, revision }: { examID: string; revision: number }) {
  const [appeals, setAppeals] = useState<StudentQuestionAppeal[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const load = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      const response = await listQuestionAppeals(examID);
      setAppeals(response.appeals);
    } catch (failure) {
      setError(friendlyError(failure));
    } finally {
      setLoading(false);
    }
  }, [examID]);
  useEffect(() => { void load(); }, [load, revision]);
  if (loading) return null;
  if (error) return <section className="appeal-status"><h2>我的复核申请</h2><p className="detail-error">申请记录暂时无法加载。<button type="button" className="text-button" onClick={() => void load()}>重试</button></p></section>;
  if (appeals.length === 0) return null;
  return <section className="appeal-status" aria-labelledby="appeal-status-title"><header><h2 id="appeal-status-title">我的复核申请</h2><button type="button" className="text-button" onClick={() => void load()}>刷新</button></header><div className="appeal-list">{appeals.map((appeal) => <article key={appeal.id}><div><strong>{appeal.question_no} · {appealReasonLabels[appeal.reason_code] ?? "复核申请"}</strong><p>{appealStatusLabel(appeal.status)} · 申请于 {formatDateTime(appeal.created_at)}</p>{appeal.public_response ? <p className="appeal-response">处理结果：{appeal.public_response}</p> : null}</div><span className={`appeal-status-tag ${appeal.status}`}>{appealStatusLabel(appeal.status)}</span></article>)}</div></section>;
}

function appealStatusLabel(status: string) {
  switch (status) {
    case "submitted": return "已提交";
    case "under_review": return "复核中";
    case "rejected": return "已答复";
    case "upheld_pending_regrade": return "正在重评";
    case "resolved": return "已处理";
    default: return "处理中";
  }
}

function EmptyState() {
  return <div className="empty-state">
    <p className="empty-state-kicker">当前状态</p>
    <h2>学校尚未发布你的成绩</h2>
    <p>成绩由学校正式发布后会自动出现在这里，不需要重复提交或刷新。</p>
    <ul>
      <li>如果老师还未通知发布，请等待学校完成阅卷与成绩确认。</li>
      <li>如果已经收到发布通知但仍看不到，请联系学校管理员核对学号绑定。</li>
    </ul>
  </div>;
}

function ErrorNotice({ message, onRetry, onBack }: { message: string; onRetry: () => void; onBack?: () => void }) {
  return <div className="error-notice" role="alert"><strong>暂时无法显示内容</strong><p>{message}</p><div><button className="secondary-button" type="button" onClick={onRetry}>重试</button>{onBack ? <button className="text-button" type="button" onClick={onBack}>返回</button> : null}</div></div>;
}

function friendlyError(reason: unknown) {
  if (reason instanceof PortalApiError) {
    if (reason.status === 401) return "登录状态已失效，请重新登录。";
    if (reason.status === 403 && reason.code === "student_score_scope_required") return "学生账号尚未关联本人学籍，请联系学校管理员核对账号与学号。";
    if (reason.status === 403) return "你无权查看此内容。";
    if (reason.status === 404) return "学校尚未发布这场考试的成绩。";
  }
  return "暂时无法连接服务，请稍后重试。";
}

function formatScore(score: number) { return Number.isInteger(score) ? String(score) : score.toFixed(1); }
function formatDate(value: string) { return new Intl.DateTimeFormat("zh-CN", { year: "numeric", month: "long", day: "numeric" }).format(new Date(value)); }
function formatDateTime(value: string) { return new Intl.DateTimeFormat("zh-CN", { year: "numeric", month: "numeric", day: "numeric", hour: "2-digit", minute: "2-digit", hour12: false }).format(new Date(value)); }
