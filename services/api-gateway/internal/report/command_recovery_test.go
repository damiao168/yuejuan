package report

import (
	"bytes"
	"context"
	"edugrade-enterprise/services/api-gateway/internal/commandreceipt"
	"testing"
)

func TestReportCommandPreservesArtifactAndNewOperationIsDistinct(t *testing.T) {
	store := NewMemoryStore()
	ctx := commandreceipt.WithID(context.Background(), "export-one")
	first, err := store.Export(ctx, "tenant", "exam", "actor")
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.Export(ctx, "tenant", "exam", "actor")
	if err != nil || first.ReportID != second.ReportID || !bytes.Equal(first.Content, second.Content) || store.ExportCount() != 1 {
		t.Fatalf("artifact replay: %+v %v", second, err)
	}
	next, err := store.Export(commandreceipt.WithID(ctx, "export-two"), "tenant", "exam", "actor")
	if err != nil || next.ReportID == first.ReportID || store.ExportCount() != 2 {
		t.Fatalf("new operation: %+v %v", next, err)
	}
}
