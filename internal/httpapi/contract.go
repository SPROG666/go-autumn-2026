package httpapi

import (
	"context"

	"github.com/google/uuid"

	"github.com/SPROG666/go-autumn-2026/api"
	"github.com/SPROG666/go-autumn-2026/internal/trip"
)

type tripService interface {
	CreateWithKey(ctx context.Context, params trip.CreateParams, key *uuid.UUID) (trip.Trip, bool, error)
	Get(ctx context.Context, id uuid.UUID) (trip.Trip, error)
	Finish(ctx context.Context, id uuid.UUID) (trip.Trip, error)
}

type databasePinger interface {
	Ping(ctx context.Context) error
}

var _ api.ServerInterface = (*Handler)(nil)
