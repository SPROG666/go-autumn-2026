//go:generate go tool mockgen -source contract.go -destination mocks_test.go -package finish_test
package finish

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/SPROG666/go-autumn-2026/internal/model"
)

type tripRepository interface {
	GetForUpdate(ctx context.Context, id uuid.UUID) (model.Trip, error)
	Complete(ctx context.Context, id uuid.UUID, at time.Time) error
}
type historyRepository interface {
	Add(ctx context.Context, id uuid.UUID, from *string, to string, at time.Time) error
}
type txManager interface {
	Do(ctx context.Context, fn func(context.Context) error) error
}
type (
	clock    interface{ Now() time.Time }
	observer interface {
		Completed(ctx context.Context, id uuid.UUID)
	}
)
