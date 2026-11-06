package middleware_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/otel/codes"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/SPROG666/go-autumn-2026/internal/http/middleware"
	"github.com/SPROG666/go-autumn-2026/internal/http/response"
)

func TestRequestSignals(t *testing.T) {
	for _, panics := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "panic"}[panics], func(t *testing.T) {
			ctx := context.Background()
			exporter := tracetest.NewInMemoryExporter()
			tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
			t.Cleanup(func() {
				if err := tp.Shutdown(ctx); err != nil {
					t.Error(err)
				}
			})
			reader := sdkmetric.NewManualReader()
			mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
			t.Cleanup(func() {
				if err := mp.Shutdown(ctx); err != nil {
					t.Error(err)
				}
			})
			var logs bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&logs, nil))
			wrappers, err := middleware.New(logger, response.New(logger), tp, mp.Meter("test"))
			if err != nil {
				t.Fatal(err)
			}
			router := chi.NewRouter()
			router.Get("/api/v1/trips/{tripId}", func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusAccepted)
				if panics {
					panic("injected failure")
				}
			})
			r := httptest.NewRequest("GET", "/api/v1/trips/secret-id", nil)
			r.Header.Set("traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
			w := httptest.NewRecorder()
			wrappers.Wrap(router).ServeHTTP(w, r)
			status := http.StatusAccepted
			if panics {
				status = http.StatusInternalServerError
			}
			if w.Code != status {
				t.Fatalf("status=%d; want %d", w.Code, status)
			}
			if panics && !strings.Contains(w.Body.String(), `"code":"internal_error"`) {
				t.Fatal(w.Body.String())
			}
			spans := exporter.GetSpans()
			if len(spans) != 1 || spans[0].Name != "GET /api/v1/trips/{tripId}" || spans[0].SpanContext.TraceID().String() != "4bf92f3577b34da6a3ce929d0e0e4736" {
				t.Fatalf("spans=%+v", spans)
			}
			if panics && spans[0].Status.Code != codes.Error {
				t.Fatal("panic span is not an error")
			}
			count := 0
			for _, line := range strings.Split(strings.TrimSpace(logs.String()), "\n") {
				var entry map[string]any
				if err := json.Unmarshal([]byte(line), &entry); err != nil {
					t.Fatal(err)
				}
				if entry["msg"] == "request handled" {
					count++
					if entry["status"] != float64(status) || entry["route"] != "/api/v1/trips/{tripId}" {
						t.Fatal(entry)
					}
				}
			}
			if count != 1 {
				t.Fatal("request log count", count)
			}
			var data metricdata.ResourceMetrics
			if err := reader.Collect(ctx, &data); err != nil {
				t.Fatal(err)
			}
			checkMetrics(t, data, status)
		})
	}
}

func checkMetrics(t *testing.T, data metricdata.ResourceMetrics, status int) {
	t.Helper()
	var foundDuration, foundActive bool
	for _, scope := range data.ScopeMetrics {
		for _, measurement := range scope.Metrics {
			switch measurement.Name {
			case "http.server.request.duration":
				points := measurement.Data.(metricdata.Histogram[float64]).DataPoints
				if len(points) != 1 || points[0].Count != 1 {
					t.Fatal("duration", points)
				}
				attrs := points[0].Attributes.ToSlice()
				for _, attr := range attrs {
					if attr.Key == "http.route" && attr.Value.AsString() != "/api/v1/trips/{tripId}" {
						t.Fatal("raw route", attr)
					}
					if attr.Key == "http.response.status_code" && attr.Value.AsInt64() != int64(status) {
						t.Fatal("wrong status", attr)
					}
				}
				foundDuration = true
			case "http.server.active_requests":
				points := measurement.Data.(metricdata.Sum[int64]).DataPoints
				if len(points) != 1 || points[0].Value != 0 {
					t.Fatal("active requests leaked", points)
				}
				foundActive = true
			}
		}
	}
	if !foundDuration || !foundActive {
		t.Fatal("missing HTTP metrics", foundDuration, foundActive)
	}
}
