//go:generate go tool mockgen -source contract.go -destination mocks_test.go -package create_test
package create

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/SPROG666/go-autumn-2026/internal/model"
)

type tripRepository interface {
	Create(ctx context.Context, value model.Trip) (model.Trip, error)
	Get(ctx context.Context, id uuid.UUID) (model.Trip, error)
}

type historyRepository interface {
	Add(ctx context.Context, id uuid.UUID, from *string, to string, at time.Time) error
}
type txManager interface {
	Do(ctx context.Context, fn func(context.Context) error) error
}
type idempotencyRepository interface {
	CleanupExpired(ctx context.Context) error
	Reserve(ctx context.Context, key uuid.UUID, params model.CreateParams) (uuid.UUID, bool, error)
	Save(ctx context.Context, key, id uuid.UUID) error
}
type (
	clock       interface{ Now() time.Time }
	idGenerator interface{ New() (uuid.UUID, error) }
	observer    interface {
		Created(ctx context.Context, id uuid.UUID)
	}
)
