import { readFileSync, readdirSync } from "node:fs";
import { join } from "node:path";

const root = process.cwd();

function read(path) {
  return readFileSync(join(root, path), "utf8");
}

const routes = read("src/router/routes.tsx");
const dashboard = read("src/pages/DashboardPage.tsx");
const appLayout = read("src/components/AppLayout.tsx");
const app = read("src/App.tsx");
const login = read("src/pages/LoginPage.tsx");
const grading = read("src/pages/GradingWorkbenchPage.tsx");
const experience = read("src/router/experience.ts");
const teacherDashboard = read("src/pages/TeacherDashboardPage.tsx");
const responsiveTable = read("src/components/ResponsiveTable.tsx");
const examWorkspace = read("src/pages/ExamWorkspacePage.tsx");
const appealCenter = read("src/pages/AppealCenterPage.tsx");
const appealApi = read("src/api/appeals.ts");
const styles = read("src/styles.css");

function sourceFiles(directory) {
  return readdirSync(join(root, directory), { withFileTypes: true }).flatMap((entry) => {
    const path = join(directory, entry.name);
    if (entry.isDirectory()) return sourceFiles(path);
    return /\.(ts|tsx)$/.test(entry.name) ? [read(path)] : [];
  });
}

const failures = [];

function assert(condition, message) {
  if (!condition) {
    failures.push(message);
  }
}

assert(
  !/from\s+["']\.\.\/data["']/.test(dashboard),
  "DashboardPage must not import static demo data from src/data.ts."
);

assert(
  /listExams/.test(dashboard) && /listReviewTasks/.test(dashboard) && /listSubmissions/.test(dashboard),
  "DashboardPage must read business work from exams, review tasks, and submissions APIs."
);

assert(
  /user\.roles\.includes\("platform_admin"\)[\s\S]*getSystemStatus/.test(dashboard) || /isOperations[\s\S]*getSystemStatus/.test(dashboard),
  "System status must be restricted to the operations home variant."
);

assert(
  /productionReady/.test(routes) && /mockRoutesEnabled/.test(routes) && /visibleRoutes/.test(routes),
  "Routes must declare production readiness and expose environment-aware visibleRoutes()."
);

for (const key of ["review", "quality", "permissions", "settings"]) {
  const routePattern = new RegExp(`key:\\s*["']${key}["'][\\s\\S]*?productionReady:\\s*false`);
  assert(routePattern.test(routes), `Route ${key} must be hidden from production navigation.`);
}

assert(
  /routeFromPath[\s\S]*routes\.filter\(isRouteVisible\)/.test(routes),
  "routeFromPath must resolve only environment-visible routes, including hidden workspace routes."
);

assert(
  /hasRouteAccess\(user, route, experience\)/.test(appLayout) && /visibleRoutes\(experience\)/.test(appLayout),
  "AppLayout menu must combine production visibility with user permissions."
);

assert(
  /ADMIN_ROLES[\s\S]*platform_admin[\s\S]*tenant_admin[\s\S]*school_admin/.test(experience)
    && /TEACHER_ROLES[\s\S]*teacher[\s\S]*grader[\s\S]*arbitrator/.test(experience),
  "Product experiences must be derived from the approved administrator and teacher role sets."
);

assert(
  /experienceFromPath/.test(experience) && /canonicalPathFromPath/.test(experience) && /pathForExperience/.test(experience),
  "Experience routing must support prefixed hashes and canonical internal paths."
);

assert(
  /defaultExperience\(nextUser\)/.test(app) && /hasExperienceAccess\(user, experience\)/.test(app),
  "Login and deep-link access must enforce the role-specific product experience."
);

assert(
  /experienceLabel\(experience\)/.test(appLayout) && /onExperienceChange/.test(appLayout),
  "AppLayout must identify the current product end and support explicit mixed-role switching."
);

assert(
  /listReviewTasks\(\{ assigned_to: user\.id \}\)/.test(teacherDashboard)
    && /listArbitrationTasks\(\{ assigned_to: user\.id \}\)/.test(teacherDashboard),
  "Teacher home must request only review and arbitration tasks assigned to the current user."
);

assert(
  /personalScope \? \{ assigned_to: currentUserId \} : \{\}/.test(grading),
  "Teacher grading must enforce personal task scope in the API query."
);

assert(
  /teacher:\s*\["overview", "paper", "questions", "grading", "quality", "scores", "appeals", "reports"\]/.test(routes)
    && /hasExamWorkspaceSectionAccess/.test(app),
  "Teacher exam workspaces must use the approved section allowlist for navigation and deep links."
);

assert(
  /AdminGradingOperationsPage/.test(app) && /experience === "admin"/.test(app),
  "Administrator grading navigation must open exam-level operations instead of the personal workbench."
);

assert(
  /canWork=\{experience === "teacher" && hasEveryPermission\(user, \["appeal:work"\]\)\}/.test(app)
    && /selectedAppeal\?\.assigned_to === currentUser\.id/.test(appealCenter),
  "Teacher appeal handling must require appeal:work and an assignment to the current user."
);

assert(
  /assignAppeal/.test(appealCenter)
    && /submitAppealRecommendation/.test(appealCenter)
    && /建议不会直接修改成绩或关闭申诉/.test(appealCenter),
  "Appeal UI must separate administrator assignment from teacher recommendations."
);

assert(
  /\/assign/.test(appealApi) && /\/recommendation/.test(appealApi),
  "Appeal API client must expose assignment and teacher recommendation endpoints."
);

assert(
  /Drawer/.test(appLayout) && /mobile-nav-button/.test(appLayout),
  "Compact viewports must use a navigation drawer."
);

assert(
  /responsive-record-list/.test(responsiveTable) && /flexibleColumns/.test(responsiveTable),
  "Wide tables must have a vertical record mode and flexible desktop columns."
);

const allUiSource = sourceFiles("src").join("\n");
assert(!/scroll=\{\{\s*x\s*:/.test(allUiSource), "Production tables must not enable horizontal scrolling.");
assert(!/overflow-x:\s*(?:auto|scroll|hidden|clip)/.test(styles), "CSS must not create or conceal horizontal overflow.");

assert(
  /currentRoute\.mock/.test(appLayout) && !/<MockBadge compact \/>/.test(appLayout),
  "AppLayout must show the mock badge only for the current mock route."
);

assert(
  !/global-search|aria-label="任务队列"|aria-label="通知"/.test(appLayout),
  "AppLayout must not expose top-bar controls without implemented behavior."
);

assert(
  /lazy\(\(\) => import/.test(app) && /Suspense/.test(app),
  "Production pages must be loaded on demand."
);

assert(
  !/Demo 教育集团|平台租户/.test(login) && /name="tenant_code"/.test(login),
  "Login must accept real tenant codes instead of a fixed demo tenant list."
);

assert(
  !/待后续 API 支持|相似答案检索|历史样例库/.test(grading),
  "Production grading must not expose placeholders for later stories."
);

assert(
  /key:\s*"arbitration"[\s\S]*?anyPermissions:\s*\["arbitration:manage",\s*"arbitration:work"\]/.test(routes),
  "Arbitration navigation must allow existing worker and manager permissions."
);

if (failures.length > 0) {
  console.error("Production route checks failed:");
  for (const failure of failures) {
    console.error(`- ${failure}`);
  }
  process.exit(1);
}

console.log("Production route checks passed.");
