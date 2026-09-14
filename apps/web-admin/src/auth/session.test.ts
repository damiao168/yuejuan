import { describe, expect, it } from "vitest";
import { sessionFromAuthUser } from "./session";

describe("session organization scope", () => {
  it("maps the server-resolved organization boundary", () => {
    const session = sessionFromAuthUser({
      id: "user-1", tenant_id: "tenant-1", tenant_code: "school",
      username: "grade_admin", display_name: "高二管理员", status: "active",
      roles: ["school_admin"], permissions: ["exam:manage"],
      data_scope: { school_admin: { scope: "grade", school_name: "示例中学" } },
      current_session_type: "public_device",
      organization_scope: {
        tenant_wide: false, school_ids: ["school-1"],
        grade_ids: ["grade-2"], class_ids: ["class-1", "class-2"]
      }
    });

    expect(session.school).toBe("示例中学");
    expect(session.organizationScope).toEqual({
      resolved: true, tenantWide: false, schoolIds: ["school-1"],
      gradeIds: ["grade-2"], classIds: ["class-1", "class-2"]
    });
    expect(session.publicComputer).toBe(true);
  });
});
