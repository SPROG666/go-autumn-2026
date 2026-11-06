package telemetry_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/SPROG666/go-autumn-2026/internal/infra/telemetry"
)

func TestLoggerTraceCorrelation(t *testing.T) {
	var output bytes.Buffer
	tp := sdktrace.NewTracerProvider()
	ctx, span := tp.Tracer("test").Start(context.Background(), "request")
	defer span.End()
	logger := telemetry.NewLogger(&output, slog.LevelInfo, "trip-service").With("component", "test")
	logger.InfoContext(ctx, "event")
	var entry map[string]any
	if err := json.Unmarshal(output.Bytes(), &entry); err != nil {
		t.Fatal(err)
	}
	if entry["trace_id"] != span.SpanContext().TraceID().String() || entry["span_id"] != span.SpanContext().SpanID().String() || entry["service"] != "trip-service" || entry["component"] != "test" {
		t.Fatal(entry)
	}
}
