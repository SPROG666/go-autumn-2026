//go:generate go tool mockgen -source contract.go -destination mocks_test.go -package create_test
package create

import (
	"context"

	"github.com/google/uuid"

	"github.com/SPROG666/go-autumn-2026/internal/http/response"
	"github.com/SPROG666/go-autumn-2026/internal/model"
)

type usecase interface {
	Execute(ctx context.Context, params model.CreateParams, key *uuid.UUID) (model.Trip, bool, error)
}
type Handler struct {
	usecase  usecase
	response *response.Writer
}

func New(usecase usecase, writer *response.Writer) *Handler {
	return &Handler{usecase: usecase, response: writer}
}
