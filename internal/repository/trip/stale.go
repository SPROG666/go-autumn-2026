package trip

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/SPROG666/go-autumn-2026/internal/domain/position"
)

func (r *Repository) Stale(ctx context.Context, staleAfter time.Duration) ([]position.StaleTrip, error) {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()
	rows, err := r.executor(ctx).Query(ctx, `SELECT id, driver_id FROM trips
        WHERE status = 'active'
          AND COALESCE(last_position_at, started_at) < CURRENT_TIMESTAMP - $1 * INTERVAL '1 second'
        ORDER BY COALESCE(last_position_at, started_at), id`, staleAfter.Seconds())
	if err != nil {
		return nil, fmt.Errorf("select stale trips: %w", err)
	}
	defer rows.Close()
	values, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (position.StaleTrip, error) {
		var value position.StaleTrip
		err := row.Scan(&value.ID, &value.DriverID)
		return value, err
	})
	if err != nil {
		return nil, fmt.Errorf("read stale trips: %w", err)
	}
	return values, nil
}
