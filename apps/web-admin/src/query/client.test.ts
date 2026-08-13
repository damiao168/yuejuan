import { describe, expect, it } from "vitest";
import { ApiClientError } from "../api/client";
import { createAppQueryClient, shouldRetryQuery } from "./client";
import { examWorkspaceKeys, invalidateExamWorkspace } from "./examWorkspace";

describe("query defaults", () => {
  it("does not retry client errors and bounds transient retries", () => {
    expect(shouldRetryQuery(0, new ApiClientError(403, "forbidden", "Forbidden"))).toBe(false);
    expect(shouldRetryQuery(0, new ApiClientError(503, "unavailable", "Unavailable"))).toBe(true);
    expect(shouldRetryQuery(2, new TypeError("Network error"))).toBe(false);
  });

  it("invalidates only the changed exam workspace projection", async () => {
    const queryClient = createAppQueryClient();
    queryClient.setQueryData(examWorkspaceKeys.detail("exam-1"), { id: "exam-1" });
    queryClient.setQueryData(examWorkspaceKeys.detail("exam-2"), { id: "exam-2" });

    await invalidateExamWorkspace(queryClient, "exam-1");

    expect(queryClient.getQueryState(examWorkspaceKeys.detail("exam-1"))?.isInvalidated).toBe(true);
    expect(queryClient.getQueryState(examWorkspaceKeys.detail("exam-2"))?.isInvalidated).toBe(false);
    queryClient.clear();
  });
});
