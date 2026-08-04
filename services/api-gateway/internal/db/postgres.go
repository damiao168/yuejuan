package db

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"edugrade-enterprise/services/api-gateway/internal/config"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

type QueryObserver struct {
	SlowThreshold time.Duration
	Observe       func(operation string, outcome string, duration time.Duration, slow bool)
	LogSlow       func(ctx context.Context, operation string, duration time.Duration, queryErr error)
}

func OpenPostgres(cfg config.PostgresConfig, observers ...QueryObserver) (*sql.DB, func() error, error) {
	dsn := cfg.DSN
	if len(observers) > 0 {
		connConfig, err := pgx.ParseConfig(cfg.DSN)
		if err != nil {
			return nil, nil, err
		}
		connConfig.Tracer = queryTracer{observer: observers[0]}
		dsn = stdlib.RegisterConnConfig(connConfig)
	}
	database, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, nil, err
	}
	database.SetMaxOpenConns(10)
	database.SetMaxIdleConns(5)
	database.SetConnMaxLifetime(30 * time.Minute)
	return database, database.Close, nil
}

type queryTraceState struct {
	startedAt time.Time
	operation string
}

type queryTraceKey struct{}

type queryTracer struct {
	observer QueryObserver
}

func (t queryTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	return context.WithValue(ctx, queryTraceKey{}, queryTraceState{
		startedAt: time.Now(),
		operation: sqlOperation(data.SQL),
	})
}

func (t queryTracer) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	state, ok := ctx.Value(queryTraceKey{}).(queryTraceState)
	if !ok {
		return
	}
	duration := time.Since(state.startedAt)
	slow := t.observer.SlowThreshold > 0 && duration >= t.observer.SlowThreshold
	outcome := "ok"
	if data.Err != nil {
		outcome = "error"
	}
	if t.observer.Observe != nil {
		t.observer.Observe(state.operation, outcome, duration, slow)
	}
	if slow && t.observer.LogSlow != nil {
		t.observer.LogSlow(ctx, state.operation, duration, data.Err)
	}
}

func sqlOperation(statement string) string {
	fields := strings.Fields(statement)
	if len(fields) == 0 {
		return "unknown"
	}
	operation := strings.ToLower(strings.Trim(fields[0], "();"))
	switch operation {
	case "select", "insert", "update", "delete", "with", "copy", "begin", "commit", "rollback":
		return operation
	default:
		return "other"
	}
}
