package postgres

import (
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/SPROG666/go-autumn-2026/internal/config"
)

func PoolConfig(cfg config.Config, tracer pgx.QueryTracer) (*pgxpool.Config, error) {
	pool, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("DATABASE_URL: %w", &connectionConfigError{cause: err})
	}
	pool.MaxConns = cfg.DatabaseMaxConns
	pool.MinConns = cfg.DatabaseMinConns
	pool.MaxConnLifetime = cfg.DatabaseMaxConnLifetime
	pool.ConnConfig.ConnectTimeout = cfg.DatabaseConnectTimeout
	pool.ConnConfig.Tracer = tracer
	return pool, nil
}

type connectionConfigError struct{ cause error }

func (*connectionConfigError) Error() string   { return "invalid PostgreSQL connection configuration" }
func (e *connectionConfigError) Unwrap() error { return e.cause }
