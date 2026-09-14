package questionbank

import (
	"context"

	"edugrade-enterprise/services/api-gateway/internal/auth"
)

func (s *MemoryStore) PreviewImports(context.Context, auth.AccessScope, ImportPreviewInput) (ImportPreviewBatch, error) {
	return ImportPreviewBatch{}, ErrUnavailable
}

func (s *MemoryStore) ImportQuestion(context.Context, auth.AccessScope, string, ImportQuestionInput) (ImportResult, error) {
	return ImportResult{}, ErrUnavailable
}
