package telemetry_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"

	"go.opentelemetry.io/otel"

	"github.com/SPROG666/go-autumn-2026/internal/config"
	"github.com/SPROG666/go-autumn-2026/internal/infra/telemetry"
)

func TestSDKErrorUsesJSONLogger(t *testing.T) {
	previous := otel.GetErrorHandler()
	t.Cleanup(func() { otel.SetErrorHandler(previous) })
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://127.0.0.1:1")
	t.Setenv("OTEL_RESOURCE_ATTRIBUTES", "")
	t.Setenv("OTEL_METRIC_EXPORT_INTERVAL", "60000")
	for _, protocol := range []string{"http/protobuf", "grpc"} {
		t.Run(protocol, func(t *testing.T) {
			var output bytes.Buffer
			logger := telemetry.NewLogger(&output, slog.LevelInfo, "telemetry-test")
			providers, err := telemetry.New(context.Background(), config.Config{
				OTelProtocol: protocol, OTelServiceName: "telemetry-test", ServiceVersion: "test",
			}, logger)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				if err := providers.Shutdown(ctx); err != nil && !errors.Is(err, context.Canceled) {
					t.Error(err)
				}
			})

			otel.Handle(errors.New("collector unavailable"))

			var entry map[string]any
			if err := json.Unmarshal(output.Bytes(), &entry); err != nil {
				t.Fatal(err)
			}
			if entry["level"] != "WARN" || entry["error"] != "collector unavailable" || entry["service"] != "telemetry-test" || entry["trace_id"] != "" || entry["span_id"] != "" {
				t.Fatal(entry)
			}
		})
	}
}
