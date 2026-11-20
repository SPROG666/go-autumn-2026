//go:generate go tool mockgen -source contract.go -destination mocks_test.go -package list_test
package list

import (
	"context"

	"github.com/google/uuid"

	"github.com/SPROG666/go-autumn-2026/internal/domain/position"
	"github.com/SPROG666/go-autumn-2026/internal/http/response"
)

type usecase interface {
	Execute(ctx context.Context, id uuid.UUID) ([]position.Position, error)
}

type Handler struct {
	usecase  usecase
	response *response.Writer
}

func New(usecase usecase, writer *response.Writer) *Handler {
	return &Handler{usecase: usecase, response: writer}
}
