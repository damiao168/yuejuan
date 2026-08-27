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
  options: { role?: TestRole; initiallyAuthenticated?: boolean; dashboardMode?: "default" | "empty" } = {}
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
      if (options.dashboardMode === "empty") {
        return json(route, {
          scope: { tenant_id: "tenant-school", school_id: "school-1" },
          updated_at: "2026-08-02T07:06:00Z",
          statistics: {
            active_exam_count: 0,
            collecting_exam_count: 0,
            pending_review_question_count: 0,
            pending_review_submission_count: 0,
            pending_arbitration_count: 0,
            pending_arbitration_submission_count: 0,
            failed_submission_count: 0,
            unmatched_submission_count: 0,
            quality_issue_submission_count: 0,
            finalized_exam_count: 0
          },
          blocking_issues: [],
          active_exams: [],
          recent_activities: [],
          warnings: []
        });
      }
      return json(route, {
        scope: { tenant_id: "tenant-school", school_id: "school-1" },
        updated_at: "2026-08-02T07:06:00Z",
        statistics: {
          active_exam_count: 3,
          collecting_exam_count: 1,
          pending_review_question_count: 107,
          pending_review_submission_count: 6,
          pending_arbitration_count: 3,
          pending_arbitration_submission_count: 2,
          failed_submission_count: 1,
          unmatched_submission_count: 1,
          quality_issue_submission_count: 1,
          finalized_exam_count: 1
        },
        blocking_issues: [
          { code: "failed_submissions", label: "答题卡处理失败", count: 1, unit: "份", impact: "会阻断后续阅卷", action: "查看并重试", drilldown_path: "/capture?issue=failed" },
          { code: "unmatched_submissions", label: "答题卡未匹配学生", count: 1, unit: "份", impact: "无法计入学生成绩", action: "确认学生身份", drilldown_path: "/capture?issue=unmatched" },
          { code: "pending_arbitration", label: "待人工复核", count: 2, unit: "份", impact: "未确认最终得分", action: "开始复核", drilldown_path: "/arbitration?status=pending" }
        ],
        active_exams: [
          {
            id: "exam-1",
            name: "2026 春季数学期中考试",
            subject: "math",
            status: "collecting",
            submission_count: 48,
            failed_count: 1,
            quality_issue_count: 1,
            unmatched_count: 0,
            created_at: "2026-08-01T00:00:00Z"
          },
          {
            id: "exam-2",
            name: "高二语文月考",
            subject: "chinese",
            status: "draft",
            submission_count: 0,
            failed_count: 0,
            quality_issue_count: 0,
            unmatched_count: 0,
            created_at: "2026-07-31T00:00:00Z"
          },
          {
            id: "exam-3",
            name: "高一英语期末考试",
            subject: "english",
            status: "grading",
            submission_count: 126,
            failed_count: 0,
            quality_issue_count: 0,
            unmatched_count: 0,
            created_at: "2026-07-30T00:00:00Z"
          }
        ],
        recent_activities: [],
        warnings: []
      });
    }

    if (path === "/api/v1/schools") {
      return json(route, { schools: [{ id: "school-1", tenant_id: "tenant-school", name: "示范学校", code: "DEMO" }] });
    }
    if (path === "/api/v1/grades") return json(route, { grades: [{ id: "grade-1", school_id: "school-1", name: "高二", code: "G11", academic_year: "2026-2027", level_no: 11 }] });
    if (path === "/api/v1/classes") return json(route, { classes: [{ id: "class-1", school_id: "school-1", grade_id: "grade-1", name: "高二（1）班", code: "G11-01" }] });
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
    if (path === "/api/v1/exams/exam-1/workspace") {
      return json(route, {
        workspace: {
          exam_id: "exam-1",
          exam_name: "2026 春季数学期中考试",
          exam_status: "collecting",
          revision: 3,
          stage: "capture",
          stages: [
            { key: "prepare", label: "开考准备", state: "completed", action_route: "/exams/exam-1/settings" },
            { key: "capture", label: "答卷导入", state: "current", action_route: "/exams/exam-1/capture" },
            { key: "grading", label: "阅卷", state: "pending", action_route: "/exams/exam-1/grading" },
            { key: "quality", label: "复核与异常", state: "pending", action_route: "/exams/exam-1/quality" },
            { key: "results", label: "成绩与报告", state: "pending", action_route: "/exams/exam-1/scores" }
          ],
          stage_progress: [
            { stage: "prepare", status: "completed", completed: 5, total: 5, unit: "项", summary: "已通过 5 / 5 项开考检查" },
            { stage: "capture", status: "current", completed: 6, unit: "份", summary: "已导入 6 份答卷" },
            { stage: "grading", status: "pending", completed: 0, total: 107, unit: "个任务", summary: "尚未开始阅卷" },
            { stage: "quality", status: "pending", completed: 0, total: 3, unit: "个任务", summary: "尚未开始人工复核" },
            { stage: "results", status: "pending", summary: "成绩尚未发布" }
          ],
          blockers: [{
            code: "failed_submissions",
            title: "答卷处理失败",
            message: "1 份答卷处理失败，会阻断后续阅卷",
            severity: "blocker",
            action_label: "查看并重试",
            action_route: "/exams/exam-1/capture"
          }],
          warnings: [{
            code: "quality_issues",
            title: "图像质量需要关注",
            message: "1 份答卷存在图像质量问题",
            severity: "warning",
            action_label: "查看异常",
            action_route: "/exams/exam-1/capture"
          }],
          counts: {
            paper_count: 1,
            question_count: 19,
            submission_count: 6,
            failed_submission_count: 1,
            quality_issue_submission_count: 1,
            unmatched_submission_count: 0,
            pending_review_count: 107,
            pending_arbitration_count: 3
          },
          next_actions: [{
            code: "failed_submissions",
            label: "查看并重试",
            description: "答卷处理失败",
            route: "/exams/exam-1/capture",
            priority: "high"
          }],
          risk_tier: "R2",
          subject_summary: {
            code: "math",
            label: "数学",
            total_score: 150,
            question_count: 19,
            configured_question_count: 19,
            frozen_question_count: 19,
            risk_tier_source: "snapshot",
            question_types: { single_choice: 10, extended_response: 9 }
          },
          updated_at: "2026-08-12T08:00:00Z"
        }
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
