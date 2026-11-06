package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/trace"
)

type (
	queryKey  struct{}
	queryInfo struct {
		started               time.Time
		operation, collection string
	}
)

type Tracer struct {
	base     *otelpgx.Tracer
	duration metric.Float64Histogram
}

func NewTracer(tp trace.TracerProvider, mp metric.MeterProvider) (*Tracer, error) {
	duration, err := mp.Meter("trip-service/postgres").Float64Histogram("db.client.operation.duration", metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(.001, .005, .01, .05, .1, .5, 1, 5, 10))
	if err != nil {
		return nil, fmt.Errorf("database duration: %w", err)
	}
	return &Tracer{base: otelpgx.NewTracer(otelpgx.WithTracerProvider(tp), otelpgx.WithMeterProvider(noop.NewMeterProvider()),
		otelpgx.WithDisableAcquireTracer(), otelpgx.WithDisableSQLStatementInAttributes(), otelpgx.WithDisableConnectionDetailsInAttributes()), duration: duration}, nil
}

func (*Tracer) query(sql string) queryInfo {
	words := strings.Fields(sql)
	info := queryInfo{started: time.Now()}
	if len(words) == 0 {
		return info
	}
	info.operation = strings.ToUpper(words[0])

	for _, table := range []string{"trips", "trip_status_history", "trip_idempotency"} {
		for _, word := range words {
			if strings.Trim(word, "\"(),") == table {
				info.collection = table
				return info
			}
		}
	}
	return info
}

func (t *Tracer) TraceQueryStart(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	ctx = t.base.TraceQueryStart(ctx, conn, data)
	info := t.query(data.SQL)
	trace.SpanFromContext(ctx).SetAttributes(attribute.String("db.operation.name", info.operation), attribute.String("db.collection.name", info.collection))
	return context.WithValue(ctx, queryKey{}, info)
}

func (t *Tracer) TraceQueryEnd(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryEndData) {
	info, ok := ctx.Value(queryKey{}).(queryInfo)
	if ok {
		errorType := ""
		if data.Err != nil {
			errorType = "database_error"
			var pgErr *pgconn.PgError
			switch {
			case errors.As(data.Err, &pgErr):
				errorType = pgErr.Code
			case errors.Is(data.Err, context.DeadlineExceeded):
				errorType = "timeout"
			case errors.Is(data.Err, context.Canceled):
				errorType = "canceled"
			}
			span := trace.SpanFromContext(ctx)
			span.SetStatus(codes.Error, errorType)
			span.SetAttributes(attribute.String("error.type", errorType))
		}
		t.duration.Record(ctx, time.Since(info.started).Seconds(), metric.WithAttributes(
			attribute.String("db.operation.name", info.operation), attribute.String("db.collection.name", info.collection), attribute.String("error.type", errorType),
		))
	}
	t.base.TraceQueryEnd(ctx, conn, data)
}
