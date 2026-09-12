package auth

import (
	"context"
	"testing"

	database "edugrade-enterprise/services/api-gateway/internal/db"
)

func TestWithUserBindsDatabaseTenantContext(t *testing.T) {
	ctx := WithUser(context.Background(), User{ID: "actor-1", TenantID: "tenant-1"})
	tenantID, ok := database.TenantFromContext(ctx)
	if !ok || tenantID != "tenant-1" {
		t.Fatalf("database tenant context=%q ok=%v", tenantID, ok)
	}
}
