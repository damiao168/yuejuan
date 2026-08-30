import { useEffect, useRef } from "react";
import { downloadReviewWorkspaceImage, type ReviewTask } from "../../../../api/review";
import { fetchTaskBundle } from "../gradingTaskContext";
import type { PrefetchedTaskBundle } from "../gradingWorkbench.types";

export interface UseGradingPrefetchOptions {
  canWork: boolean;
  canViewOriginalImage: boolean;
  currentUserId: string;
  initialExamId: string;
  selectedTaskId: string;
  tasks: ReviewTask[];
}

export function useGradingPrefetch({
  canWork,
  canViewOriginalImage,
  currentUserId,
  initialExamId,
  selectedTaskId,
  tasks
}: UseGradingPrefetchOptions) {
  const prefetchedTaskRef = useRef(new Map<string, Promise<PrefetchedTaskBundle>>());
  const prefetchedPreviewRef = useRef(new Map<string, Promise<Awaited<ReturnType<typeof downloadReviewWorkspaceImage>> | null>>());

  useEffect(() => {
    if (!canWork || tasks.length === 0) return;
    for (const taskId of prefetchedTaskRef.current.keys()) {
      if (taskId !== selectedTaskId && !tasks.some((task) => task.id === taskId)) prefetchedTaskRef.current.delete(taskId);
    }
    for (const taskId of prefetchedPreviewRef.current.keys()) {
      if (taskId !== selectedTaskId && !tasks.some((task) => task.id === taskId)) prefetchedPreviewRef.current.delete(taskId);
    }
    for (const task of tasks) {
      if (task.assigned_to !== currentUserId || !["assigned", "in_progress", "returned"].includes(task.status) || prefetchedTaskRef.current.has(task.id)) continue;
      const bundlePromise = fetchTaskBundle(task.id, currentUserId, initialExamId, canViewOriginalImage);
      prefetchedTaskRef.current.set(task.id, bundlePromise);
      prefetchedPreviewRef.current.set(
        task.id,
        bundlePromise
          .then((bundle) => bundle.context.segmentImageUrl ? downloadReviewWorkspaceImage(bundle.context.segmentImageUrl) : null)
          .catch(() => null)
      );
      void bundlePromise.catch(() => {
        prefetchedTaskRef.current.delete(task.id);
        prefetchedPreviewRef.current.delete(task.id);
      });
    }
  }, [canViewOriginalImage, canWork, currentUserId, initialExamId, selectedTaskId, tasks]);

  return { prefetchedTaskRef, prefetchedPreviewRef };
}
