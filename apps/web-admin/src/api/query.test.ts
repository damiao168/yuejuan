import { describe, expect, it } from "vitest";
import { buildQueryString } from "./query";

describe("buildQueryString", () => {
  it("encodes cursor filters consistently", () => {
    expect(buildQueryString({ status: "under review", limit: 50, cursor: "a/b+c=" }))
      .toBe("?status=under+review&limit=50&cursor=a%2Fb%2Bc%3D");
  });

  it("keeps legacy omission semantics and comma-joins list filters", () => {
    expect(buildQueryString({ q: "", limit: 0, cursor: undefined, ids: ["student-1", " ", "student-2"] }))
      .toBe("?ids=student-1%2Cstudent-2");
  });
});
