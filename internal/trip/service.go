package trip

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type CreateParams struct {
	UserID         uuid.UUID
	DriverID       uuid.UUID
	StartLatitude  float64
	StartLongitude float64
	EndLatitude    float64
	EndLongitude   float64
	Price          int64
}

type Service struct {
	trips       *Repository
	history     *HistoryRepository
	tx          TxManager
	idempotency *IdempotencyRepository
}

func NewService(trips *Repository, history *HistoryRepository, tx TxManager, idempotency *IdempotencyRepository) *Service {
	return &Service{trips: trips, history: history, tx: tx, idempotency: idempotency}
}

func (s *Service) CreateWithKey(ctx context.Context, params CreateParams, key *uuid.UUID) (Trip, bool, error) {
	if key == nil {
		value, err := s.Create(ctx, params)
		return value, false, err
	}
	if err := s.idempotency.CleanupExpired(ctx); err != nil {
		return Trip{}, false, err
	}
	var value Trip
	var replay bool
	err := s.tx.Do(ctx, func(txCtx context.Context) error {
		var err error
		var tripID uuid.UUID
		tripID, replay, err = s.idempotency.Reserve(txCtx, *key, params)
		if err != nil {
			return err
		}
		if replay {
			value, err = s.trips.Get(txCtx, tripID)
			return err
		}
		value, err = s.Create(txCtx, params)
		if err != nil {
			return err
		}
		return s.idempotency.Save(txCtx, *key, value.ID)
	})
	if err != nil {
		return Trip{}, false, fmt.Errorf("create trip with idempotency key: %w", err)
	}
	return value, replay, nil
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (Trip, error) {
	return s.trips.Get(ctx, id)
}

func (s *Service) Finish(ctx context.Context, id uuid.UUID) (Trip, error) {
	var value Trip
	err := s.tx.Do(ctx, func(txCtx context.Context) error {
		var err error
		value, err = s.trips.GetForUpdate(txCtx, id)
		if err != nil {
			return err
		}
		if value.Status == "completed" {
			return ErrCompleted
		}
		finishedAt := time.Now().UTC().Truncate(time.Microsecond)
		if err := s.trips.Complete(txCtx, id, finishedAt); err != nil {
			return err
		}
		from := value.Status
		if err := s.history.Add(txCtx, id, &from, "completed", finishedAt); err != nil {
			return err
		}
		value.Status = "completed"
		value.FinishedAt = &finishedAt
		value.UpdatedAt = finishedAt
		return nil
	})
	if err != nil {
		return Trip{}, fmt.Errorf("finish trip with history: %w", err)
	}
	return value, nil
}

func (s *Service) Create(ctx context.Context, params CreateParams) (Trip, error) {
	id, err := uuid.NewRandom()
	if err != nil {
		return Trip{}, fmt.Errorf("generate trip ID: %w", err)
	}
	value := Trip{
		ID:             id,
		UserID:         params.UserID,
		DriverID:       params.DriverID,
		StartLatitude:  params.StartLatitude,
		StartLongitude: params.StartLongitude,
		EndLatitude:    params.EndLatitude,
		EndLongitude:   params.EndLongitude,
		Price:          params.Price,
		Status:         "active",
		StartedAt:      time.Now().UTC().Truncate(time.Microsecond),
	}

	err = s.tx.Do(ctx, func(txCtx context.Context) error {
		var err error
		value, err = s.trips.Create(txCtx, value)
		if err != nil {
			return err
		}
		return s.history.Add(txCtx, value.ID, nil, value.Status, value.StartedAt)
	})
	if err != nil {
		return Trip{}, fmt.Errorf("create trip with history: %w", err)
	}
	return value, nil
}
