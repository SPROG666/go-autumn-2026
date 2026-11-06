package get

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/SPROG666/go-autumn-2026/internal/model"
)

type Usecase struct{ trips tripRepository }

func New(trips tripRepository) *Usecase { return &Usecase{trips: trips} }
func (u *Usecase) Execute(ctx context.Context, id uuid.UUID) (model.Trip, error) {
	value, err := u.trips.Get(ctx, id)
	if err != nil {
		return model.Trip{}, fmt.Errorf("get trip: %w", err)
	}
	return value, nil
}
