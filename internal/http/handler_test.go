package http_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.uber.org/mock/gomock"

	"github.com/SPROG666/go-autumn-2026/api"
	httptransport "github.com/SPROG666/go-autumn-2026/internal/http"
	"github.com/SPROG666/go-autumn-2026/internal/http/response"
)

func TestHealthResponses(t *testing.T) {
	for _, test := range []struct {
		name      string
		ready     bool
		pingError error
		code      int
		status    api.HealthResponseStatus
		errorType string
	}{
		{name: "health does not query database", code: http.StatusOK, status: api.Ok},
		{name: "ready", ready: true, code: http.StatusOK, status: api.Ok},
		{name: "database unavailable", ready: true, pingError: errors.New("database unavailable"), code: http.StatusServiceUnavailable, status: api.Unavailable, errorType: "unavailable"},
	} {
		t.Run(test.name, func(t *testing.T) {
			pool := NewMockdatabasePinger(gomock.NewController(t))
			if test.ready {
				pool.EXPECT().Ping(gomock.Any()).DoAndReturn(func(ctx context.Context) error {
					if _, ok := ctx.Deadline(); !ok {
						t.Error("readiness Ping must have a deadline")
					}
					return test.pingError
				})
			}
			logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
			handler := httptransport.NewHandler(nil, nil, nil, pool, time.Second, response.New(logger), logger)
			exporter := tracetest.NewInMemoryExporter()
			provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
			t.Cleanup(func() {
				if err := provider.Shutdown(context.Background()); err != nil {
					t.Error(err)
				}
			})
			ctx, span := provider.Tracer("test").Start(context.Background(), test.name)
			request := httptest.NewRequestWithContext(ctx, http.MethodGet, "/ready", nil)
			writer := httptest.NewRecorder()
			if test.ready {
				handler.Ready(writer, request)
			} else {
				handler.Health(writer, request)
			}
			span.End()

			var body api.HealthResponse
			if err := json.Unmarshal(writer.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if writer.Code != test.code || body.Status != test.status || writer.Header().Get("Content-Type") != "application/json" {
				t.Fatalf("response=%d %s", writer.Code, writer.Body.String())
			}
			spans := exporter.GetSpans()
			if len(spans) != 1 {
				t.Fatalf("expected one span, got %d", len(spans))
			}
			wantStatus := codes.Unset
			if test.errorType != "" {
				wantStatus = codes.Error
			}
			if spans[0].Status.Code != wantStatus {
				t.Fatalf("span status=%v; want %v", spans[0].Status.Code, wantStatus)
			}
			var errorType string
			for _, attr := range spans[0].Attributes {
				if attr.Key == "error.type" {
					errorType = attr.Value.AsString()
				}
			}
			if errorType != test.errorType {
				t.Fatalf("error.type=%q; want %q", errorType, test.errorType)
			}
		})
	}
}
