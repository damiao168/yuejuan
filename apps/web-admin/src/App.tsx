import { lazy, Suspense, useEffect, useMemo, useState } from "react";
import { App as AntApp, ConfigProvider } from "antd";
import "antd/dist/reset.css";
import { getCurrentUser, login as loginWithPassword, logout as logoutSession } from "./api/auth";
import { listSchools } from "./api/org";
import { ApiClientError } from "./api/client";
import { hasAnyPermission, hasEveryPermission, sessionFromAuthUser, type SessionUser } from "./auth/session";
import { clearReviewDraftFallbacks } from "./auth/reviewDraftFallback";
import { clearRememberedLogin, loadRememberedLogin, saveRememberedLogin } from "./auth/rememberedLogin";
import { AppLayout } from "./components/AppLayout";
import { ErrorBoundary } from "./components/ErrorBoundary";
import { ForbiddenState, LoadingState, NotFoundState } from "./components/PageState";
import { LoginPage } from "./pages/LoginPage";
import { examWorkspaceFromPath, hasExamWorkspaceSectionAccess, hasRouteAccess, notFoundRoute, pathFromHash, routeFromPath } from "./router/routes";
import {
  availableExperiences,
  canonicalPathFromPath,
  defaultExperience,
  experienceFromPath,
  hasExperienceAccess,
  pathForExperience,
  type ProductExperience
} from "./router/experience";
import type { LoginFormValues } from "./pages/LoginPage";

const AppealCenterPage = lazy(() => import("./pages/AppealCenterPage").then((module) => ({ default: module.AppealCenterPage })));
const ArbitrationPage = lazy(() => import("./pages/ArbitrationPage").then((module) => ({ default: module.ArbitrationPage })));
const AuditLogPage = lazy(() => import("./pages/AuditLogPage").then((module) => ({ default: module.AuditLogPage })));
const DashboardPage = lazy(() => import("./pages/DashboardPage").then((module) => ({ default: module.DashboardPage })));
const TeacherDashboardPage = lazy(() => import("./pages/TeacherDashboardPage").then((module) => ({ default: module.TeacherDashboardPage })));
const ExamManagementPage = lazy(() => import("./pages/ExamManagementPage").then((module) => ({ default: module.ExamManagementPage })));
const GradingWorkbenchPage = lazy(() => import("./pages/GradingWorkbenchPage").then((module) => ({ default: module.GradingWorkbenchPage })));
const AdminGradingOperationsPage = lazy(() => import("./pages/AdminGradingOperationsPage").then((module) => ({ default: module.AdminGradingOperationsPage })));
const LearningReportsPage = lazy(() => import("./pages/LearningReportsPage").then((module) => ({ default: module.LearningReportsPage })));
const ModelGovernancePage = lazy(() => import("./pages/ModelGovernancePage").then((module) => ({ default: module.ModelGovernancePage })));
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
  const [loginDefaults, setLoginDefaults] = useState<Partial<LoginFormValues>>();
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
      .catch(async () => {
        const remembered = await loadRememberedLogin();
        if (!active) {
          return;
        }
        if (!remembered) {
          setUser(null);
          return;
        }
        setLoginDefaults({ ...remembered, remember_password: true });
        try {
          const response = await loginWithPassword(remembered);
          if (!active) {
            return;
          }
          const nextUser = sessionFromAuthUser(response.user);
          const nextPath = pathForExperience("/dashboard", defaultExperience(nextUser));
          setUser(nextUser);
          window.location.hash = nextPath;
          setPath(nextPath);
        } catch (error) {
          if (!active) {
            return;
          }
          setUser(null);
          if (error instanceof ApiClientError && error.code === "invalid_credentials") {
            clearRememberedLogin();
            setLoginDefaults(undefined);
            setLoginError("保存的登录信息已失效，请重新输入密码。");
          } else {
            setLoginError("自动登录失败，请检查网络连接后重试。");
          }
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

  const requestedExperience = useMemo(() => experienceFromPath(path), [path]);
  const experience = useMemo<ProductExperience>(
    () => requestedExperience ?? (user ? defaultExperience(user) : "admin"),
    [requestedExperience, user]
  );
  const navigationExperience = useMemo<ProductExperience>(
    () => user && !hasExperienceAccess(user, experience) ? defaultExperience(user) : experience,
    [experience, user]
  );
  const canonicalPath = useMemo(() => canonicalPathFromPath(path), [path]);
  const route = useMemo(() => routeFromPath(canonicalPath), [canonicalPath]);
  const examWorkspace = useMemo(() => examWorkspaceFromPath(canonicalPath), [canonicalPath]);

  useEffect(() => {
    if (!user || requestedExperience) return;
    const nextPath = pathForExperience(canonicalPath, defaultExperience(user));
    window.history.replaceState(null, "", `#${nextPath}`);
    setPath(nextPath);
  }, [canonicalPath, requestedExperience, user]);

  const navigate = (nextPath: string) => {
    const targetExperience = experienceFromPath(nextPath) ?? navigationExperience;
    const externalPath = pathForExperience(nextPath, targetExperience);
    window.location.hash = externalPath;
    setPath(externalPath);
  };

  const changeExperience = (nextExperience: ProductExperience) => {
    navigate(pathForExperience("/dashboard", nextExperience));
  };

  const login = async (values: LoginFormValues) => {
    setLoginLoading(true);
    setLoginError(undefined);
    const { remember_password, ...credentials } = values;
    try {
      const response = await loginWithPassword(credentials);
      const nextUser = sessionFromAuthUser(response.user);
      const nextPath = pathForExperience("/dashboard", defaultExperience(nextUser));
      if (remember_password) {
        try {
          await saveRememberedLogin(credentials);
          setLoginDefaults({ ...credentials, remember_password: true });
        } catch {
          clearRememberedLogin();
        }
      } else {
        clearRememberedLogin();
        setLoginDefaults(undefined);
      }
      setUser(nextUser);
      window.location.hash = nextPath;
      setPath(nextPath);
    } catch (error) {
      if (remember_password && error instanceof ApiClientError && error.code === "invalid_credentials") {
        clearRememberedLogin();
        setLoginDefaults(undefined);
      }
      setLoginError(error instanceof ApiClientError && error.code === "invalid_credentials" ? "学校代码、账号或密码不正确。" : "暂时无法登录，请检查网络连接后重试。");
    } finally {
      setLoginLoading(false);
    }
  };

  const logout = () => {
    void logoutSession().catch(() => undefined);
    if (user?.id) {
      clearReviewDraftFallbacks(user.id);
    }
    setUser(null);
  };

  const forgetRememberedLogin = () => {
    clearRememberedLogin();
    setLoginDefaults(undefined);
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
          <LoginPage
            onLogin={login}
            onForgetRemembered={forgetRememberedLogin}
            initialValues={loginDefaults}
            loading={loginLoading}
            error={loginError}
          />
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
        return <AnswerSheetTemplatePage examId={examId} canManage={experience === "admin" && hasEveryPermission(user, ["exam:manage", "file:manage"])} canCalibrate={experience === "admin" && hasEveryPermission(user, ["grading:manage"])} onExamChanged={refreshWorkspace} />;
      case "settings":
        return <ExamReadinessPage examId={examId} canManage={hasEveryPermission(user, ["exam:manage"])} onNavigate={navigate} onExamChanged={refreshWorkspace} />;
      case "capture":
        return <CaptureBatchPage examId={examId} canManage={hasEveryPermission(user, ["capture:manage", "file:manage"])} />;
      case "processing":
        return <SubmissionCapturePage canManage={hasEveryPermission(user, ["submission:manage", "file:manage", "ocr:manage", "segment:manage"])} canReadStudentNames={hasEveryPermission(user, ["org:manage"])} initialExamId={examId} />;
      case "grading":
        return <GradingWorkbenchPage canWork={hasAnyPermission(user, ["review:manage", "review:work"])} canManageTasks={experience === "admin" && hasEveryPermission(user, ["review:manage"])} canViewOriginalImage={experience === "admin"} canGrade={experience === "admin" && hasEveryPermission(user, ["grading:manage"])} canVerifyEvidence={experience === "admin" && hasEveryPermission(user, ["evidence:manage"])} canReturn={experience === "admin" && hasEveryPermission(user, ["review:manage"])} currentUserId={user.id} initialExamId={examId} personalScope={experience === "teacher"} />;
      case "quality":
        return <ArbitrationPage canAssign={experience === "admin" && hasEveryPermission(user, ["arbitration:manage"])} canWork={hasAnyPermission(user, ["arbitration:manage", "arbitration:work"])} canReadAudit={experience === "admin" && hasEveryPermission(user, ["audit:read"])} canReadExams={hasEveryPermission(user, ["exam:manage"])} currentUser={user} initialExamId={examId} personalScope={experience === "teacher"} />;
      case "scores":
        return <ScoreManagementPage mode={experience} canManage={experience === "admin" && hasEveryPermission(user, ["score:manage", "exam:manage", "submission:manage"])} canReadStudentNames={experience === "admin" && hasEveryPermission(user, ["org:manage"])} canReadAudit={experience === "admin" && hasEveryPermission(user, ["audit:read"])} initialExamId={examId} />;
      case "appeals":
        return <AppealCenterPage mode={experience} canRead={hasEveryPermission(user, ["appeal:read"])} canManage={experience === "admin" && hasEveryPermission(user, ["appeal:manage"])} canWork={experience === "teacher" && hasEveryPermission(user, ["appeal:work"])} canReadAudit={experience === "admin" && hasEveryPermission(user, ["audit:read"])} canReadIdentities={experience === "admin" && hasEveryPermission(user, ["org:manage"])} canReadExams={hasEveryPermission(user, ["exam:manage"])} currentUser={user} initialExamId={examId} />;
      case "reports":
        return <LearningReportsPage canRead={hasEveryPermission(user, ["report:read"])} canExport={hasEveryPermission(user, ["report:export"])} initialExamId={examId} />;
      default:
        return undefined;
    }
  })() : undefined;

  const content =
    route === notFoundRoute ? (
      <NotFoundState onBack={() => navigate("/dashboard")} />
    ) : !hasExperienceAccess(user, experience)
      || !hasRouteAccess(user, route, experience)
      || Boolean(examWorkspace && !hasExamWorkspaceSectionAccess(experience, examWorkspace.section)) ? (
      <ForbiddenState onBack={() => navigate("/dashboard")} />
    ) : route.path === "/dashboard" ? (
      experience === "admin" ? <DashboardPage user={user} onNavigate={navigate} /> : <TeacherDashboardPage user={user} onNavigate={navigate} />
    ) : examWorkspace ? (
      <ExamWorkspacePage examId={examWorkspace.examId} section={examWorkspace.section} experience={experience} currentUser={user} moduleContent={workspaceModule} refreshKey={workspaceRefreshKey} onNavigate={navigate} />
    ) : route.path === "/exams" ? (
      <ExamManagementPage mode={experience} canManage={experience === "admin" && hasEveryPermission(user, ["exam:manage"])} currentUser={user} onOpenWorkspace={(examId) => navigate(`/exams/${encodeURIComponent(examId)}/overview`)} />
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
      experience === "admin" ? <AdminGradingOperationsPage onNavigate={navigate} /> : (
        <GradingWorkbenchPage
          canWork={hasAnyPermission(user, ["review:manage", "review:work"])}
          canManageTasks={false}
          canViewOriginalImage={false}
          canGrade={false}
          canVerifyEvidence={false}
          canReturn={false}
          currentUserId={user.id}
          personalScope
        />
      )
    ) : route.path === "/arbitration" ? (
      <ArbitrationPage
        canAssign={experience === "admin" && hasEveryPermission(user, ["arbitration:manage"])}
        canWork={hasAnyPermission(user, ["arbitration:manage", "arbitration:work"])}
        canReadAudit={experience === "admin" && hasEveryPermission(user, ["audit:read"])}
        canReadExams={hasEveryPermission(user, ["exam:manage"])}
        currentUser={user}
        personalScope={experience === "teacher"}
      />
    ) : route.path === "/scores" ? (
      <ScoreManagementPage
        mode={experience}
        canManage={experience === "admin" && hasEveryPermission(user, ["score:manage", "exam:manage", "submission:manage"])}
        canReadStudentNames={experience === "admin" && hasEveryPermission(user, ["org:manage"])}
        canReadAudit={experience === "admin" && hasEveryPermission(user, ["audit:read"])}
      />
    ) : route.path === "/reports" ? (
      <LearningReportsPage canRead={hasEveryPermission(user, ["report:read"])} canExport={hasEveryPermission(user, ["report:export"])} />
    ) : route.path === "/appeals" ? (
      <AppealCenterPage
        mode={experience}
        canRead={hasEveryPermission(user, ["appeal:read"])}
        canManage={experience === "admin" && hasEveryPermission(user, ["appeal:manage"])}
        canWork={experience === "teacher" && hasEveryPermission(user, ["appeal:work"])}
        canReadAudit={experience === "admin" && hasEveryPermission(user, ["audit:read"])}
        canReadIdentities={experience === "admin" && hasEveryPermission(user, ["org:manage"])}
        canReadExams={hasEveryPermission(user, ["exam:manage"])}
        currentUser={user}
      />
    ) : route.path === "/audit" ? (
      <AuditLogPage canRead={hasEveryPermission(user, ["audit:read"])} canExport={hasEveryPermission(user, ["audit:export"])} tenantName={user.tenant} />
    ) : route.path === "/system/status" ? (
      <SystemStatusPage />
    ) : route.path === "/system/models" ? (
      <ModelGovernancePage
        canManageProviders={hasEveryPermission(user, ["model:provider:manage"])}
        canManagePolicy={hasEveryPermission(user, ["model:policy:manage"])}
        canManageEvaluations={hasEveryPermission(user, ["model:evaluation:manage"])}
      />
    ) : (
      <ModulePage route={route} experience={navigationExperience} onNavigate={navigate} />
    );

  return (
    <AntApp>
      <AppLayout
        user={user}
        currentRoute={route}
        experience={navigationExperience}
        availableExperiences={availableExperiences(user)}
        onNavigate={navigate}
        onExperienceChange={changeExperience}
        onLogout={logout}
        immersive={experience === "teacher" && (route.path === "/grading" || examWorkspace?.section === "grading")}
      >
        <ErrorBoundary resetKey={`${navigationExperience}:${canonicalPath}`}>
          <Suspense fallback={<LoadingState label="正在加载页面" />}>{content}</Suspense>
        </ErrorBoundary>
      </AppLayout>
    </AntApp>
  );
}

export default App;
