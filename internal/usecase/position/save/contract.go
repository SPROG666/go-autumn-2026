//go:generate go tool mockgen -source contract.go -destination mocks_test.go -package save_test
package save

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/SPROG666/go-autumn-2026/internal/domain/position"
	"github.com/SPROG666/go-autumn-2026/internal/model"
)

type tripRepository interface {
	GetForUpdate(ctx context.Context, id uuid.UUID) (model.Trip, error)
}
type positionRepository interface {
	Save(ctx context.Context, value position.Position) (position.Position, error)
}
type txManager interface {
	Do(ctx context.Context, fn func(context.Context) error) error
}
type (
	clock    interface{ Now() time.Time }
	observer interface{ Saved(ctx context.Context) }
)
