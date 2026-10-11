package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/SPROG666/go-autumn-2026/internal/storage"
	"github.com/SPROG666/go-autumn-2026/internal/trip"
)

type IdempotencyRepository struct {
	pool         *pgxpool.Pool
	queryTimeout time.Duration
	ttl          time.Duration
}

func NewIdempotencyRepository(pool *pgxpool.Pool, queryTimeout, ttl time.Duration) *IdempotencyRepository {
	return &IdempotencyRepository{pool: pool, queryTimeout: queryTimeout, ttl: ttl}
}

func (r *IdempotencyRepository) CleanupExpired(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()
	query, args, err := sq.StatementBuilder.PlaceholderFormat(sq.Dollar).
		Delete("trip_idempotency").
		Where("key IN (SELECT key FROM trip_idempotency WHERE expires_at <= clock_timestamp() ORDER BY expires_at LIMIT 100 FOR UPDATE SKIP LOCKED)").ToSql()
	if err != nil {
		return fmt.Errorf("build expired keys cleanup: %w", err)
	}
	if _, err := r.pool.Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("delete expired idempotency keys: %w", err)
	}
	return nil
}

func (r *IdempotencyRepository) Reserve(ctx context.Context, key uuid.UUID, params trip.CreateParams) (uuid.UUID, bool, error) {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()
	executor := storage.Executor(ctx, r.pool)
	if _, ok := executor.(pgx.Tx); !ok {
		return uuid.Nil, false, fmt.Errorf("reserve idempotency key requires a transaction")
	}
	payload, err := json.Marshal(params)
	if err != nil {
		return uuid.Nil, false, fmt.Errorf("encode idempotency request: %w", err)
	}
	query, args, err := sq.StatementBuilder.PlaceholderFormat(sq.Dollar).
		Insert("trip_idempotency").Columns("key", "request", "expires_at").
		Values(key, payload, sq.Expr("clock_timestamp() + ? * interval '1 microsecond'", r.ttl.Microseconds())).
		Suffix("ON CONFLICT (key) DO UPDATE SET key = EXCLUDED.key").
		Suffix("RETURNING request = ?::jsonb, trip_id, expires_at <= clock_timestamp()", payload).ToSql()
	if err != nil {
		return uuid.Nil, false, fmt.Errorf("build reserve idempotency key: %w", err)
	}
	var matches, expired bool
	var tripID *uuid.UUID
	if err := executor.QueryRow(ctx, query, args...).Scan(&matches, &tripID, &expired); err != nil {
		return uuid.Nil, false, fmt.Errorf("reserve idempotency key: %w", err)
	}
	if expired {
		query, args, err = sq.StatementBuilder.PlaceholderFormat(sq.Dollar).
			Update("trip_idempotency").Set("request", payload).Set("trip_id", nil).
			Where(sq.Eq{"key": key}).ToSql()
		if err != nil {
			return uuid.Nil, false, fmt.Errorf("build reset idempotency key: %w", err)
		}
		if _, err := executor.Exec(ctx, query, args...); err != nil {
			return uuid.Nil, false, fmt.Errorf("reset idempotency key: %w", err)
		}
		return uuid.Nil, false, nil
	}
	if !matches {
		return uuid.Nil, false, trip.ErrIdempotencyConflict
	}
	if tripID == nil {
		return uuid.Nil, false, nil
	}
	return *tripID, true, nil
}

func (r *IdempotencyRepository) Save(ctx context.Context, key uuid.UUID, tripID uuid.UUID) error {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()
	executor := storage.Executor(ctx, r.pool)
	if _, ok := executor.(pgx.Tx); !ok {
		return fmt.Errorf("save idempotency trip link requires a transaction")
	}
	query, args, err := sq.StatementBuilder.PlaceholderFormat(sq.Dollar).
		Update("trip_idempotency").Set("trip_id", tripID).
		Set("expires_at", sq.Expr("clock_timestamp() + ? * interval '1 microsecond'", r.ttl.Microseconds())).
		Where(sq.Eq{"key": key}).ToSql()
	if err != nil {
		return fmt.Errorf("build save idempotency trip link: %w", err)
	}
	result, err := executor.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("save idempotency trip link: %w", err)
	}
	if result.RowsAffected() != 1 {
		return fmt.Errorf("save idempotency trip link: expected one reserved key")
	}
	return nil
}
