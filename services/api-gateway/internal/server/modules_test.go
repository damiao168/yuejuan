package server

import (
	"strings"
	"testing"
)

func TestProductionStoreGraphRejectsMemoryStores(t *testing.T) {
	err := validatePostgresStoreGraph(NewMemoryApplicationStores())
	if err == nil || !strings.Contains(err.Error(), "Memory") {
		t.Fatalf("production graph accepted a memory store: %v", err)
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
