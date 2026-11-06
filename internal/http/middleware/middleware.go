package middleware

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	"github.com/SPROG666/go-autumn-2026/internal/http/response"
)

type (
	stateKey     struct{}
	requestState struct {
		route string
		ctx   context.Context
	}
)

type Middleware struct {
	logger   *slog.Logger
	writer   *response.Writer
	tp       trace.TracerProvider
	duration metric.Float64Histogram
	active   metric.Int64UpDownCounter
}

func New(logger *slog.Logger, writer *response.Writer, tp trace.TracerProvider, meter metric.Meter) (*Middleware, error) {
	duration, err := meter.Float64Histogram("http.server.request.duration", metric.WithUnit("s"), metric.WithExplicitBucketBoundaries(.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10))
	if err != nil {
		return nil, fmt.Errorf("HTTP duration: %w", err)
	}
	active, err := meter.Int64UpDownCounter("http.server.active_requests", metric.WithUnit("{request}"))
	if err != nil {
		return nil, fmt.Errorf("HTTP active requests: %w", err)
	}
	return &Middleware{logger: logger, writer: writer, tp: tp, duration: duration, active: active}, nil
}

func (m *Middleware) Wrap(router chi.Router) http.Handler {
	return m.recover(m.tracing(m.logging(m.metrics(router))), router)
}

func (m *Middleware) recover(next http.Handler, router chi.Router) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buffer := &bufferedResponse{header: make(http.Header)}
		route := "unmatched"
		routeCtx := chi.NewRouteContext()
		if router.Match(routeCtx, r.Method, r.URL.Path) {
			route = routeCtx.RoutePattern()
		}
		state := &requestState{route: route, ctx: r.Context()}
		r = r.WithContext(context.WithValue(r.Context(), stateKey{}, state))
		defer func() {
			if recovered := recover(); recovered != nil {
				m.logger.ErrorContext(state.ctx, "request panic", "panic_type", fmt.Sprintf("%T", recovered), "stack", string(debug.Stack()))
				buffer = &bufferedResponse{header: make(http.Header)}
				m.writer.Problem(buffer, r, http.StatusInternalServerError, "internal_error", "internal-error", "Internal Server Error", "Internal server error")
			}
			for name, values := range buffer.header {
				w.Header()[name] = values
			}
			w.WriteHeader(responseStatus(buffer.status, true))
			if _, err := w.Write(buffer.body.Bytes()); err != nil {
				m.logger.ErrorContext(state.ctx, "flush response", "error", err)
			}
		}()
		next.ServeHTTP(buffer, r)
	})
}

type bufferedResponse struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func (w *bufferedResponse) Header() http.Header { return w.header }
func (w *bufferedResponse) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
}

func (w *bufferedResponse) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.body.Write(data)
}

func (m *Middleware) tracing(next http.Handler) http.Handler {
	wrapped := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		state := r.Context().Value(stateKey{}).(*requestState)
		state.ctx = r.Context()
		trace.SpanFromContext(r.Context()).SetAttributes(attribute.String("http.route", state.route))
		next.ServeHTTP(w, r)
	})
	return otelhttp.NewHandler(wrapped, "HTTP", otelhttp.WithTracerProvider(m.tp),
		otelhttp.WithMeterProvider(noop.NewMeterProvider()),
		otelhttp.WithPropagators(propagation.TraceContext{}),
		otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
			return r.Method + " " + r.Context().Value(stateKey{}).(*requestState).route
		}))
}

func (m *Middleware) logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		wrapped := chimiddleware.NewWrapResponseWriter(w, r.ProtoMajor)
		completed := false
		defer func() {
			status := responseStatus(wrapped.Status(), completed)
			level := slog.LevelInfo
			if status >= http.StatusInternalServerError {
				level = slog.LevelError
			}
			m.logger.Log(r.Context(), level, "request handled", "method", r.Method,
				"route", r.Context().Value(stateKey{}).(*requestState).route, "status", status, "duration_ms", time.Since(started).Milliseconds())
		}()
		next.ServeHTTP(wrapped, r)
		completed = true
	})
}

func (m *Middleware) metrics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attrs := []attribute.KeyValue{attribute.String("http.request.method", r.Method), attribute.String("http.route", r.Context().Value(stateKey{}).(*requestState).route)}
		m.active.Add(r.Context(), 1, metric.WithAttributes(attrs...))
		started := time.Now()
		wrapped := chimiddleware.NewWrapResponseWriter(w, r.ProtoMajor)
		completed := false
		defer func() {
			m.active.Add(r.Context(), -1, metric.WithAttributes(attrs...))
			status := responseStatus(wrapped.Status(), completed)
			m.duration.Record(r.Context(), time.Since(started).Seconds(), metric.WithAttributes(append(attrs, attribute.Int("http.response.status_code", status))...))
			if !completed {
				span := trace.SpanFromContext(r.Context())
				span.SetStatus(codes.Error, "internal_error")
				span.SetAttributes(attribute.String("error.type", "panic"), attribute.Int("http.response.status_code", status))
			}
		}()
		next.ServeHTTP(wrapped, r)
		completed = true
	})
}

func responseStatus(status int, completed bool) int {
	if !completed {
		return http.StatusInternalServerError
	}
	if status == 0 {
		return http.StatusOK
	}
	return status
}
