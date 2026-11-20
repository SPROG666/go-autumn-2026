package telemetry_test

import (
	"context"
	"testing"
	"time"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/SPROG666/go-autumn-2026/internal/infra/telemetry"
)

func TestPositionMetrics(t *testing.T) {
	ctx := context.Background()
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	t.Cleanup(func() {
		if err := provider.Shutdown(ctx); err != nil {
			t.Error(err)
		}
	})
	observer, err := telemetry.NewPositionObserver(provider.Meter("test"))
	if err != nil {
		t.Fatal(err)
	}
	observer.Saved(ctx)
	for _, result := range []string{"queued", "duplicate", "dropped"} {
		observer.Check(ctx, result)
	}
	observer.Queue(ctx, 10)
	observer.Queue(ctx, 0)
	observer.Active(ctx, 4)
	observer.Active(ctx, 0)
	observer.Requested(ctx, "success", 100*time.Millisecond)
	observer.Requested(ctx, "error", time.Second)
	observer.Dropped(ctx)
	var data metricdata.ResourceMetrics
	if err := reader.Collect(ctx, &data); err != nil {
		t.Fatal(err)
	}
	metrics := make(map[string]metricdata.Metrics)
	for _, scope := range data.ScopeMetrics {
		for _, metric := range scope.Metrics {
			metrics[metric.Name] = metric
		}
	}
	for _, name := range []string{"tripgo.positions.saved", "tripgo.stale_checks", "tripgo.position_requests", "tripgo.position_requests.duration", "tripgo.position_requests.queue.size", "tripgo.position_requests.workers.active"} {
		if _, ok := metrics[name]; !ok {
			t.Fatal("missing metric", name)
		}
	}
	saved := metrics["tripgo.positions.saved"].Data.(metricdata.Sum[int64])
	if len(saved.DataPoints) != 1 || saved.DataPoints[0].Value != 1 {
		t.Fatal("saved counter", saved)
	}
	for _, name := range []string{"tripgo.position_requests.queue.size", "tripgo.position_requests.workers.active"} {
		gauge := metrics[name].Data.(metricdata.Gauge[int64])
		if len(gauge.DataPoints) != 1 || gauge.DataPoints[0].Value != 0 {
			t.Fatal("gauge did not return to zero", name, gauge)
		}
	}
	for _, name := range []string{"tripgo.stale_checks", "tripgo.position_requests"} {
		sum := metrics[name].Data.(metricdata.Sum[int64])
		if len(sum.DataPoints) != 3 {
			t.Fatal(name, sum)
		}
		for _, point := range sum.DataPoints {
			if point.Value != 1 {
				t.Fatal("wrong counter", point)
			}
			if _, ok := point.Attributes.Value("result"); !ok {
				t.Fatal("missing result attribute")
			}
		}
	}
	histogram := metrics["tripgo.position_requests.duration"].Data.(metricdata.Histogram[float64])
	if len(histogram.DataPoints) != 2 || metrics["tripgo.position_requests.duration"].Unit != "s" {
		t.Fatal("duration histogram", histogram)
	}
}
