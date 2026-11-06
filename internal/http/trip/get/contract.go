//go:generate go tool mockgen -source contract.go -destination mocks_test.go -package get_test
package get

import (
	"context"

	"github.com/google/uuid"

	"github.com/SPROG666/go-autumn-2026/internal/http/response"
	"github.com/SPROG666/go-autumn-2026/internal/model"
)

type usecase interface {
	Execute(ctx context.Context, id uuid.UUID) (model.Trip, error)
}
type Handler struct {
	usecase  usecase
	response *response.Writer
}

func New(usecase usecase, writer *response.Writer) *Handler {
	return &Handler{usecase: usecase, response: writer}
}
