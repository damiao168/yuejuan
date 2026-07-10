import { lazy, Suspense, useEffect, useMemo, useState } from "react";
import { App as AntApp, ConfigProvider } from "antd";
import "antd/dist/reset.css";
import { getCurrentUser, login as loginWithPassword, logout as logoutSession } from "./api/auth";
import { ApiClientError } from "./api/client";
import { hasAnyPermission, hasEveryPermission, sessionFromAuthUser, type SessionUser } from "./auth/session";
import { AppLayout } from "./components/AppLayout";
import { ForbiddenState, LoadingState, NotFoundState } from "./components/PageState";
import { LoginPage } from "./pages/LoginPage";
import { hasRouteAccess, notFoundRoute, pathFromHash, routeFromPath } from "./router/routes";
import type { LoginFormValues } from "./pages/LoginPage";

const AppealCenterPage = lazy(() => import("./pages/AppealCenterPage").then((module) => ({ default: module.AppealCenterPage })));
const ArbitrationPage = lazy(() => import("./pages/ArbitrationPage").then((module) => ({ default: module.ArbitrationPage })));
const AuditLogPage = lazy(() => import("./pages/AuditLogPage").then((module) => ({ default: module.AuditLogPage })));
const DashboardPage = lazy(() => import("./pages/DashboardPage").then((module) => ({ default: module.DashboardPage })));
const ExamManagementPage = lazy(() => import("./pages/ExamManagementPage").then((module) => ({ default: module.ExamManagementPage })));
const GradingWorkbenchPage = lazy(() => import("./pages/GradingWorkbenchPage").then((module) => ({ default: module.GradingWorkbenchPage })));
const LearningReportsPage = lazy(() => import("./pages/LearningReportsPage").then((module) => ({ default: module.LearningReportsPage })));
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

  useEffect(() => {
    const onHashChange = () => setPath(pathFromHash());
    window.addEventListener("hashchange", onHashChange);
    return () => window.removeEventListener("hashchange", onHashChange);
  }, []);

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

  const content =
    route === notFoundRoute ? (
      <NotFoundState onBack={() => navigate("/dashboard")} />
    ) : !hasRouteAccess(user, route) ? (
      <ForbiddenState onBack={() => navigate("/dashboard")} />
    ) : route.path === "/dashboard" ? (
      <DashboardPage />
    ) : route.path === "/exams" ? (
      <ExamManagementPage canManage={hasEveryPermission(user, ["exam:manage"])} />
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
