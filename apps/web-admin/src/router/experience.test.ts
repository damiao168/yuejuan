import { describe, expect, it } from "vitest";
import type { SessionUser } from "../auth/session";
import {
  availableExperiences,
  canonicalPathFromPath,
  defaultExperience,
  pathForExperience
} from "./experience";

function user(roles: string[], permissions: string[] = []): SessionUser {
  return {
    id: "user-1",
    username: "tester",
    displayName: "Tester",
    name: "Tester",
    role: roles[0] ?? "user",
    roles,
    tenant: "school-1",
    school: "School 1",
    currentExam: "",
    publicComputer: false,
    permissions,
    organizationScope: { resolved: false, tenantWide: false, schoolIds: [], gradeIds: [], classIds: [] }
  };
}

describe("role-specific product experience", () => {
  it("keeps platform and school administrators in the admin experience", () => {
    expect(availableExperiences(user(["platform_admin"]))).toEqual(["admin"]);
    expect(defaultExperience(user(["school_admin", "grader"]))).toBe("admin");
  });

  it("keeps a grader without administrator permissions out of the admin experience", () => {
    const grader = user(["grader"], ["review:work"]);

    expect(availableExperiences(grader)).toEqual(["teacher"]);
    expect(defaultExperience(grader)).toBe("teacher");
  });

	it("never promotes a business identity to admin from a permission", () => {
		expect(availableExperiences(user(["teacher"], ["exam:manage"]))).toEqual(["teacher"]);
	});

  it("preserves the canonical path and query when changing product experience", () => {
    expect(canonicalPathFromPath("/teacher/grading?exam_id=exam-1")).toBe("/grading");
    expect(pathForExperience("/teacher/grading?exam_id=exam-1", "admin"))
      .toBe("/admin/grading?exam_id=exam-1");
  });

  it("projects auditor and student identities into isolated read-only entries", () => {
    expect(availableExperiences(user(["auditor"], ["audit:read"]))).toEqual(["auditor"]);
    expect(defaultExperience(user(["auditor"], ["audit:read"]))).toBe("auditor");
    expect(pathForExperience("/dashboard", "auditor")).toBe("/auditor/dashboard");
    expect(availableExperiences(user(["student"], ["student:read"]))).toEqual(["student"]);
    expect(pathForExperience("/dashboard", "student")).toBe("/student/dashboard");
  });
});
