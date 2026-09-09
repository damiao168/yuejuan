package server

import (
	"strings"
	"testing"

	"edugrade-enterprise/services/api-gateway/internal/auth"
	"edugrade-enterprise/services/api-gateway/internal/config"
)

func TestTransactionalApplicationCompositionRejectsMissingCoordinatorCapability(t *testing.T) {
	stores := NewMemoryApplicationStores()
	_, err := NewTransactionalCaptureProcessingModule(config.Config{}, stores.Capture,
		&IdentityModule{AuthStore: stores.Identity.Auth},
		&ExamPreparationModule{SubmissionStore: stores.Exam.Submissions, FileStore: stores.Exam.Files})
	if err == nil || !strings.Contains(err.Error(), "transactional command coordination") {
		t.Fatalf("production composition accepted a store without required capability: %v", err)
	}
}

func TestTransactionalApplicationRejectsTypedNilBeforeConstruction(t *testing.T) {
	stores := NewMemoryApplicationStores()
	var missing *auth.MemoryStore
	stores.Identity.Auth = missing
	_, err := NewTransactionalApplicationModules(ApplicationDependencies{}, stores)
	if err == nil || !strings.Contains(err.Error(), "stores.Identity.Auth is not configured") {
		t.Fatalf("production composition accepted a typed nil store: %v", err)
	}
}

func TestProductionStoreGraphRejectsMissingStores(t *testing.T) {
	stores := NewMemoryApplicationStores()
	stores.Identity.Auth = nil
	err := validatePostgresStoreGraph(stores)
	if err == nil || !strings.Contains(err.Error(), "stores.Identity.Auth is not configured") {
		t.Fatalf("production graph accepted a missing store: %v", err)
	}
}
