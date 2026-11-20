package save

import (
	"context"
	"fmt"

	"github.com/SPROG666/go-autumn-2026/internal/domain/position"
	"github.com/SPROG666/go-autumn-2026/internal/domain/trip"
)

type Usecase struct {
	trips     tripRepository
	positions positionRepository
	tx        txManager
	clock     clock
	observer  observer
}

func New(trips tripRepository, positions positionRepository, tx txManager, clock clock, observer observer) *Usecase {
	return &Usecase{trips: trips, positions: positions, tx: tx, clock: clock, observer: observer}
}

func (u *Usecase) Execute(ctx context.Context, input position.Position) (position.Position, error) {
	if err := input.Validate(u.clock.Now()); err != nil {
		return position.Position{}, fmt.Errorf("validate position: %w", err)
	}
	var value position.Position
	err := u.tx.Do(ctx, func(ctx context.Context) error {
		valueTrip, err := u.trips.GetForUpdate(ctx, input.TripID)
		if err != nil {
			return fmt.Errorf("lock trip: %w", err)
		}
		if valueTrip.Status != trip.StatusActive {
			return trip.ErrTripCompleted
		}
		value, err = u.positions.Save(ctx, input)
		return err
	})
	if err != nil {
		return position.Position{}, fmt.Errorf("save position transaction: %w", err)
	}
	u.observer.Saved(ctx)
	return value, nil
}
