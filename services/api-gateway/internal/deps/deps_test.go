package deps

import (
	"context"
	"database/sql"
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

func TestPostgresCheckerDoesNotOverrideConfiguredPoolLimits(t *testing.T) {
	database, err := sql.Open("pgx", "postgres://test:test@127.0.0.1:5432/test?sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	database.SetMaxOpenConns(17)

	_ = NewPostgresChecker(database)
	if database.Stats().MaxOpenConnections != 17 {
		t.Fatalf("readiness checker changed configured pool ceiling: %d", database.Stats().MaxOpenConnections)
	}
}
