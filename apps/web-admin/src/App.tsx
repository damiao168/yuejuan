import { lazy, Suspense, useEffect, useMemo, useState } from "react";
import { App as AntApp, ConfigProvider } from "antd";
import "antd/dist/reset.css";
import { getCurrentUser, login as loginWithPassword, logout as logoutSession } from "./api/auth";
import { listSchools } from "./api/org";
import { ApiClientError } from "./api/client";
import { hasAnyPermission, hasEveryPermission, sessionFromAuthUser, type SessionUser } from "./auth/session";
import { AppLayout } from "./components/AppLayout";
import { ForbiddenState, LoadingState, NotFoundState } from "./components/PageState";
import { LoginPage } from "./pages/LoginPage";
import { examWorkspaceFromPath, hasRouteAccess, notFoundRoute, pathFromHash, routeFromPath } from "./router/routes";
import type { LoginFormValues } from "./pages/LoginPage";

const AppealCenterPage = lazy(() => import("./pages/AppealCenterPage").then((module) => ({ default: module.AppealCenterPage })));
const ArbitrationPage = lazy(() => import("./pages/ArbitrationPage").then((module) => ({ default: module.ArbitrationPage })));
const AuditLogPage = lazy(() => import("./pages/AuditLogPage").then((module) => ({ default: module.AuditLogPage })));
const DashboardPage = lazy(() => import("./pages/DashboardPage").then((module) => ({ default: module.DashboardPage })));
const ExamManagementPage = lazy(() => import("./pages/ExamManagementPage").then((module) => ({ default: module.ExamManagementPage })));
const GradingWorkbenchPage = lazy(() => import("./pages/GradingWorkbenchPage").then((module) => ({ default: module.GradingWorkbenchPage })));
const LearningReportsPage = lazy(() => import("./pages/LearningReportsPage").then((module) => ({ default: module.LearningReportsPage })));
const OrganizationSetupPage = lazy(() => import("./pages/OrganizationSetupPage").then((module) => ({ default: module.OrganizationSetupPage })));
const ExamWorkspacePage = lazy(() => import("./pages/ExamWorkspacePage").then((module) => ({ default: module.ExamWorkspacePage })));
const AnswerSheetTemplatePage = lazy(() => import("./pages/AnswerSheetTemplatePage").then((module) => ({ default: module.AnswerSheetTemplatePage })));
const CaptureBatchPage = lazy(() => import("./pages/CaptureBatchPage").then((module) => ({ default: module.CaptureBatchPage })));
const ExamStudentScopePage = lazy(() => import("./pages/ExamPreparationPage").then((module) => ({ default: module.ExamStudentScopePage })));
const ExamReadinessPage = lazy(() => import("./pages/ExamPreparationPage").then((module) => ({ default: module.ExamReadinessPage })));
const ModulePage = lazy(() => import("./pages/ModulePage").then((module) => ({ default: module.ModulePage })));
const PaperRubricPage = lazy(() => import("./pages/PaperRubricPage").then((module) => ({ default: module.PaperRubricPage })));
const ScoreManagementPage = lazy(() => import("./pages/ScoreManagementPage").then((module) => ({ default: module.ScoreManagementPage })));
const SubmissionCapturePage = lazy(() => import("./pages/SubmissionCapturePage").then((module) => ({ default: module.SubmissionCapturePage })));
const SystemStatusPage = lazy(() => import("./pages/SystemStatusPage").then((module) => ({ default: module.SystemStatusPage })));

function App() {
  const [user, setUser] = useState<SessionUser | null>(null);
  const [authLoading, setAuthLoading] = useState(true);
  const [loginLoading, setLoginLoading] = useState(false);
  const [loginError, setLoginError] = useState<string | undefined>();
  const [path, setPath] = useState(pathFromHash);
  const [workspaceRefreshKey, setWorkspaceRefreshKey] = useState(0);

  useEffect(() => {
    const onHashChange = () => setPath(pathFromHash());
    window.addEventListener("hashchange", onHashChange);
    return () => window.removeEventListener("hashchange", onHashChange);
  }, []);

  useEffect(() => {
    if (!user || !user.permissions.includes("org:manage") || user.school !== user.tenant) {
      return;
    }
    let active = true;
    listSchools()
      .then((response) => {
        const school = response.schools[0];
        if (active && school) {
          setUser((current) => current?.id === user.id ? { ...current, school: school.name } : current);
        }
      })
      .catch(() => undefined);
    return () => { active = false; };
  }, [user]);

  useEffect(() => {
    let active = true;
    setAuthLoading(true);
    getCurrentUser()
      .then((response) => {
        if (active) {
          setUser(sessionFromAuthUser(response.user));
        }
      })
      .catch(() => {
        if (active) {
          setUser(null);
        }
      })
      .finally(() => {
        if (active) {
          setAuthLoading(false);
        }
      });
    return () => {
      active = false;
    };
  }, []);

  const route = useMemo(() => routeFromPath(path), [path]);
  const examWorkspace = useMemo(() => examWorkspaceFromPath(path), [path]);

  const navigate = (nextPath: string) => {
    window.location.hash = nextPath;
    setPath(nextPath);
  };

  const login = async (values: LoginFormValues) => {
    setLoginLoading(true);
    setLoginError(undefined);
    try {
      const response = await loginWithPassword(values);
      setUser(sessionFromAuthUser(response.user));
      navigate("/dashboard");
    } catch (error) {
      setLoginError(error instanceof ApiClientError && error.code === "invalid_credentials" ? "租户、账号或密码不正确。" : "暂时无法登录，请检查网络连接后重试。");
    } finally {
      setLoginLoading(false);
    }
  };

  const logout = () => {
    void logoutSession().catch(() => undefined);
    setUser(null);
  };

  if (authLoading) {
    return (
      <ConfigProvider>
        <AntApp>
          <LoadingState label="正在恢复登录状态" />
        </AntApp>
      </ConfigProvider>
    );
  }

  if (!user) {
    return (
      <ConfigProvider>
        <AntApp>
          <LoginPage onLogin={login} loading={loginLoading} error={loginError} />
        </AntApp>
      </ConfigProvider>
    );
  }

  const workspaceModule = examWorkspace ? (() => {
    const examId = examWorkspace.examId;
    const refreshWorkspace = () => setWorkspaceRefreshKey((value) => value + 1);
    switch (examWorkspace.section) {
      case "students":
        return <ExamStudentScopePage examId={examId} canManage={hasEveryPermission(user, ["exam:manage", "org:manage"])} onExamChanged={refreshWorkspace} />;
      case "paper":
      case "questions":
        return <PaperRubricPage canManage={hasEveryPermission(user, ["exam:manage", "file:manage"])} initialExamId={examId} onExamChanged={refreshWorkspace} />;
      case "template":
        return <AnswerSheetTemplatePage examId={examId} canManage={hasEveryPermission(user, ["exam:manage", "file:manage"])} onExamChanged={refreshWorkspace} />;
      case "settings":
        return <ExamReadinessPage examId={examId} canManage={hasEveryPermission(user, ["exam:manage"])} onNavigate={navigate} onExamChanged={refreshWorkspace} />;
      case "capture":
        return <CaptureBatchPage examId={examId} canManage={hasEveryPermission(user, ["capture:manage", "file:manage"])} />;
      case "processing":
        return <SubmissionCapturePage canManage={hasEveryPermission(user, ["submission:manage", "file:manage", "ocr:manage", "segment:manage"])} canReadStudentNames={hasEveryPermission(user, ["org:manage"])} initialExamId={examId} />;
      case "grading":
        return <GradingWorkbenchPage canWork={hasAnyPermission(user, ["review:manage", "review:work"])} canGrade={hasEveryPermission(user, ["grading:manage"])} canVerifyEvidence={hasEveryPermission(user, ["evidence:manage"])} canReturn={hasEveryPermission(user, ["review:manage"])} initialExamId={examId} />;
      case "quality":
        return <ArbitrationPage canAssign={hasEveryPermission(user, ["arbitration:manage"])} canWork={hasAnyPermission(user, ["arbitration:manage", "arbitration:work"])} canReadAudit={hasEveryPermission(user, ["audit:read"])} canReadExams={hasEveryPermission(user, ["exam:manage"])} currentUser={user} initialExamId={examId} />;
      case "scores":
        return <ScoreManagementPage canManage={hasEveryPermission(user, ["score:manage", "exam:manage", "submission:manage"])} canReadStudentNames={hasEveryPermission(user, ["org:manage"])} canReadAudit={hasEveryPermission(user, ["audit:read"])} initialExamId={examId} />;
      case "appeals":
        return <AppealCenterPage canRead={hasEveryPermission(user, ["appeal:read"])} canManage={hasEveryPermission(user, ["appeal:manage"])} canReadAudit={hasEveryPermission(user, ["audit:read"])} canReadIdentities={hasEveryPermission(user, ["org:manage"])} canReadExams={hasEveryPermission(user, ["exam:manage"])} currentUser={user} initialExamId={examId} />;
      case "reports":
        return <LearningReportsPage canRead={hasEveryPermission(user, ["report:read"])} canExport={hasEveryPermission(user, ["report:export"])} initialExamId={examId} />;
      default:
        return undefined;
    }
  })() : undefined;

  const content =
    route === notFoundRoute ? (
      <NotFoundState onBack={() => navigate("/dashboard")} />
    ) : !hasRouteAccess(user, route) ? (
      <ForbiddenState onBack={() => navigate("/dashboard")} />
    ) : route.path === "/dashboard" ? (
      <DashboardPage user={user} onNavigate={navigate} />
    ) : examWorkspace ? (
      <ExamWorkspacePage examId={examWorkspace.examId} section={examWorkspace.section} currentUser={user} moduleContent={workspaceModule} refreshKey={workspaceRefreshKey} onNavigate={navigate} />
    ) : route.path === "/exams" ? (
      <ExamManagementPage canManage={hasEveryPermission(user, ["exam:manage"])} currentUser={user} onOpenWorkspace={(examId) => navigate(`/exams/${encodeURIComponent(examId)}/overview`)} />
    ) : route.path === "/organization/setup" ? (
      <OrganizationSetupPage onNavigate={navigate} />
    ) : route.path === "/papers" ? (
      <PaperRubricPage canManage={hasEveryPermission(user, ["exam:manage", "file:manage"])} />
    ) : route.path === "/capture" ? (
      <SubmissionCapturePage
        canManage={hasEveryPermission(user, ["submission:manage", "file:manage", "ocr:manage", "segment:manage"])}
        canReadStudentNames={hasEveryPermission(user, ["org:manage"])}
      />
    ) : route.path === "/grading" ? (
      <GradingWorkbenchPage
        canWork={hasAnyPermission(user, ["review:manage", "review:work"])}
        canGrade={hasEveryPermission(user, ["grading:manage"])}
        canVerifyEvidence={hasEveryPermission(user, ["evidence:manage"])}
        canReturn={hasEveryPermission(user, ["review:manage"])}
      />
    ) : route.path === "/arbitration" ? (
      <ArbitrationPage
        canAssign={hasEveryPermission(user, ["arbitration:manage"])}
        canWork={hasAnyPermission(user, ["arbitration:manage", "arbitration:work"])}
        canReadAudit={hasEveryPermission(user, ["audit:read"])}
        canReadExams={hasEveryPermission(user, ["exam:manage"])}
        currentUser={user}
      />
    ) : route.path === "/scores" ? (
      <ScoreManagementPage
        canManage={hasEveryPermission(user, ["score:manage", "exam:manage", "submission:manage"])}
        canReadStudentNames={hasEveryPermission(user, ["org:manage"])}
        canReadAudit={hasEveryPermission(user, ["audit:read"])}
      />
    ) : route.path === "/reports" ? (
      <LearningReportsPage canRead={hasEveryPermission(user, ["report:read"])} canExport={hasEveryPermission(user, ["report:export"])} />
    ) : route.path === "/appeals" ? (
      <AppealCenterPage
        canRead={hasEveryPermission(user, ["appeal:read"])}
        canManage={hasEveryPermission(user, ["appeal:manage"])}
        canReadAudit={hasEveryPermission(user, ["audit:read"])}
        canReadIdentities={hasEveryPermission(user, ["org:manage"])}
        canReadExams={hasEveryPermission(user, ["exam:manage"])}
        currentUser={user}
      />
    ) : route.path === "/audit" ? (
      <AuditLogPage canRead={hasEveryPermission(user, ["audit:read"])} canExport={hasEveryPermission(user, ["audit:export"])} tenantName={user.tenant} />
    ) : route.path === "/system/status" ? (
      <SystemStatusPage />
    ) : (
      <ModulePage route={route} />
    );

  return (
    <AntApp>
      <AppLayout user={user} currentRoute={route} onNavigate={navigate} onLogout={logout}>
        <Suspense fallback={<LoadingState label="正在加载页面" />}>{content}</Suspense>
      </AppLayout>
    </AntApp>
  );
}

export default App;
