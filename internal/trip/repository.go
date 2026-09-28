package trip

import (
	"context"
	"errors"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/SPROG666/go-autumn-2026/internal/storage"
)

type Repository struct {
	pool         *pgxpool.Pool
	queryTimeout time.Duration
}

func NewRepository(pool *pgxpool.Pool, queryTimeout time.Duration) *Repository {
	return &Repository{pool: pool, queryTimeout: queryTimeout}
}

func (r *Repository) Create(ctx context.Context, value Trip) (Trip, error) {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	query, args, err := sq.StatementBuilder.
		PlaceholderFormat(sq.Dollar).
		Insert("trips").
		Columns(
			"id", "user_id", "driver_id",
			"start_latitude", "start_longitude", "end_latitude", "end_longitude",
			"price", "status", "started_at", "finished_at",
		).
		Values(
			value.ID, value.UserID, value.DriverID,
			value.StartLatitude, value.StartLongitude, value.EndLatitude, value.EndLongitude,
			value.Price, value.Status, value.StartedAt, value.FinishedAt,
		).
		Suffix("RETURNING created_at, updated_at").
		ToSql()
	if err != nil {
		return Trip{}, fmt.Errorf("build create trip query: %w", err)
	}

	err = storage.Executor(ctx, r.pool).QueryRow(ctx, query, args...).Scan(&value.CreatedAt, &value.UpdatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "trips_active_driver_uidx" {
			return Trip{}, fmt.Errorf("create trip: %w", ErrDriverBusy)
		}
		return Trip{}, fmt.Errorf("create trip: %w", err)
	}
	return value, nil
}

func (r *Repository) Get(ctx context.Context, id uuid.UUID) (Trip, error) {
	return r.get(ctx, id, false)
}

func (r *Repository) GetForUpdate(ctx context.Context, id uuid.UUID) (Trip, error) {
	if _, ok := storage.Executor(ctx, r.pool).(pgx.Tx); !ok {
		return Trip{}, fmt.Errorf("get trip for update requires a transaction")
	}
	return r.get(ctx, id, true)
}

func (r *Repository) get(ctx context.Context, id uuid.UUID, lock bool) (Trip, error) {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	builder := sq.StatementBuilder.
		PlaceholderFormat(sq.Dollar).
		Select(
			"id", "user_id", "driver_id",
			"start_latitude", "start_longitude", "end_latitude", "end_longitude",
			"price", "status", "started_at", "finished_at", "created_at", "updated_at",
		).
		From("trips").
		Where(sq.Eq{"id": id})
	if lock {
		builder = builder.Suffix("FOR UPDATE")
	}
	query, args, err := builder.ToSql()
	if err != nil {
		return Trip{}, fmt.Errorf("build get trip query: %w", err)
	}

	var result Trip
	err = storage.Executor(ctx, r.pool).QueryRow(ctx, query, args...).Scan(
		&result.ID, &result.UserID, &result.DriverID,
		&result.StartLatitude, &result.StartLongitude, &result.EndLatitude, &result.EndLongitude,
		&result.Price, &result.Status, &result.StartedAt, &result.FinishedAt,
		&result.CreatedAt, &result.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Trip{}, fmt.Errorf("get trip %s: %w", id, ErrNotFound)
	}
	if err != nil {
		return Trip{}, fmt.Errorf("get trip %s: %w", id, err)
	}
	result.StartedAt = result.StartedAt.UTC()
	result.CreatedAt = result.CreatedAt.UTC()
	result.UpdatedAt = result.UpdatedAt.UTC()
	if result.FinishedAt != nil {
		finishedAt := result.FinishedAt.UTC()
		result.FinishedAt = &finishedAt
	}
	return result, nil
}

func (r *Repository) Complete(ctx context.Context, id uuid.UUID, finishedAt time.Time) error {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()
	query, args, err := sq.StatementBuilder.PlaceholderFormat(sq.Dollar).
		Update("trips").
		Set("status", "completed").
		Set("finished_at", finishedAt).
		Set("updated_at", finishedAt).
		Where(sq.Eq{"id": id, "status": "active"}).ToSql()
	if err != nil {
		return fmt.Errorf("build complete trip query: %w", err)
	}
	result, err := storage.Executor(ctx, r.pool).Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("complete trip: %w", err)
	}
	if result.RowsAffected() != 1 {
		return fmt.Errorf("complete trip: expected one locked active trip")
	}
	return nil
}
