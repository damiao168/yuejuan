package db

import (
	"testing"
	"time"

	"edugrade-enterprise/services/api-gateway/internal/config"
)

func TestSQLOperationDoesNotExposeStatementText(t *testing.T) {
	tests := map[string]string{
		"SELECT * FROM student WHERE name = 'secret'": "select",
		"\nUPDATE exam SET status = $1":               "update",
		"student private answer":                      "other",
		"":                                            "unknown",
	}
	for statement, expected := range tests {
		if actual := sqlOperation(statement); actual != expected {
			t.Fatalf("sqlOperation(%q)=%q, want %q", statement, actual, expected)
		}
	}
}

func TestOpenPostgresAppliesBoundedPoolAndSessionTimeouts(t *testing.T) {
	cfg := config.PostgresConfig{
		DSN:              "postgres://test:test@127.0.0.1:5432/test?sslmode=disable",
		MaxOpenConns:     7,
		MaxIdleConns:     3,
		ConnMaxLifetime:  20 * time.Minute,
		ConnMaxIdleTime:  2 * time.Minute,
		StatementTimeout: 30 * time.Second,
		LockTimeout:      2 * time.Second,
	}
	database, closeDatabase, err := OpenPostgres(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if database.Stats().MaxOpenConnections != 7 {
		t.Fatalf("unexpected connection ceiling: %d", database.Stats().MaxOpenConnections)
	}
	connConfig, err := newPostgresConnConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if connConfig.RuntimeParams["statement_timeout"] != "30000ms" || connConfig.RuntimeParams["lock_timeout"] != "2000ms" {
		t.Fatalf("unexpected PostgreSQL session timeouts: %#v", connConfig.RuntimeParams)
	}
	if err := closeDatabase(); err != nil {
		t.Fatal(err)
	}
	if err := closeDatabase(); err != nil {
		t.Fatal("database cleanup must be idempotent")
	}
}

func TestPostgresDurationUsesBoundedMillisecondFormat(t *testing.T) {
	if actual := postgresDuration(1500 * time.Millisecond); actual != "1500ms" {
		t.Fatalf("unexpected PostgreSQL duration: %s", actual)
	}
}
