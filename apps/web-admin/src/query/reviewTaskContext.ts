import { queryOptions, useQuery, type QueryClient } from "@tanstack/react-query";
import { getReviewTaskContext } from "../api/review";

export const reviewTaskContextKeys = {
  all: ["review-task-context"] as const,
  detail: (taskId: string) => ["review-task-context", taskId] as const
};

export function reviewTaskContextQueryOptions(taskId: string) {
  return queryOptions({
    queryKey: reviewTaskContextKeys.detail(taskId),
    queryFn: async ({ signal }) => (await getReviewTaskContext(taskId, signal)).context,
    enabled: Boolean(taskId),
    staleTime: 10_000
  });
}

export function useReviewTaskContext(taskId: string) {
  return useQuery(reviewTaskContextQueryOptions(taskId));
}

export function invalidateReviewTaskContext(queryClient: QueryClient, taskId: string) {
  return queryClient.invalidateQueries({ queryKey: reviewTaskContextKeys.detail(taskId) });
}
