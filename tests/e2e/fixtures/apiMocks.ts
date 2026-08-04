import type { Page, Route } from "@playwright/test";

type TestRole = "school_admin" | "platform_admin" | "grader";

const rolePermissions: Record<TestRole, string[]> = {
  school_admin: [
    "exam:manage",
    "submission:manage",
    "file:manage",
    "ocr:manage",
    "segment:manage",
    "review:manage",
    "arbitration:manage",
    "score:manage",
    "report:read",
    "appeal:manage",
    "org:manage",
    "audit:read"
  ],
  platform_admin: ["tenant:manage", "system:read", "model:read", "audit:read"],
  grader: ["review:work"]
};

function userFor(role: TestRole) {
  return {
    id: `user-${role}`,
    tenant_id: role === "platform_admin" ? "platform" : "tenant-school",
    tenant_code: role === "platform_admin" ? "platform" : "demo-school",
    username: role,
    display_name: role === "school_admin" ? "学校管理员" : role === "grader" ? "阅卷老师" : "平台管理员",
    status: "active",
    roles: [role],
    permissions: rolePermissions[role],
    data_scope: role === "platform_admin" ? {} : { school_id: "school-1", school_name: "示范学校" }
  };
}

function json(route: Route, body: unknown, status = 200) {
  return route.fulfill({
    status,
    contentType: "application/json",
    body: JSON.stringify(body)
  });
}

export async function installApiMocks(
  page: Page,
  options: { role?: TestRole; initiallyAuthenticated?: boolean } = {}
) {
  const role = options.role ?? "school_admin";
  const user = userFor(role);
  let authenticated = options.initiallyAuthenticated ?? false;

  await page.route("**/api/v1/**", async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    const path = url.pathname;

    if (path === "/api/v1/auth/me") {
      return authenticated
        ? json(route, { user })
        : json(route, { error: { code: "unauthorized", message: "请先登录" } }, 401);
    }
    if (path === "/api/v1/auth/login") {
      authenticated = true;
      return json(route, {
        expires_at: "2099-01-01T00:00:00Z",
        user
      });
    }
    if (path === "/api/v1/auth/logout") return json(route, { status: "ok" });

    if (path === "/api/v1/dashboard/summary") {
      return json(route, {
        scope: { tenant_id: "tenant-school", school_id: "school-1" },
        updated_at: "2026-08-02T07:06:00Z",
        statistics: {
          active_exam_count: 12,
          collecting_exam_count: 2,
          pending_review_question_count: 107,
          pending_review_submission_count: 6,
          pending_arbitration_count: 3,
          pending_arbitration_submission_count: 2,
          failed_submission_count: 1,
          unmatched_submission_count: 1,
          quality_issue_submission_count: 1,
          finalized_exam_count: 1
        },
        blocking_issues: [],
        active_exams: [{
          id: "exam-1",
          name: "2026 春季数学期中考试",
          subject: "math",
          status: "collecting",
          submission_count: 48,
          failed_count: 0,
          quality_issue_count: 0,
          unmatched_count: 0,
          created_at: "2026-08-01T00:00:00Z"
        }],
        recent_activities: [],
        warnings: []
      });
    }

    if (path === "/api/v1/schools") {
      return json(route, { schools: [{ id: "school-1", tenant_id: "tenant-school", name: "示范学校", code: "DEMO" }] });
    }
    if (path === "/api/v1/students") return json(route, { students: [] });
    if (path === "/api/v1/ocr/availability") {
      return json(route, {
        generated_at: "2026-08-02T07:06:00Z",
        worker: {
          status: "ok",
          availability: "online",
          automation_available: true,
          fresh_instances: 1,
          stale_instances: 0,
          total_instances: 1
        }
      });
    }

    if (path === "/api/v1/exams") {
      return json(route, {
        exams: [{
          id: "exam-1",
          tenant_id: "tenant-school",
          school_id: "school-1",
          name: "2026 春季数学期中考试",
          subject: "math",
          exam_type: "midterm",
          total_score: 150,
          status: "collecting",
          grading_mode: "ai_assisted",
          appeal_enabled: true,
          publish_policy: "manual",
          created_by: "user-school_admin",
          class_ids: [],
          created_at: "2026-08-01T00:00:00Z"
        }]
      });
    }
    if (/^\/api\/v1\/exams\/[^/]+\/submissions$/.test(path)) return json(route, { submissions: [] });
    if (path === "/api/v1/ocr/tasks") return json(route, { tasks: [] });

    return json(route, { error: { code: "e2e_mock_missing", message: `未配置测试接口 ${path}` } }, 404);
  });
}

export function captureFailedRequests(page: Page) {
  const failures: string[] = [];
  page.on("requestfailed", (request) => {
    failures.push(`${request.method()} ${request.url()} ${request.failure()?.errorText ?? ""}`);
  });
  return failures;
}
