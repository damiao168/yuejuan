package seedquality

import (
	"context"
	"edugrade-enterprise/services/api-gateway/internal/commandreceipt"
)

type commandResult struct {
	Task struct {
		ID       string `json:"id"`
		Status   string `json:"status"`
		Revision int64  `json:"revision"`
	} `json:"task"`
}

func seedCommandResult(task Task) commandResult {
	var result commandResult
	result.Task.ID = task.ID
	result.Task.Status = task.Status
	result.Task.Revision = task.Revision
	return result
}
func (s *PostgresStore) RecoverCommand(ctx context.Context, tenant, actor, id string) (commandreceipt.Receipt, error) {
	return commandreceipt.Recover(ctx, s.db, tenant, actor, id, "review.")
}
func (s *MemoryStore) RecoverCommand(ctx context.Context, tenant, actor, id string) (commandreceipt.Receipt, error) {
	return s.receipts.Recover(ctx, tenant, actor, id)
}
func (s *Service) RecoverCommand(ctx context.Context, tenant, actor, id string) (commandreceipt.Receipt, error) {
	return s.store.RecoverCommand(ctx, tenant, actor, id)
}
