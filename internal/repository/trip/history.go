package trip

import (
	"context"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"

	"github.com/SPROG666/go-autumn-2026/internal/infra/postgres/transaction"
)

type HistoryRepository struct {
	pool         database
	queryTimeout time.Duration
}

func NewHistoryRepository(pool database, queryTimeout time.Duration) *HistoryRepository {
	return &HistoryRepository{pool: pool, queryTimeout: queryTimeout}
}

func (r *HistoryRepository) Add(ctx context.Context, tripID uuid.UUID, from *string, to string, changedAt time.Time) error {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	query, args, err := sq.StatementBuilder.
		PlaceholderFormat(sq.Dollar).
		Insert("trip_status_history").
		Columns("trip_id", "from_status", "to_status", "changed_at").
		Values(tripID, from, to, changedAt).
		ToSql()
	if err != nil {
		return fmt.Errorf("build trip history query: %w", err)
	}
	if _, err := r.executor(ctx).Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("add trip history: %w", err)
	}
	return nil
}

func (r *HistoryRepository) executor(ctx context.Context) database {
	if tx, ok := transaction.Current(ctx); ok {
		return tx
	}
	return r.pool
}
