package telemetry

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/metric"
)

type TripObserver struct {
	logger    *slog.Logger
	created   metric.Int64Counter
	completed metric.Int64Counter
}

func NewTripObserver(logger *slog.Logger, meter metric.Meter) (*TripObserver, error) {
	created, err := meter.Int64Counter("tripgo.trips.created", metric.WithUnit("{trip}"))
	if err != nil {
		return nil, fmt.Errorf("created counter: %w", err)
	}
	completed, err := meter.Int64Counter("tripgo.trips.completed", metric.WithUnit("{trip}"))
	if err != nil {
		return nil, fmt.Errorf("completed counter: %w", err)
	}
	return &TripObserver{logger: logger, created: created, completed: completed}, nil
}

func (o *TripObserver) Created(ctx context.Context, id uuid.UUID) {
	o.created.Add(ctx, 1)
	o.logger.InfoContext(ctx, "trip created", "trip_id", id)
}

func (o *TripObserver) Completed(ctx context.Context, id uuid.UUID) {
	o.completed.Add(ctx, 1)
	o.logger.InfoContext(ctx, "trip completed", "trip_id", id)
}
