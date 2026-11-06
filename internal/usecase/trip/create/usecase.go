package create

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/SPROG666/go-autumn-2026/internal/domain/trip"
	"github.com/SPROG666/go-autumn-2026/internal/model"
)

type Usecase struct {
	trips       tripRepository
	history     historyRepository
	tx          txManager
	idempotency idempotencyRepository
	clock       clock
	ids         idGenerator
	observer    observer
}

func New(trips tripRepository, history historyRepository, tx txManager, keys idempotencyRepository, clock clock, ids idGenerator, observer observer) *Usecase {
	return &Usecase{trips: trips, history: history, tx: tx, idempotency: keys, clock: clock, ids: ids, observer: observer}
}

func (u *Usecase) Execute(ctx context.Context, params model.CreateParams, key *uuid.UUID) (model.Trip, bool, error) {
	if err := params.Validate(); err != nil {
		return model.Trip{}, false, fmt.Errorf("validate trip: %w", err)
	}
	if key != nil {
		if err := u.idempotency.CleanupExpired(ctx); err != nil {
			return model.Trip{}, false, fmt.Errorf("cleanup keys: %w", err)
		}
	}
	var value model.Trip
	var replay bool
	err := u.tx.Do(ctx, func(ctx context.Context) error {
		if key != nil {
			id, exists, err := u.idempotency.Reserve(ctx, *key, params)
			if err != nil {
				return fmt.Errorf("reserve key: %w", err)
			}
			if exists {
				replay = true
				value, err = u.trips.Get(ctx, id)
				return err
			}
		}
		var err error
		value, err = u.create(ctx, params)
		if err != nil {
			return err
		}
		if key != nil {
			return u.idempotency.Save(ctx, *key, value.ID)
		}
		return nil
	})
	if err != nil {
		return model.Trip{}, false, fmt.Errorf("create trip transaction: %w", err)
	}
	if !replay {
		u.observer.Created(ctx, value.ID)
	}
	return value, replay, nil
}

func (u *Usecase) create(ctx context.Context, params model.CreateParams) (model.Trip, error) {
	id, err := u.ids.New()
	if err != nil {
		return model.Trip{}, fmt.Errorf("generate trip ID: %w", err)
	}
	value, err := u.trips.Create(ctx, model.Trip{
		ID: id, UserID: params.UserID, DriverID: params.DriverID,
		StartLatitude: params.StartLatitude, StartLongitude: params.StartLongitude,
		EndLatitude: params.EndLatitude, EndLongitude: params.EndLongitude,
		Price: params.Price, Status: trip.StatusActive,
		StartedAt: u.clock.Now().UTC().Truncate(time.Microsecond),
	})
	if err != nil {
		return model.Trip{}, fmt.Errorf("save trip: %w", err)
	}
	if err := u.history.Add(ctx, value.ID, nil, value.Status, value.StartedAt); err != nil {
		return model.Trip{}, fmt.Errorf("save initial status: %w", err)
	}
	return value, nil
}
