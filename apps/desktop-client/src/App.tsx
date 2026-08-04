import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  Alert,
  Button,
  ConfigProvider,
  Empty,
  Form,
  Input,
  InputNumber,
  Progress,
  Select,
  Space,
  Table,
  Tag,
  Tooltip,
  type TableColumnsType
} from "antd";
import {
  BookOpenCheck,
  CloudUpload,
  FileUp,
  HardDrive,
  ListChecks,
  LogIn,
  RefreshCw,
  RotateCcw,
  ScrollText,
  ServerCog,
  Settings,
  ShieldAlert,
  Stethoscope,
  UploadCloud,
  Wifi
} from "lucide-react";
import { motion } from "framer-motion";
import { getCurrentUser, login } from "./api/auth";
import { DesktopApiClient } from "./api/client";
import { listExams } from "./api/exams";
import { uploadFileWithProgress } from "./api/files";
import { listReviewTasks } from "./api/review";
import { addSubmissionPage, runSubmissionQualityCheck } from "./api/submissions";
import { OfflineWorkbench } from "./components/OfflineWorkbench";
import {
  appendLocalLog,
  clearLocalLogs,
  getCapabilityStatuses,
  getRuntimeDiagnostics,
  scanLocalCacheSecurity,
  readLocalLogs
} from "./lib/localRuntime";
import { readOfflineDraftEnvelopes } from "./lib/offlineStore";
import type {
  AuthUser,
  CapabilityProbe,
  Exam,
  LocalLogEntry,
  LocalCacheSecurityStatus,
  ReviewTask,
  RuntimeDiagnostics,
  ScanQualityCheck,
  SubmissionQualityResult,
  SyncQueueItem,
  SystemStatus,
  WorkspaceKey
} from "./types";

const defaultServer = "http://127.0.0.1:8080";
const scanQueueStorageKey = "edugrade.desktop.scan_queue";
const maxUploadBytes = 104857600;
const allowedUploadExtensions = [".pdf", ".png", ".jpg", ".jpeg"];

const navItems: { key: WorkspaceKey; label: string; icon: React.ReactNode }[] = [
  { key: "connect", label: "连接登录", icon: <LogIn size={18} /> },
  { key: "tasks", label: "任务列表", icon: <ListChecks size={18} /> },
  { key: "scan", label: "扫描上传", icon: <FileUp size={18} /> },
  { key: "offline", label: "离线阅卷", icon: <BookOpenCheck size={18} /> },
  { key: "sync", label: "同步队列", icon: <CloudUpload size={18} /> },
  { key: "diagnostics", label: "系统诊断", icon: <Stethoscope size={18} /> },
  { key: "logs", label: "本地日志", icon: <ScrollText size={18} /> }
];

const sourceLabels: Record<string, string> = {
  ai_low_confidence: "AI 低置信",
  ocr_low_confidence: "OCR 低置信",
  subjective_default_review: "主观题复核",
  evidence_verification_failed: "证据校验失败",
  double_mark_required: "双评任务",
  score_anomaly: "分数异常",
  manual_sample: "人工抽检"
};

const queueStatusLabels: Record<SyncQueueItem["status"], string> = {
  pending: "pending",
  uploading: "uploading",
  succeeded: "succeeded",
  failed: "failed",
  not_configured: "未配置/待接入"
};

function App() {
  const [workspace, setWorkspace] = useState<WorkspaceKey>("connect");
  const [serverUrl, setServerUrl] = useState(() => window.sessionStorage.getItem("edugrade.desktop.server_url") ?? defaultServer);
  const [tenantCode, setTenantCode] = useState("demo");
  const [username, setUsername] = useState("admin");
  const [password, setPassword] = useState("");
  const [token, setToken] = useState<string | null>(null);
  const [expiresAt, setExpiresAt] = useState<string | null>(null);
  const [user, setUser] = useState<AuthUser | null>(null);
  const [authError, setAuthError] = useState<string | null>(null);
  const [isLoggingIn, setIsLoggingIn] = useState(false);
  const [tasks, setTasks] = useState<ReviewTask[]>([]);
  const [taskError, setTaskError] = useState<string | null>(null);
  const [isLoadingTasks, setIsLoadingTasks] = useState(false);
  const [capabilities, setCapabilities] = useState<CapabilityProbe[]>([]);
  const [diagnostics, setDiagnostics] = useState<RuntimeDiagnostics | null>(null);
  const [localCacheSecurity, setLocalCacheSecurity] = useState<LocalCacheSecurityStatus>(() => scanLocalCacheSecurity());
  const [serviceStatus, setServiceStatus] = useState<SystemStatus | null>(null);
  const [isCheckingServiceStatus, setIsCheckingServiceStatus] = useState(false);
  const [diagnosticError, setDiagnosticError] = useState<string | null>(null);
  const [queue, setQueue] = useState<SyncQueueItem[]>(() => readPersistedScanQueue());
  const [logs, setLogs] = useState<LocalLogEntry[]>(() => readLocalLogs());
  const [exams, setExams] = useState<Exam[]>([]);
  const [selectedExamId, setSelectedExamId] = useState("");
  const [examError, setExamError] = useState<string | null>(null);
  const [isLoadingExams, setIsLoadingExams] = useState(false);
  const [scanSubmissionId, setScanSubmissionId] = useState("");
  const [scanStartPage, setScanStartPage] = useState(1);
  const [isOnline, setIsOnline] = useState(() => navigator.onLine);
  const [qualityResult, setQualityResult] = useState<SubmissionQualityResult | null>(null);
  const [qualityError, setQualityError] = useState<string | null>(null);
  const [isCheckingQuality, setIsCheckingQuality] = useState(false);
  const fileInputRef = useRef<HTMLInputElement | null>(null);
  const fileBufferRef = useRef(new Map<string, File>());
  const uploadInFlightRef = useRef(new Set<string>());
  const queueRef = useRef(queue);

  const client = useMemo(
    () =>
      new DesktopApiClient({
        baseUrl: serverUrl,
        getToken: () => token
      }),
    [serverUrl, token]
  );

  const updateQueue = useCallback((updater: (current: SyncQueueItem[]) => SyncQueueItem[]) => {
    setQueue((current) => {
      const next = updater(current);
      queueRef.current = next;
      persistScanQueue(next);
      return next;
    });
  }, []);

  const refreshLogs = useCallback(() => setLogs(readLocalLogs()), []);

  const logEvent = useCallback(
    async (level: LocalLogEntry["level"], message: string, context?: string) => {
      try {
        await appendLocalLog({ level, message, context });
      } catch (error) {
        console.warn("local log write failed", error);
      } finally {
        refreshLogs();
      }
    },
    [refreshLogs]
  );

  const refreshCapabilities = useCallback(async () => {
    const [nextCapabilities, nextDiagnostics] = await Promise.all([getCapabilityStatuses(), getRuntimeDiagnostics()]);
    setCapabilities(nextCapabilities);
    setDiagnostics(nextDiagnostics);
    setLocalCacheSecurity(scanLocalCacheSecurity());
  }, []);

  useEffect(() => {
    refreshCapabilities();
    void logEvent("info", "desktop client shell loaded", "STORY-034");
  }, [logEvent, refreshCapabilities]);

  useEffect(() => {
    queueRef.current = queue;
  }, [queue]);

  useEffect(() => () => {
    for (const item of queueRef.current) {
      if (item.previewUrl) URL.revokeObjectURL(item.previewUrl);
    }
    fileBufferRef.current.clear();
    uploadInFlightRef.current.clear();
  }, []);

  const saveServerForSession = async () => {
    window.sessionStorage.setItem("edugrade.desktop.server_url", serverUrl);
    await logEvent("info", "server url saved for current session", serverUrl);
  };

  const handleLogin = async () => {
    setAuthError(null);
    setIsLoggingIn(true);
    try {
      const result = await login(new DesktopApiClient({ baseUrl: serverUrl }), {
        tenant_code: tenantCode.trim(),
        username: username.trim(),
        password
      });
      setToken(result.access_token);
      setExpiresAt(result.expires_at);
      setUser(result.user);
      setPassword("");
      await logEvent("info", "login succeeded", `${result.user.username}@${result.user.tenant_code}`);
    } catch (error) {
      const message = error instanceof Error ? error.message : "登录请求失败";
      setAuthError(message);
      await logEvent("error", "login failed", message);
    } finally {
      setIsLoggingIn(false);
    }
  };

  const handleCheckSession = async () => {
    setAuthError(null);
    try {
      const result = await getCurrentUser(client);
      setUser(result.user);
      await logEvent("info", "session verified", result.user.username);
    } catch (error) {
      const message = error instanceof Error ? error.message : "Session 校验失败";
      setAuthError(message);
      await logEvent("warning", "session verification failed", message);
    }
  };

  const handleLoadTasks = async () => {
    setTaskError(null);
    setIsLoadingTasks(true);
    try {
      const result = await listReviewTasks(client, {});
      setTasks(result.tasks);
      await logEvent("info", "review tasks loaded", `${result.tasks.length} tasks`);
    } catch (error) {
      const message = error instanceof Error ? error.message : "任务列表读取失败";
      setTaskError(message);
      setTasks([]);
      await logEvent("warning", "review task load failed", message);
    } finally {
      setIsLoadingTasks(false);
    }
  };

  const handleHealthCheck = async () => {
    setDiagnosticError(null);
    try {
      const result = await client.health();
      await logEvent("info", "server health checked", JSON.stringify(result));
    } catch (error) {
      const message = error instanceof Error ? error.message : "服务端健康检查失败";
      setDiagnosticError(message);
      await logEvent("warning", "server health check failed", message);
    }
  };

  const handleSystemStatusCheck = async () => {
    setDiagnosticError(null);
    setIsCheckingServiceStatus(true);
    try {
      const result = await client.systemStatus();
      setServiceStatus(result);
      await logEvent("info", "server system status checked", result.status);
    } catch (error) {
      const message = error instanceof Error ? error.message : "系统状态检查失败";
      setDiagnosticError(message);
      await logEvent("warning", "server system status check failed", message);
    } finally {
      setIsCheckingServiceStatus(false);
    }
  };

  const handleLoadExams = async () => {
    setExamError(null);
    setIsLoadingExams(true);
    try {
      const result = await listExams(client, {});
      setExams(result.exams);
      if (!selectedExamId && result.exams[0]) {
        setSelectedExamId(result.exams[0].id);
      }
      await logEvent("info", "exam list loaded for scan workstation", `${result.exams.length} exams`);
    } catch (error) {
      const message = error instanceof Error ? error.message : "考试列表读取失败";
      setExamError(message);
      setExams([]);
      await logEvent("warning", "exam list load failed", message);
    } finally {
      setIsLoadingExams(false);
    }
  };

  const handleFileSelection = async (files: FileList | null) => {
    const selected = files ? Array.from(files) : [];
    if (!selected.length) {
      return;
    }
    const selectedExam = exams.find((exam) => exam.id === selectedExamId);
    const submissionId = scanSubmissionId.trim();
    const startPage = scanStartPage || 1;
    const nextItems: SyncQueueItem[] = [];
    for (const [index, file] of selected.entries()) {
      const qualityChecks = await inspectScanFile(file);
      const failedQuality = qualityChecks.some((check) => check.status === "failed");
      const recovered = queueRef.current.find(
        (item) => item.requiresReselect && item.fileName === file.name && item.fileSize === file.size && item.kind === "scan_upload"
      );
      if (recovered) {
        if (recovered.previewUrl) URL.revokeObjectURL(recovered.previewUrl);
        fileBufferRef.current.set(recovered.id, file);
        updateQueue((current) =>
          current.map((item) =>
            item.id === recovered.id
              ? {
                  ...item,
                  status: failedQuality ? "failed" : "pending",
                  progress: 0,
                  detail: failedQuality ? "本地质量检查未通过" : "文件句柄已恢复，等待上传",
                  requiresReselect: false,
                  qualityChecks,
                  previewUrl: previewUrlForFile(file),
                  updatedAt: new Date().toISOString()
                }
              : item
          )
        );
        continue;
      }
      const id = crypto.randomUUID();
      fileBufferRef.current.set(id, file);
      nextItems.push({
        id,
        title: file.name,
        kind: "scan_upload",
        status: failedQuality ? "failed" : "pending",
        progress: 0,
        detail: failedQuality ? "本地质量检查未通过" : "已通过本地基础检查，等待上传",
        updatedAt: new Date().toISOString(),
        examId: selectedExamId || undefined,
        examName: selectedExam?.name,
        submissionId: submissionId || undefined,
        pageNo: startPage + index,
        fileName: file.name,
        fileSize: file.size,
        contentType: file.type || inferContentType(file.name),
        previewUrl: previewUrlForFile(file),
        requiresReselect: false,
        qualityChecks
      });
    }
    if (nextItems.length) {
      updateQueue((current) => [...nextItems, ...current]);
    }
    await logEvent("info", "scan files queued", `${selected.length} files`);
  };

  const uploadQueueItem = useCallback(
    async (id: string) => {
      const item = queueRef.current.find((candidate) => candidate.id === id);
      if (
        !item ||
        item.kind !== "scan_upload" ||
        item.status === "succeeded" ||
        item.status === "uploading" ||
        uploadInFlightRef.current.has(id)
      ) {
        return;
      }
      if (!token) {
        updateQueue((current) =>
          current.map((candidate) =>
            candidate.id === id ? { ...candidate, status: "failed", detail: "未登录，无法通过后端鉴权上传", updatedAt: new Date().toISOString() } : candidate
          )
        );
        return;
      }
      if (!isOnline) {
        updateQueue((current) =>
          current.map((candidate) =>
            candidate.id === id ? { ...candidate, status: "pending", detail: "当前离线，等待联网后继续上传", updatedAt: new Date().toISOString() } : candidate
          )
        );
        return;
      }
      if (!item.submissionId || !item.pageNo || !item.examId) {
        updateQueue((current) =>
          current.map((candidate) =>
            candidate.id === id ? { ...candidate, status: "failed", detail: "缺少考试、submission 或页码上下文", updatedAt: new Date().toISOString() } : candidate
          )
        );
        return;
      }
      if (item.qualityChecks?.some((check) => check.status === "failed")) {
        updateQueue((current) =>
          current.map((candidate) =>
            candidate.id === id ? { ...candidate, status: "failed", detail: "本地质量检查未通过，未上传", updatedAt: new Date().toISOString() } : candidate
          )
        );
        return;
      }
      const file = fileBufferRef.current.get(id);
      if (!file && !item.fileAssetId) {
        updateQueue((current) =>
          current.map((candidate) =>
            candidate.id === id
              ? {
                  ...candidate,
                  status: "failed",
                  requiresReselect: true,
                  detail: "本地队列已恢复，但文件句柄不可用；请重新选择同名文件后重试",
                  updatedAt: new Date().toISOString()
                }
              : candidate
          )
        );
        return;
      }
      uploadInFlightRef.current.add(id);
      updateQueue((current) =>
        current.map((candidate) =>
          candidate.id === id ? { ...candidate, status: "uploading", progress: Math.max(candidate.progress, 1), detail: "正在上传到后端文件 API", updatedAt: new Date().toISOString() } : candidate
        )
      );
      try {
        let fileAssetId = item.fileAssetId;
        let serverStatus = item.serverStatus;
        if (!fileAssetId && file) {
          const result = await uploadFileWithProgress(
            client,
            file,
            {
              owner_type: "answer_page",
              owner_id: item.submissionId,
              submission_id: item.submissionId,
              exam_id: item.examId
            },
            (progress) => {
              updateQueue((current) => current.map((candidate) => (candidate.id === id ? { ...candidate, progress, updatedAt: new Date().toISOString() } : candidate)));
            }
          );
          fileAssetId = result.file.id;
          serverStatus = `file_asset 已登记：${result.file.id}`;
          updateQueue((current) =>
            current.map((candidate) =>
              candidate.id === id
                ? {
                    ...candidate,
                    fileAssetId,
                    serverStatus,
                    detail: "文件已上传，正在关联 submission page",
                    updatedAt: new Date().toISOString()
                  }
                : candidate
            )
          );
        }
        const page = await addSubmissionPage(client, item.submissionId, {
          file_asset_id: fileAssetId!,
          page_no: item.pageNo
        });
        updateQueue((current) =>
          current.map((candidate) =>
            candidate.id === id
              ? {
                  ...candidate,
                  status: "succeeded",
                  progress: 100,
                  fileAssetId,
                  serverStatus: `submission page ${page.page.page_no} 已关联，状态 ${page.page.status}`,
                  detail: "服务端已接收文件并关联答卷页",
                  updatedAt: new Date().toISOString()
                }
              : candidate
          )
        );
        fileBufferRef.current.delete(id);
        await logEvent("info", "scan queue item uploaded", `${item.fileName ?? item.title} -> ${fileAssetId}`);
      } catch (error) {
        const message = error instanceof Error ? error.message : "上传失败";
        updateQueue((current) =>
          current.map((candidate) =>
            candidate.id === id
              ? {
                  ...candidate,
                  status: "failed",
                  detail: message,
                  updatedAt: new Date().toISOString()
                }
              : candidate
          )
        );
        await logEvent("error", "scan queue item failed", `${item.fileName ?? item.title}: ${message}`);
      } finally {
        uploadInFlightRef.current.delete(id);
      }
    },
    [client, isOnline, logEvent, token, updateQueue]
  );

  const uploadQueueItems = useCallback(
    async (mode: "pending" | "failed" | "all") => {
      const candidates = queueRef.current.filter((item) => {
        if (item.kind !== "scan_upload") {
          return false;
        }
        if (mode === "all") {
          return item.status === "pending" || item.status === "failed";
        }
        return item.status === mode;
      });
      for (const item of candidates) {
        await uploadQueueItem(item.id);
      }
    },
    [uploadQueueItem]
  );

  useEffect(() => {
    const handleOnline = () => {
      setIsOnline(true);
      void logEvent("info", "network online", "scan queue auto resume");
      void uploadQueueItems("all");
    };
    const handleOffline = () => {
      setIsOnline(false);
      void logEvent("warning", "network offline", "scan upload paused");
    };
    window.addEventListener("online", handleOnline);
    window.addEventListener("offline", handleOffline);
    return () => {
      window.removeEventListener("online", handleOnline);
      window.removeEventListener("offline", handleOffline);
    };
  }, [logEvent, uploadQueueItems]);

  const handleRunQualityCheck = async () => {
    const submissionId = scanSubmissionId.trim();
    if (!submissionId) {
      setQualityError("缺少 submission_id，无法运行服务端质量门禁");
      return;
    }
    setQualityError(null);
    setIsCheckingQuality(true);
    try {
      const result = await runSubmissionQualityCheck(client, submissionId);
      setQualityResult(result.result);
      await logEvent("info", "submission quality check completed", JSON.stringify(result.result));
    } catch (error) {
      const message = error instanceof Error ? error.message : "服务端质量门禁失败";
      setQualityError(message);
      await logEvent("warning", "submission quality check failed", message);
    } finally {
      setIsCheckingQuality(false);
    }
  };

  const clearSucceededQueueItems = () => {
    updateQueue((current) => {
      for (const item of current) {
        if (item.status !== "succeeded") continue;
        if (item.previewUrl) URL.revokeObjectURL(item.previewUrl);
        fileBufferRef.current.delete(item.id);
      }
      return current.filter((item) => item.status !== "succeeded");
    });
  };

  const activeCapabilityWarnings = capabilities.filter((item) => item.status === "not_configured");

  return (
    <ConfigProvider
      theme={{
        token: {
          colorPrimary: "#1677ff",
          colorSuccess: "#52c41a",
          colorWarning: "#faad14",
          colorError: "#ff4d4f",
          colorInfo: "#13c2c2",
          borderRadius: 6,
          fontFamily: 'Inter, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif'
        }
      }}
    >
      <div className="desktop-shell">
        <aside className="desktop-sidebar">
          <div className="desktop-brand">
            <div className="desktop-brand-mark">E</div>
            <div>
              <h1>EduGrade EXE</h1>
              <p>扫描与离线工作站</p>
            </div>
          </div>
          <nav className="desktop-nav" aria-label="Desktop workspace">
            {navItems.map((item) => (
              <button
                className={workspace === item.key ? "active" : ""}
                key={item.key}
                type="button"
                onClick={() => setWorkspace(item.key)}
              >
                {item.icon}
                <span>{item.label}</span>
              </button>
            ))}
          </nav>
          <div className="desktop-sidebar-footer">
            <StatusLine label="后端" value={serverUrl} tone={token ? "ready" : "idle"} />
            <StatusLine label="安全存储" value="未配置/待接入" tone="warning" />
          </div>
        </aside>

        <main className="desktop-main">
          <header className="desktop-topbar">
            <div>
              <p className="eyebrow">Windows EXE Client Skeleton</p>
              <h2>{navItems.find((item) => item.key === workspace)?.label}</h2>
            </div>
            <div className="topbar-actions">
              <Tag color={token ? "success" : "default"}>{token ? "已登录" : "未登录"}</Tag>
              <Tag color={activeCapabilityWarnings.length ? "warning" : "success"}>
                {activeCapabilityWarnings.length ? "存在待接入能力" : "本地能力就绪"}
              </Tag>
              <Tooltip title="刷新本地能力与运行时诊断">
                <Button icon={<RefreshCw size={16} />} onClick={refreshCapabilities} />
              </Tooltip>
            </div>
          </header>

          <motion.div
            key={workspace}
            initial={{ opacity: 0, y: 8 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.18 }}
            className="workspace"
          >
            {workspace === "connect" && renderConnect()}
            {workspace === "tasks" && renderTasks()}
            {workspace === "scan" && renderScan()}
            {workspace === "offline" && renderOffline()}
            {workspace === "sync" && renderSync()}
            {workspace === "diagnostics" && renderDiagnostics()}
            {workspace === "logs" && renderLogs()}
          </motion.div>
        </main>
      </div>
    </ConfigProvider>
  );

  function renderConnect() {
    return (
      <div className="workspace-grid two">
        <section className="panel">
          <SectionHead icon={<ServerCog size={20} />} title="服务端地址配置" description="当前 Story 不启用安全落盘；地址只保存到本次会话。" />
          <Form layout="vertical">
            <Form.Item label="API 服务端地址">
              <Input value={serverUrl} onChange={(event) => setServerUrl(event.target.value)} placeholder={defaultServer} />
            </Form.Item>
            <Space wrap>
              <Button type="primary" icon={<Settings size={16} />} onClick={saveServerForSession}>
                保存到当前会话
              </Button>
              <Button icon={<Wifi size={16} />} onClick={handleHealthCheck}>
                检查 /health
              </Button>
            </Space>
          </Form>
          {diagnosticError && <Alert className="section-alert" type="warning" message={diagnosticError} showIcon />}
        </section>

        <section className="panel">
          <SectionHead icon={<LogIn size={20} />} title="登录" description="调用桌面客户端专用 POST /api/v1/auth/token；token 仅保存在内存中。" />
          <Form layout="vertical">
            <Form.Item label="租户代码">
              <Input value={tenantCode} onChange={(event) => setTenantCode(event.target.value)} />
            </Form.Item>
            <Form.Item label="用户名">
              <Input value={username} onChange={(event) => setUsername(event.target.value)} />
            </Form.Item>
            <Form.Item label="密码">
              <Input.Password value={password} onChange={(event) => setPassword(event.target.value)} onPressEnter={handleLogin} />
            </Form.Item>
            <Space wrap>
              <Button type="primary" icon={<LogIn size={16} />} loading={isLoggingIn} onClick={handleLogin}>
                登录后端
              </Button>
              <Button icon={<ShieldAlert size={16} />} disabled={!token} onClick={handleCheckSession}>
                校验 session
              </Button>
            </Space>
          </Form>
          {authError && <Alert className="section-alert" type="error" message={authError} showIcon />}
          {user && (
            <div className="identity-strip">
              <span>{user.display_name || user.username}</span>
              <Tag color="blue">{user.tenant_code}</Tag>
              <Tag>{user.roles.join(", ") || "无角色"}</Tag>
              {expiresAt && <span className="muted">过期：{formatDate(expiresAt)}</span>}
            </div>
          )}
        </section>
      </div>
    );
  }

  function renderTasks() {
    const columns: TableColumnsType<ReviewTask> = [
      { title: "匿名号", dataIndex: "anonymous_code", width: 130 },
      { title: "题号", dataIndex: "question_no", width: 90 },
      {
        title: "来源",
        dataIndex: "source",
        render: (value: string) => sourceLabels[value] ?? value
      },
      {
        title: "状态",
        dataIndex: "status",
        width: 110,
        render: (value: string) => <Tag>{value}</Tag>
      },
      { title: "优先级", dataIndex: "priority", width: 90 },
      { title: "创建时间", dataIndex: "created_at", render: formatDate }
    ];
    return (
      <section className="panel full">
        <SectionHead
          icon={<ListChecks size={20} />}
          title="复核任务列表"
          description="调用真实 GET /api/v1/review-tasks；无权限或无 token 时显示后端错误，不填充假任务。"
          action={
            <Button icon={<RefreshCw size={16} />} loading={isLoadingTasks} disabled={!token} onClick={handleLoadTasks}>
              刷新任务
            </Button>
          }
        />
        {!token && <Alert type="warning" message="未登录：任务列表不会读取，也不会回退到 mock 数据。" showIcon />}
        {taskError && <Alert className="section-alert" type="error" message={taskError} showIcon />}
        <Table rowKey="id" size="middle" columns={columns} dataSource={tasks} loading={isLoadingTasks} scroll={{ x: 760 }} locale={{ emptyText: <Empty description="暂无真实任务" /> }} />
      </section>
    );
  }

  function renderScan() {
    const selectedExam = exams.find((exam) => exam.id === selectedExamId);
    const scanItems = queue.filter((item) => item.kind === "scan_upload");
    const readyCount = scanItems.filter((item) => item.status === "pending").length;
    const failedCount = scanItems.filter((item) => item.status === "failed").length;
    return (
      <div className="workspace-grid scan-workstation">
        <section className="panel full">
          <SectionHead
            icon={<UploadCloud size={20} />}
            title="扫描工作站"
            description="选择考试和 submission 后批量选择 PDF/图片；真实扫描仪驱动未配置/待接入。"
            action={<Tag color={isOnline ? "success" : "error"}>{isOnline ? "在线" : "离线，上传暂停"}</Tag>}
          />
          <Alert type="info" showIcon message="真实扫描仪：未配置/待接入。本页只处理文件选择上传，不模拟扫描仪。" />
          <div className="scan-toolbar">
            <Form layout="vertical" className="scan-context-form">
              <Form.Item label="考试">
                <Space.Compact block>
                  <Select
                    value={selectedExamId || undefined}
                    placeholder={token ? "选择真实考试" : "登录后加载考试"}
                    loading={isLoadingExams}
                    disabled={!token}
                    onChange={setSelectedExamId}
                    options={exams.map((exam) => ({
                      value: exam.id,
                      label: `${exam.name} / ${exam.subject} / ${exam.status}`
                    }))}
                  />
                  <Button icon={<RefreshCw size={16} />} disabled={!token} loading={isLoadingExams} onClick={handleLoadExams}>
                    刷新
                  </Button>
                </Space.Compact>
              </Form.Item>
              <Form.Item label="Submission ID">
                <Input value={scanSubmissionId} onChange={(event) => setScanSubmissionId(event.target.value)} placeholder="submission uuid" />
              </Form.Item>
              <Form.Item label="起始页码">
                <InputNumber min={1} value={scanStartPage} onChange={(value) => setScanStartPage(value ?? 1)} />
              </Form.Item>
            </Form>
            <div className="scan-actions">
              <input
                ref={fileInputRef}
                type="file"
                multiple
                accept=".pdf,.png,.jpg,.jpeg"
                onChange={(event) => {
                  void handleFileSelection(event.target.files);
                  event.target.value = "";
                }}
              />
              <Button icon={<FileUp size={16} />} disabled={!selectedExamId || !scanSubmissionId.trim()} onClick={() => fileInputRef.current?.click()}>
                批量选择 PDF/图片
              </Button>
              <Button type="primary" icon={<UploadCloud size={16} />} disabled={!token || !isOnline || readyCount === 0} onClick={() => void uploadQueueItems("pending")}>
                上传 pending
              </Button>
              <Button icon={<RotateCcw size={16} />} disabled={!token || !isOnline || failedCount === 0} onClick={() => void uploadQueueItems("failed")}>
                重试 failed
              </Button>
              <Button danger disabled={!scanItems.some((item) => item.status === "succeeded")} onClick={clearSucceededQueueItems}>
                清空 succeeded
              </Button>
            </div>
          </div>
          {examError && <Alert className="section-alert" type="error" message={examError} showIcon />}
          {!token && <Alert className="section-alert" type="warning" message="未登录：不会读取考试，也不会上传文件。" showIcon />}
          {!selectedExamId && token && <p className="muted">请先选择真实考试；客户端不会创建假考试上下文。</p>}
          {selectedExam && <p className="muted">当前考试：{selectedExam.name} / {selectedExam.subject} / {selectedExam.status}</p>}
        </section>

        <section className="panel full">
          <SectionHead icon={<ListChecks size={20} />} title="上传前预览与本地质量检查" description="文件进入队列后先展示检查结果；PDF 页数与复杂分辨率检测明确预留。" />
          <ScanQueueTable items={scanItems} onRetry={(id) => void uploadQueueItem(id)} />
        </section>

        <section className="panel full">
          <SectionHead
            icon={<CloudUpload size={20} />}
            title="服务端处理状态"
            description="文件上传后显示 file_asset 和 submission page 关联结果；质量门禁调用真实 submission API。"
            action={
              <Button icon={<RefreshCw size={16} />} loading={isCheckingQuality} disabled={!token || !scanSubmissionId.trim()} onClick={handleRunQualityCheck}>
                运行服务端质量门禁
              </Button>
            }
          />
          {qualityError && <Alert type="error" message={qualityError} showIcon />}
          {qualityResult ? (
            <div className="quality-result">
              <Tag color={qualityResult.valid ? "success" : "warning"}>{qualityResult.valid ? "valid" : "issues"}</Tag>
              {qualityResult.issues.length ? (
                qualityResult.issues.map((issue) => (
                  <p key={`${issue.code}-${issue.message}`}>
                    {issue.code}: {issue.message}
                  </p>
                ))
              ) : (
                <p>服务端质量门禁未返回问题。</p>
              )}
            </div>
          ) : (
            <Empty description="尚未运行服务端质量门禁" />
          )}
        </section>
      </div>
    );
  }

  function renderOffline() {
    return (
      <OfflineWorkbench client={client} token={token} user={user} isOnline={isOnline} onLog={logEvent} />
    );
  }

  function renderSync() {
    const offlinePlaceholder: SyncQueueItem = {
      id: "offline-sync-not-configured",
      title: "离线阅卷同步",
      kind: "offline_grade",
      status: "not_configured",
      progress: 0,
      detail: "未配置/待接入：离线草稿同步 API 尚未实现。",
      updatedAt: new Date().toISOString()
    };
    return (
      <section className="panel full">
        <SectionHead icon={<CloudUpload size={20} />} title="同步队列" description="显示真实上传操作和未接入离线同步能力；不伪造同步成功。" />
        <QueueList items={[...queue, offlinePlaceholder]} />
      </section>
    );
  }

  function renderDiagnostics() {
    const scanItems = queue.filter((item) => item.kind === "scan_upload");
    const offlineDrafts = readOfflineDraftEnvelopes();
    const recentErrors = logs.filter((entry) => entry.level === "error").slice(0, 5);
    const pendingUploads = scanItems.filter((item) => item.status === "pending" || item.status === "uploading").length;
    const failedUploads = scanItems.filter((item) => item.status === "failed").length;
    const localCacheBytes = estimateLocalCacheBytes();
    return (
      <div className="workspace-grid two">
        <section className="panel">
          <SectionHead
            icon={<Stethoscope size={20} />}
            title="运行时诊断"
            description="读取 Tauri 运行时信息；浏览器开发模式会明确标注。"
            action={
              <Button icon={<RefreshCw size={16} />} onClick={refreshCapabilities}>
                刷新
              </Button>
            }
          />
          {diagnostics ? (
            <div className="diagnostic-list">
              <StatusLine label="Runtime" value={diagnostics.runtime} tone="ready" />
              <StatusLine label="Platform" value={diagnostics.platform} tone="idle" />
              <StatusLine label="Version" value={diagnostics.appVersion} tone="idle" />
              <StatusLine label="Log path" value={diagnostics.logPath ?? "未返回"} tone="idle" />
            </div>
          ) : (
            <Empty description="诊断信息读取中" />
          )}
          <Button className="section-button" icon={<Wifi size={16} />} onClick={handleHealthCheck}>
            检查后端 /health
          </Button>
          {diagnosticError && <Alert className="section-alert" type="warning" message={diagnosticError} showIcon />}
        </section>

        <section className="panel">
          <SectionHead
            icon={<ServerCog size={20} />}
            title="服务端连接"
            description="调用真实 /api/v1/system/status；依赖未配置会显示未配置/待接入。"
            action={
              <Button icon={<RefreshCw size={16} />} loading={isCheckingServiceStatus} onClick={handleSystemStatusCheck}>
                检查状态
              </Button>
            }
          />
          <div className="diagnostic-list">
            <StatusLine label="Server" value={serverUrl} tone={serviceStatus ? (serviceStatus.status === "healthy" ? "ready" : "warning") : "idle"} />
            <StatusLine label="Status" value={serviceStatus?.status ?? "未检查"} tone={serviceStatus ? (serviceStatus.status === "healthy" ? "ready" : "warning") : "idle"} />
            <StatusLine label="Service" value={serviceStatus ? `${serviceStatus.service} / ${serviceStatus.environment}` : "未返回"} tone="idle" />
            <StatusLine label="Generated" value={serviceStatus ? formatDate(serviceStatus.generated_at) : "未返回"} tone="idle" />
          </div>
          {serviceStatus && (
            <div className="dependency-chip-list">
              {serviceStatus.dependencies.map((dependency) => (
                <Tooltip key={dependency.name} title={dependency.error ?? dependency.detail ?? `${dependency.duration_ms} ms`}>
                  <Tag color={dependencyTone(dependency.status)}>
                    {dependency.name}: {dependencyLabel(dependency.status)}
                  </Tag>
                </Tooltip>
              ))}
            </div>
          )}
        </section>

        <section className="panel">
          <SectionHead icon={<ShieldAlert size={20} />} title="本地能力状态" description="未接入能力必须明确显示，不作为真实可用能力。" />
          <div className="capability-list">
            {capabilities.map((capability) => (
              <div className="capability-row" key={capability.key}>
                <div>
                  <strong>{capability.name}</strong>
                  <p>{capability.detail}</p>
                </div>
                <CapabilityTag status={capability.status} />
              </div>
            ))}
          </div>
        </section>

        <section className="panel">
          <SectionHead icon={<HardDrive size={20} />} title="本地缓存与上传队列" description="基于当前客户端本地缓存和真实上传队列统计。" />
          <div className="diagnostic-summary-grid">
            <div>
              <span>缓存大小</span>
              <strong>{formatBytes(localCacheBytes)}</strong>
              <small>localStorage 估算</small>
            </div>
            <div>
              <span>离线草稿</span>
              <strong>{offlineDrafts.length}</strong>
              <small>加密草稿 envelope</small>
            </div>
            <div>
              <span>待上传</span>
              <strong>{pendingUploads}</strong>
              <small>pending/uploading</small>
            </div>
            <div>
              <span>失败队列</span>
              <strong>{failedUploads}</strong>
              <small>需人工处理</small>
            </div>
          </div>
          <div className="local-cache-security">
            <StatusLine
              label="缓存安全"
              value={localCacheSecurity.status === "passed" ? "未发现敏感缓存" : `${localCacheSecurity.issues.length} 个风险`}
              tone={localCacheSecurity.status === "passed" ? "ready" : "warning"}
            />
            <p>
              已扫描 {localCacheSecurity.scannedKeys} 个本地缓存键，最近检查 {formatDate(localCacheSecurity.checkedAt)}
            </p>
            {localCacheSecurity.issues.length ? (
              <div className="dependency-chip-list">
                {localCacheSecurity.issues.map((issue) => (
                  <Tooltip key={`${issue.key}-${issue.message}`} title={issue.message}>
                    <Tag color={issue.severity === "critical" ? "error" : "warning"}>{issue.key}</Tag>
                  </Tooltip>
                ))}
              </div>
            ) : null}
          </div>
        </section>

        <section className="panel full">
          <SectionHead icon={<ScrollText size={20} />} title="最近错误日志" description="只展示本客户端本地 error 级别日志；不是后端 audit_log。" />
          <div className="log-list">
            {recentErrors.length ? (
              recentErrors.map((entry) => (
                <div className="log-row" key={entry.id}>
                  <Tag color="error">{entry.level}</Tag>
                  <span>{formatDate(entry.at)}</span>
                  <strong>{entry.message}</strong>
                  {entry.context && <p>{entry.context}</p>}
                </div>
              ))
            ) : (
              <Empty description="暂无 error 日志" />
            )}
          </div>
        </section>
      </div>
    );
  }

  function renderLogs() {
    return (
      <section className="panel full">
        <SectionHead
          icon={<ScrollText size={20} />}
          title="本地日志"
          description="记录客户端侧关键操作；这不是后端 audit_log。"
          action={
            <Button
              danger
              onClick={() => {
                clearLocalLogs();
                refreshLogs();
              }}
            >
              清空本地日志
            </Button>
          }
        />
        <div className="log-list">
          {logs.length ? (
            logs.map((entry) => (
              <div className="log-row" key={entry.id}>
                <Tag color={entry.level === "error" ? "error" : entry.level === "warning" ? "warning" : "blue"}>{entry.level}</Tag>
                <span>{formatDate(entry.at)}</span>
                <strong>{entry.message}</strong>
                {entry.context && <p>{entry.context}</p>}
              </div>
            ))
          ) : (
            <Empty description="暂无本地日志" />
          )}
        </div>
      </section>
    );
  }
}

function SectionHead(props: { icon: React.ReactNode; title: string; description: string; action?: React.ReactNode }) {
  return (
    <div className="section-head">
      <div className="section-title">
        <span>{props.icon}</span>
        <div>
          <h3>{props.title}</h3>
          <p>{props.description}</p>
        </div>
      </div>
      {props.action}
    </div>
  );
}

function StatusLine(props: { label: string; value: string; tone: "ready" | "warning" | "idle" }) {
  return (
    <div className="status-line">
      <span>{props.label}</span>
      <strong>{props.value}</strong>
      <i className={props.tone} />
    </div>
  );
}

function CapabilityTag(props: { status: string }) {
  if (props.status === "ready") {
    return <Tag color="success">已配置</Tag>;
  }
  if (props.status === "browser_fallback") {
    return <Tag color="processing">开发模式</Tag>;
  }
  if (props.status === "unavailable") {
    return <Tag color="error">不可用</Tag>;
  }
  return <Tag color="warning">未配置/待接入</Tag>;
}

function QueueList(props: { items: SyncQueueItem[] }) {
  if (!props.items.length) {
    return <Empty description="暂无队列项目" />;
  }
  return (
    <div className="queue-list">
      {props.items.map((item) => (
        <div className="queue-row" key={item.id}>
          <div>
            <strong>{item.title}</strong>
            <p>{item.detail}</p>
          </div>
          <div className="queue-progress">
            <CapabilityTag status={item.status === "not_configured" ? "not_configured" : item.status === "failed" ? "unavailable" : item.status === "succeeded" ? "ready" : "browser_fallback"} />
            <Progress percent={item.progress} size="small" status={item.status === "failed" ? "exception" : undefined} />
          </div>
        </div>
      ))}
    </div>
  );
}

function ScanQueueTable(props: { items: SyncQueueItem[]; onRetry: (id: string) => void }) {
  const columns: TableColumnsType<SyncQueueItem> = [
    {
      title: "预览",
      dataIndex: "previewUrl",
      width: 82,
      render: (_value, item) =>
        item.previewUrl ? <img className="scan-preview" src={item.previewUrl} alt={item.fileName ?? item.title} /> : <span className="scan-preview placeholder">PDF</span>
    },
    {
      title: "文件",
      dataIndex: "fileName",
      render: (_value, item) => (
        <div className="scan-file-cell">
          <strong>{item.fileName ?? item.title}</strong>
          <span>
            {item.contentType ?? "unknown"} / {formatBytes(item.fileSize)}
          </span>
          {item.requiresReselect && <Tag color="warning">需重新选择文件</Tag>}
        </div>
      )
    },
    {
      title: "页码",
      dataIndex: "pageNo",
      width: 80,
      render: (value?: number) => value ?? "未设置"
    },
    {
      title: "本地质量检查",
      dataIndex: "qualityChecks",
      render: (checks?: ScanQualityCheck[]) => (
        <div className="quality-checks">
          {(checks ?? []).map((check) => (
            <Tooltip key={check.key} title={check.detail}>
              <Tag color={check.status === "passed" ? "success" : check.status === "failed" ? "error" : "warning"}>
                {check.label}: {check.status === "passed" ? "通过" : check.status === "failed" ? "失败" : "未配置/待接入"}
              </Tag>
            </Tooltip>
          ))}
        </div>
      )
    },
    {
      title: "队列状态",
      dataIndex: "status",
      width: 130,
      render: (status: SyncQueueItem["status"]) => <Tag color={status === "succeeded" ? "success" : status === "failed" ? "error" : status === "uploading" ? "processing" : "default"}>{queueStatusLabels[status]}</Tag>
    },
    {
      title: "进度",
      dataIndex: "progress",
      width: 150,
      render: (_value, item) => <Progress percent={item.progress} size="small" status={item.status === "failed" ? "exception" : undefined} />
    },
    {
      title: "服务端状态",
      dataIndex: "serverStatus",
      render: (_value, item) => item.serverStatus ?? item.detail
    },
    {
      title: "操作",
      key: "action",
      width: 92,
      render: (_value, item) => (
        <Button size="small" icon={<RotateCcw size={14} />} disabled={item.status === "uploading" || item.status === "succeeded"} onClick={() => props.onRetry(item.id)}>
          重试
        </Button>
      )
    }
  ];
  return (
    <Table
      rowKey="id"
      size="middle"
      columns={columns}
      dataSource={props.items}
      scroll={{ x: 1080 }}
      pagination={{ pageSize: 8, showSizeChanger: false }}
      locale={{ emptyText: <Empty description="尚未选择文件" /> }}
    />
  );
}

function formatDate(value?: string) {
  if (!value) {
    return "未记录";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return date.toLocaleString("zh-CN", { hour12: false });
}

function dependencyTone(status: "ok" | "error" | "not_configured") {
  if (status === "ok") {
    return "success";
  }
  if (status === "not_configured") {
    return "warning";
  }
  return "error";
}

function dependencyLabel(status: "ok" | "error" | "not_configured") {
  if (status === "ok") {
    return "正常";
  }
  if (status === "not_configured") {
    return "未配置/待接入";
  }
  return "异常";
}

function estimateLocalCacheBytes() {
  let total = 0;
  for (let index = 0; index < window.localStorage.length; index += 1) {
    const key = window.localStorage.key(index);
    if (!key || !key.startsWith("edugrade.desktop.")) {
      continue;
    }
    total += key.length;
    total += window.localStorage.getItem(key)?.length ?? 0;
  }
  return total;
}

async function inspectScanFile(file: File): Promise<ScanQualityCheck[]> {
  const extension = fileExtension(file.name);
  const allowedType = allowedUploadExtensions.includes(extension);
  const checks: ScanQualityCheck[] = [
    {
      key: "file_type",
      label: "文件类型",
      status: allowedType ? "passed" : "failed",
      detail: allowedType ? `${extension} 可上传` : `${extension || "无扩展名"} 不在允许范围`
    },
    {
      key: "file_size",
      label: "文件大小",
      status: file.size <= maxUploadBytes ? "passed" : "failed",
      detail: `${formatBytes(file.size)} / 上限 ${formatBytes(maxUploadBytes)}`
    },
    {
      key: "page_count",
      label: "页数",
      status: "not_configured",
      detail: "PDF 页数解析未配置/待接入；当前以后端 submission 质量门禁为准"
    }
  ];
  if (file.type.startsWith("image/")) {
    const resolution = await readImageResolution(file);
    checks.push({
      key: "resolution",
      label: "分辨率",
      status: resolution ? "passed" : "not_configured",
      detail: resolution ?? "图片分辨率无法读取，预留人工复核"
    });
  } else {
    checks.push({
      key: "resolution",
      label: "分辨率",
      status: "not_configured",
      detail: "PDF/非图片分辨率检测未配置/待接入"
    });
  }
  return checks;
}

function readImageResolution(file: File) {
  return new Promise<string | null>((resolve) => {
    const url = URL.createObjectURL(file);
    const image = new Image();
    image.onload = () => {
      const value = `${image.naturalWidth} x ${image.naturalHeight}`;
      URL.revokeObjectURL(url);
      resolve(value);
    };
    image.onerror = () => {
      URL.revokeObjectURL(url);
      resolve(null);
    };
    image.src = url;
  });
}

function previewUrlForFile(file: File) {
  return file.type.startsWith("image/") ? URL.createObjectURL(file) : undefined;
}

function fileExtension(filename: string) {
  const index = filename.lastIndexOf(".");
  return index >= 0 ? filename.slice(index).toLowerCase() : "";
}

function inferContentType(filename: string) {
  const extension = fileExtension(filename);
  if (extension === ".pdf") {
    return "application/pdf";
  }
  if (extension === ".png") {
    return "image/png";
  }
  if (extension === ".jpg" || extension === ".jpeg") {
    return "image/jpeg";
  }
  return "application/octet-stream";
}

function formatBytes(value?: number) {
  if (typeof value !== "number") {
    return "未记录";
  }
  if (value < 1024) {
    return `${value} B`;
  }
  if (value < 1024 * 1024) {
    return `${(value / 1024).toFixed(1)} KB`;
  }
  return `${(value / 1024 / 1024).toFixed(1)} MB`;
}

function readPersistedScanQueue(): SyncQueueItem[] {
  try {
    const raw = window.localStorage.getItem(scanQueueStorageKey);
    if (!raw) {
      return [];
    }
    const parsed = JSON.parse(raw) as SyncQueueItem[];
    if (!Array.isArray(parsed)) {
      return [];
    }
    return parsed.map((item) => {
      if (item.kind !== "scan_upload" || item.status === "succeeded") {
        return item;
      }
      return {
        ...item,
        status: item.status === "uploading" ? "failed" : item.status,
        progress: item.status === "uploading" ? 0 : item.progress,
        requiresReselect: !item.fileAssetId,
        detail: item.fileAssetId ? item.detail : "本地队列已恢复；文件句柄不可恢复，请重新选择同名文件后继续",
        previewUrl: undefined
      };
    });
  } catch {
    return [];
  }
}

function persistScanQueue(items: SyncQueueItem[]) {
  const serializable = items.map((item) => {
    const { previewUrl: _previewUrl, ...rest } = item;
    return rest;
  });
  try {
    window.localStorage.setItem(scanQueueStorageKey, JSON.stringify(serializable.slice(0, 300)));
  } catch (error) {
    console.warn("scan queue persistence failed", error);
  }
}

export default App;
