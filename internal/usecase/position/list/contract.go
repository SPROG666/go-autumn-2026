//go:generate go tool mockgen -source contract.go -destination mocks_test.go -package list_test
package list

import (
	"context"

	"github.com/google/uuid"

	"github.com/SPROG666/go-autumn-2026/internal/domain/position"
	"github.com/SPROG666/go-autumn-2026/internal/model"
)

type tripRepository interface {
	Get(ctx context.Context, id uuid.UUID) (model.Trip, error)
}
type positionRepository interface {
	List(ctx context.Context, id uuid.UUID) ([]position.Position, error)
}
