//go:generate go tool mockgen -source contract.go -destination mocks_test.go -package get_test
package get

import (
	"context"

	"github.com/google/uuid"

	"github.com/SPROG666/go-autumn-2026/internal/model"
)

type tripRepository interface {
	Get(ctx context.Context, id uuid.UUID) (model.Trip, error)
}
