package telemetry

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/SPROG666/go-autumn-2026/internal/config"
)

type Providers struct {
	Traces  *sdktrace.TracerProvider
	Metrics *sdkmetric.MeterProvider
}

func New(ctx context.Context, cfg config.Config, logger *slog.Logger) (*Providers, error) {
	otel.SetErrorHandler(otel.ErrorHandlerFunc(func(err error) {
		logger.Warn("telemetry error", "error", err)
	}))
	res, err := resource.New(ctx, resource.WithFromEnv(), resource.WithAttributes(
		attribute.String("service.name", cfg.OTelServiceName),
		attribute.String("service.version", cfg.ServiceVersion),
	))
	if err != nil {
		return nil, fmt.Errorf("telemetry resource: %w", err)
	}
	var traces sdktrace.SpanExporter
	var metrics sdkmetric.Exporter
	if cfg.OTelProtocol == "grpc" {
		traces, err = otlptracegrpc.New(ctx)
	} else {
		traces, err = otlptracehttp.New(ctx)
	}
	if err != nil {
		return nil, fmt.Errorf("trace exporter: %w", err)
	}
	if cfg.OTelProtocol == "grpc" {
		metrics, err = otlpmetricgrpc.New(ctx)
	} else {
		metrics, err = otlpmetrichttp.New(ctx)
	}
	if err != nil {
		return nil, errors.Join(fmt.Errorf("metric exporter: %w", err), traces.Shutdown(ctx))
	}
	return &Providers{
		Traces:  sdktrace.NewTracerProvider(sdktrace.WithResource(res), sdktrace.WithBatcher(traces)),
		Metrics: sdkmetric.NewMeterProvider(sdkmetric.WithResource(res), sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metrics))),
	}, nil
}

func (p *Providers) Shutdown(ctx context.Context) error {
	return errors.Join(p.Metrics.Shutdown(ctx), p.Traces.Shutdown(ctx))
}
