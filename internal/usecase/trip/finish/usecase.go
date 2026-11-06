package finish

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/SPROG666/go-autumn-2026/internal/model"
)

type Usecase struct {
	trips    tripRepository
	history  historyRepository
	tx       txManager
	clock    clock
	observer observer
}

func New(trips tripRepository, history historyRepository, tx txManager, clock clock, observer observer) *Usecase {
	return &Usecase{trips: trips, history: history, tx: tx, clock: clock, observer: observer}
}

func (u *Usecase) Execute(ctx context.Context, id uuid.UUID) (model.Trip, error) {
	var value model.Trip
	err := u.tx.Do(ctx, func(ctx context.Context) error {
		var err error
		value, err = u.trips.GetForUpdate(ctx, id)
		if err != nil {
			return fmt.Errorf("lock trip: %w", err)
		}
		from := value.Status
		if err := value.Finish(u.clock.Now()); err != nil {
			return err
		}
		if err := u.trips.Complete(ctx, id, *value.FinishedAt); err != nil {
			return fmt.Errorf("complete trip: %w", err)
		}
		if err := u.history.Add(ctx, id, &from, value.Status, *value.FinishedAt); err != nil {
			return fmt.Errorf("save completed status: %w", err)
		}
		return nil
	})
	if err != nil {
		return model.Trip{}, fmt.Errorf("finish trip transaction: %w", err)
	}
	u.observer.Completed(ctx, value.ID)
	return value, nil
}
