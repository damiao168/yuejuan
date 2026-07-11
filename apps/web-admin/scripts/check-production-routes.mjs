import { readFileSync } from "node:fs";
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
  /hasRouteAccess/.test(appLayout) && /visibleRoutes\(\)/.test(appLayout),
  "AppLayout menu must combine production visibility with user permissions."
);

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
