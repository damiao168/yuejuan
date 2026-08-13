import { queryOptions, useQuery, type QueryClient } from "@tanstack/react-query";
import { getExamWorkspace } from "../api/workspace";

export const examWorkspaceKeys = {
  all: ["exam-workspace"] as const,
  detail: (examId: string) => ["exam-workspace", examId] as const
};

export function examWorkspaceQueryOptions(examId: string) {
  return queryOptions({
    queryKey: examWorkspaceKeys.detail(examId),
    queryFn: async () => (await getExamWorkspace(examId)).workspace
  });
}

export function useExamWorkspace(examId: string) {
  return useQuery(examWorkspaceQueryOptions(examId));
}

export function invalidateExamWorkspace(queryClient: QueryClient, examId: string) {
  return queryClient.invalidateQueries({ queryKey: examWorkspaceKeys.detail(examId) });
}
