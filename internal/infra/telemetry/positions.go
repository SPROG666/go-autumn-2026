package telemetry

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type PositionObserver struct {
	saved    metric.Int64Counter
	checks   metric.Int64Counter
	requests metric.Int64Counter
	duration metric.Float64Histogram
	queue    metric.Int64Gauge
	active   metric.Int64Gauge
}

func NewPositionObserver(meter metric.Meter) (*PositionObserver, error) {
	var o PositionObserver
	var err error
	o.saved, err = meter.Int64Counter("tripgo.positions.saved")
	if err != nil {
		return nil, fmt.Errorf("position saved metric: %w", err)
	}
	o.checks, err = meter.Int64Counter("tripgo.stale_checks")
	if err != nil {
		return nil, fmt.Errorf("stale checks metric: %w", err)
	}
	o.requests, err = meter.Int64Counter("tripgo.position_requests")
	if err != nil {
		return nil, fmt.Errorf("position requests metric: %w", err)
	}
	o.duration, err = meter.Float64Histogram("tripgo.position_requests.duration", metric.WithUnit("s"))
	if err != nil {
		return nil, fmt.Errorf("position request duration metric: %w", err)
	}
	o.queue, err = meter.Int64Gauge("tripgo.position_requests.queue.size")
	if err != nil {
		return nil, fmt.Errorf("position queue metric: %w", err)
	}
	o.active, err = meter.Int64Gauge("tripgo.position_requests.workers.active")
	if err != nil {
		return nil, fmt.Errorf("position workers metric: %w", err)
	}
	return &o, nil
}

func (o *PositionObserver) Saved(ctx context.Context) { o.saved.Add(ctx, 1) }
func (o *PositionObserver) Check(ctx context.Context, result string) {
	o.checks.Add(ctx, 1, metric.WithAttributes(attribute.String("result", result)))
}
func (o *PositionObserver) Queue(ctx context.Context, size int64)   { o.queue.Record(ctx, size) }
func (o *PositionObserver) Active(ctx context.Context, count int64) { o.active.Record(ctx, count) }
func (o *PositionObserver) Dropped(ctx context.Context) {
	o.requests.Add(ctx, 1, metric.WithAttributes(attribute.String("result", "dropped")))
}

func (o *PositionObserver) Requested(ctx context.Context, result string, duration time.Duration) {
	attributes := metric.WithAttributes(attribute.String("result", result))
	o.requests.Add(ctx, 1, attributes)
	o.duration.Record(ctx, duration.Seconds(), attributes)
}
