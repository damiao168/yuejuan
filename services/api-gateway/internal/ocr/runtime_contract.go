package ocr

import "edugrade-enterprise/services/api-gateway/internal/workerruntime"

const RuntimeResultSchema = "ocr-result.v1"

// RuntimeResult is the canonical source result mirrored into Worker Runtime.
// Keep this small and source-derived so retry comparisons remain deterministic.
func RuntimeResult(task Task) map[string]any {
	return map[string]any{
		"ocr_task_id":   task.ID,
		"result_count":  task.ResultCount,
		"model_version": task.ModelVersion,
		"config_hash":   task.ConfigHash,
		"input_hash":    task.InputHash,
	}
}

func runtimeCreateInput(task Task, sourceFileAssetIDs ...string) workerruntime.CreateTaskInput {
	pages := make([]any, 0, len(sourceFileAssetIDs))
	for _, fileAssetID := range sourceFileAssetIDs {
		pages = append(pages, map[string]any{"source_file_asset_id": fileAssetID})
	}
	return workerruntime.CreateTaskInput{
		TaskType: "ocr", QueueName: "ocr", SourceType: "ocr_task", SourceID: task.ID,
		IdempotencyKey: "ocr-task:" + task.ID, PayloadSchemaVersion: "ocr-task.v1",
		Payload: map[string]any{
			"ocr_task_id": task.ID, "submission_id": task.SubmissionID,
			"engine": task.Engine, "engine_version": task.EngineVersion,
			"pages": pages,
		},
		MaxAttempts: 3, RetryBackoffSeconds: 30,
	}
}

func runtimeCompleteInput(task Task, leaseToken string) workerruntime.CompleteInput {
	return workerruntime.CompleteInput{
		LeaseToken: leaseToken, ResultSchemaVersion: RuntimeResultSchema,
		DurationMS: task.DurationMS, Result: RuntimeResult(task),
	}
}
