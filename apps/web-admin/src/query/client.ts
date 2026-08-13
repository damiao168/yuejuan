import { QueryClient } from "@tanstack/react-query";
import { ApiClientError } from "../api/client";

export function shouldRetryQuery(failureCount: number, error: unknown): boolean {
  if (error instanceof ApiClientError && error.status < 500) {
    return false;
  }
  return failureCount < 2;
}

export function createAppQueryClient(): QueryClient {
  return new QueryClient({
    defaultOptions: {
      queries: {
        staleTime: 30_000,
        gcTime: 5 * 60_000,
        retry: shouldRetryQuery,
        refetchOnWindowFocus: false
      },
      mutations: {
        retry: false
      }
    }
  });
}

export const appQueryClient = createAppQueryClient();
