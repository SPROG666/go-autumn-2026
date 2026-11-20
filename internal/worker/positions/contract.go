//go:generate go tool mockgen -source contract.go -destination mocks_test.go -package positions_test
package positions

import (
	"context"
	"time"

	"github.com/SPROG666/go-autumn-2026/internal/domain/position"
)

type tripRepository interface {
	Stale(ctx context.Context, staleAfter time.Duration) ([]position.StaleTrip, error)
}

type gateway interface {
	RequestPosition(ctx context.Context, value position.StaleTrip) error
}

type Ticker interface {
	C() <-chan time.Time
	Stop()
}

type observer interface {
	Check(ctx context.Context, result string)
	Queue(ctx context.Context, size int64)
	Active(ctx context.Context, count int64)
	Requested(ctx context.Context, result string, duration time.Duration)
	Dropped(ctx context.Context)
}
