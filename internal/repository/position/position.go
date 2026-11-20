package position

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/SPROG666/go-autumn-2026/internal/domain/position"
	"github.com/SPROG666/go-autumn-2026/internal/infra/postgres/transaction"
)

type Repository struct {
	pool    database
	timeout time.Duration
}

func New(pool database, timeout time.Duration) *Repository {
	return &Repository{pool: pool, timeout: timeout}
}

func (r *Repository) Save(ctx context.Context, value position.Position) (position.Position, error) {
	tx, ok := transaction.Current(ctx)
	if !ok {
		return position.Position{}, fmt.Errorf("save position requires a transaction")
	}
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	value.RecordedAt = value.RecordedAt.UTC().Truncate(time.Microsecond)
	err := tx.QueryRow(ctx, `INSERT INTO trip_positions (trip_id, latitude, longitude, recorded_at)
        VALUES ($1, $2, $3, $4) RETURNING id`, value.TripID, value.Latitude, value.Longitude, value.RecordedAt).Scan(&value.ID)
	if err != nil {
		return position.Position{}, fmt.Errorf("insert position: %w", err)
	}
	_, err = tx.Exec(ctx, `UPDATE trips SET last_position_at = GREATEST(last_position_at, $2::timestamptz),
        updated_at = CURRENT_TIMESTAMP WHERE id = $1`, value.TripID, value.RecordedAt)
	if err != nil {
		return position.Position{}, fmt.Errorf("update last position time: %w", err)
	}
	return value, nil
}

func (r *Repository) List(ctx context.Context, id uuid.UUID) ([]position.Position, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	executor := r.pool
	if tx, ok := transaction.Current(ctx); ok {
		executor = tx
	}
	rows, err := executor.Query(ctx, `SELECT id, trip_id, latitude, longitude, recorded_at
        FROM trip_positions WHERE trip_id = $1 ORDER BY recorded_at, id`, id)
	if err != nil {
		return nil, fmt.Errorf("list positions: %w", err)
	}
	defer rows.Close()
	values, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (position.Position, error) {
		var value position.Position
		err := row.Scan(&value.ID, &value.TripID, &value.Latitude, &value.Longitude, &value.RecordedAt)
		value.RecordedAt = value.RecordedAt.UTC()
		return value, err
	})
	if err != nil {
		return nil, fmt.Errorf("read positions: %w", err)
	}
	return values, nil
}
