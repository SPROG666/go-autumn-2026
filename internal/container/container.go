package container

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel/metric"

	"github.com/SPROG666/go-autumn-2026/internal/config"
	httptransport "github.com/SPROG666/go-autumn-2026/internal/http"
	"github.com/SPROG666/go-autumn-2026/internal/http/middleware"
	"github.com/SPROG666/go-autumn-2026/internal/http/response"
	createhandler "github.com/SPROG666/go-autumn-2026/internal/http/trip/create"
	finishhandler "github.com/SPROG666/go-autumn-2026/internal/http/trip/finish"
	gethandler "github.com/SPROG666/go-autumn-2026/internal/http/trip/get"
	"github.com/SPROG666/go-autumn-2026/internal/infra/postgres"
	"github.com/SPROG666/go-autumn-2026/internal/infra/postgres/transaction"
	"github.com/SPROG666/go-autumn-2026/internal/infra/runtime"
	"github.com/SPROG666/go-autumn-2026/internal/infra/telemetry"
	"github.com/SPROG666/go-autumn-2026/internal/repository/trip"
	"github.com/SPROG666/go-autumn-2026/internal/usecase/trip/create"
	"github.com/SPROG666/go-autumn-2026/internal/usecase/trip/finish"
	"github.com/SPROG666/go-autumn-2026/internal/usecase/trip/get"
)

type App struct {
	Server    *http.Server
	Logger    *slog.Logger
	pool      *pgxpool.Pool
	telemetry *telemetry.Providers
	cfg       config.Config
}

func New(ctx context.Context, cfg config.Config) (_ *App, err error) {
	logger := telemetry.NewLogger(os.Stdout, cfg.LogLevel, cfg.OTelServiceName)
	providers, err := telemetry.New(ctx, cfg, logger)
	if err != nil {
		return nil, err
	}
	app := &App{Logger: logger, cfg: cfg, telemetry: providers}
	defer func() {
		if err != nil {
			cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), cfg.ShutdownTimeout)
			defer cancel()
			err = errors.Join(err, app.Close(cleanup))
		}
	}()
	tracer, err := postgres.NewTracer(providers.Traces, providers.Metrics)
	if err != nil {
		return nil, err
	}
	poolConfig, err := postgres.PoolConfig(cfg, tracer)
	if err != nil {
		return nil, err
	}
	app.pool, err = pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("create PostgreSQL pool: %w", err)
	}
	ping, cancel := context.WithTimeout(ctx, cfg.DatabaseQueryTimeout)
	defer cancel()
	if err := app.pool.Ping(ping); err != nil {
		return nil, fmt.Errorf("ping PostgreSQL at startup: %w", err)
	}

	repo := trip.NewRepository(app.pool, cfg.DatabaseQueryTimeout)
	history := trip.NewHistoryRepository(app.pool, cfg.DatabaseQueryTimeout)
	keys := trip.NewIdempotencyRepository(app.pool, cfg.DatabaseQueryTimeout, cfg.IdempotencyTTL)
	tx := transaction.NewTransactionManager(app.pool, cfg.DatabaseQueryTimeout)
	meter := providers.Metrics.Meter("trip-service")
	observer, err := telemetry.NewTripObserver(logger, meter)
	if err != nil {
		return nil, err
	}
	if err := app.activeTrips(meter, repo); err != nil {
		return nil, err
	}
	createTrip := create.New(repo, history, tx, keys, runtime.Clock{}, runtime.IDs{}, observer)
	getTrip := get.New(repo)
	finishTrip := finish.New(repo, history, tx, runtime.Clock{}, observer)
	writer := response.New(logger)
	handler := httptransport.NewHandler(createhandler.New(createTrip, writer), gethandler.New(getTrip, writer), finishhandler.New(finishTrip, writer), app.pool, cfg.DatabaseQueryTimeout, writer, logger)
	router, err := httptransport.NewRouter(logger, handler, writer)
	if err != nil {
		return nil, err
	}
	wrappers, err := middleware.New(logger, writer, providers.Traces, meter)
	if err != nil {
		return nil, err
	}
	app.Server = &http.Server{
		Addr: cfg.HTTPAddr, Handler: wrappers.Wrap(router),
		ReadTimeout: cfg.HTTPReadTimeout, ReadHeaderTimeout: cfg.HTTPReadHeaderTimeout,
		WriteTimeout: cfg.HTTPWriteTimeout, IdleTimeout: cfg.HTTPIdleTimeout,
	}
	return app, nil
}

func (a *App) activeTrips(meter metric.Meter, repo *trip.Repository) error {
	gauge, err := meter.Int64ObservableGauge("tripgo.trips.active", metric.WithUnit("{trip}"))
	if err != nil {
		return fmt.Errorf("active trips gauge: %w", err)
	}
	_, err = meter.RegisterCallback(func(ctx context.Context, observer metric.Observer) error {
		count, err := repo.CountActive(ctx)
		if err != nil {
			return fmt.Errorf("observe active trips: %w", err)
		}
		observer.ObserveInt64(gauge, count)
		return nil
	}, gauge)
	if err != nil {
		return fmt.Errorf("active trips callback: %w", err)
	}
	return nil
}
