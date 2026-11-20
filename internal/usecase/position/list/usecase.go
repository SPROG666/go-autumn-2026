package list

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/SPROG666/go-autumn-2026/internal/domain/position"
)

type Usecase struct {
	trips     tripRepository
	positions positionRepository
}

func New(trips tripRepository, positions positionRepository) *Usecase {
	return &Usecase{trips: trips, positions: positions}
}

func (u *Usecase) Execute(ctx context.Context, id uuid.UUID) ([]position.Position, error) {
	if _, err := u.trips.Get(ctx, id); err != nil {
		return nil, fmt.Errorf("get route trip: %w", err)
	}
	values, err := u.positions.List(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get route positions: %w", err)
	}
	return values, nil
}
