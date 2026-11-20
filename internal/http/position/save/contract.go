//go:generate go tool mockgen -source contract.go -destination mocks_test.go -package save_test
package save

import (
	"context"

	"github.com/google/uuid"

	"github.com/SPROG666/go-autumn-2026/internal/domain/position"
	"github.com/SPROG666/go-autumn-2026/internal/http/response"
)

type usecase interface {
	Execute(ctx context.Context, value position.Position) (position.Position, error)
}
type canceller interface{ Cancel(id uuid.UUID) }

type Handler struct {
	usecase  usecase
	queue    canceller
	response *response.Writer
}

func New(usecase usecase, queue canceller, writer *response.Writer) *Handler {
	return &Handler{usecase: usecase, queue: queue, response: writer}
}
