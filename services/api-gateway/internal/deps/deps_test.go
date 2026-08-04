package deps

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCheckAllOnlyBlocksReadinessForRequiredDependencies(t *testing.T) {
	results, ready := CheckAll(context.Background(), time.Second, []Checker{
		StaticChecker{CheckerName: "postgres"},
		NotConfiguredChecker{CheckerName: "ai_service", DetailText: "optional"},
	})
	if !ready {
		t.Fatal("optional dependency must not block readiness")
	}
	if len(results) != 2 || !results[0].Required || results[1].Required || results[1].Status != "not_configured" {
		t.Fatalf("unexpected dependency results: %#v", results)
	}

	_, ready = CheckAll(context.Background(), time.Second, []Checker{
		StaticChecker{CheckerName: "postgres", Err: errors.New("offline")},
	})
	if ready {
		t.Fatal("required dependency failure must block readiness")
	}
}
