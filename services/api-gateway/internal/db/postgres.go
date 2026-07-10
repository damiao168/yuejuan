package db

import (
	"database/sql"
	"time"

	"edugrade-enterprise/services/api-gateway/internal/config"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func OpenPostgres(cfg config.PostgresConfig) (*sql.DB, func() error, error) {
	database, err := sql.Open("pgx", cfg.DSN)
	if err != nil {
		return nil, nil, err
	}
	database.SetMaxOpenConns(10)
	database.SetMaxIdleConns(5)
	database.SetConnMaxLifetime(30 * time.Minute)
	return database, database.Close, nil
}
