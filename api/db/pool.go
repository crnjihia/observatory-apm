// path: api/db/pool.go
package db

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPool initializes and validates a high-concurrency PostgreSQL/TimescaleDB connection pool.
func NewPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	if dsn == "" {
		dsn = os.Getenv("DATABASE_URL")
	}
	if dsn == "" {
		dsn = "postgres://observatory:observatory_secret@localhost:5432/observatory?sslmode=disable"
	}

	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database DSN: %w", err)
	}

	// Connection Pool Sizing
	maxConns := 50
	if v := os.Getenv("DB_MAX_CONNS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxConns = n
		}
	}
	minConns := 10
	if v := os.Getenv("DB_MIN_CONNS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			minConns = n
		}
	}

	config.MaxConns = int32(maxConns)
	config.MinConns = int32(minConns)
	config.MaxConnLifetime = 1 * time.Hour
	config.MaxConnIdleTime = 30 * time.Minute
	config.HealthCheckPeriod = 1 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create pgxpool: %w", err)
	}

	return pool, nil
}

// Ping checks whether the database is reachable.
func Ping(ctx context.Context, pool *pgxpool.Pool) error {
	if pool == nil {
		return fmt.Errorf("database pool is uninitialized")
	}
	return pool.Ping(ctx)
}

