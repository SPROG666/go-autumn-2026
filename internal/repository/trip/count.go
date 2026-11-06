package trip

import (
	"context"
	"fmt"
)

func (r *Repository) CountActive(ctx context.Context) (int64, error) {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()
	var count int64
	if err := r.executor(ctx).QueryRow(ctx, "SELECT count(*) FROM trips WHERE status = $1", "active").Scan(&count); err != nil {
		return 0, fmt.Errorf("count active trips: %w", err)
	}
	return count, nil
}
