package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/SPROG666/go-autumn-2026/internal/config"
	"github.com/SPROG666/go-autumn-2026/internal/httpapi"
	"github.com/SPROG666/go-autumn-2026/internal/repository"
	"github.com/SPROG666/go-autumn-2026/internal/service"
	"github.com/SPROG666/go-autumn-2026/internal/storage"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "configuration error:", err)
		os.Exit(1)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: cfg.LogLevel,
	})).With("service", "trip-service")

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, cfg, logger); err != nil {
		logger.Error("service stopped with error", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, cfg config.Config, logger *slog.Logger) (runErr error) {
	pool, err := storage.OpenPool(ctx, cfg)
	if err != nil {
		return err
	}
	var shutdownDeadline time.Time
	defer func() {
		budget := cfg.ShutdownTimeout
		if !shutdownDeadline.IsZero() {
			budget = time.Until(shutdownDeadline)
		}
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), budget)
		defer cancel()
		closed := make(chan struct{})
		go func() {
			pool.Close()
			close(closed)
		}()
		select {
		case <-closed:
			logger.Info("PostgreSQL pool closed")
		case <-cleanupCtx.Done():
			runErr = errors.Join(runErr, fmt.Errorf("shutdown budget exhausted while closing PostgreSQL pool: %w", cleanupCtx.Err()))
		}
	}()
	logger.Info("PostgreSQL connection established")
	trips := repository.NewRepository(pool, cfg.DatabaseQueryTimeout)
	history := repository.NewHistoryRepository(pool, cfg.DatabaseQueryTimeout)
	tx := storage.NewTransactionManager(pool, cfg.DatabaseQueryTimeout)
	idempotency := repository.NewIdempotencyRepository(pool, cfg.DatabaseQueryTimeout, cfg.IdempotencyTTL)
	tripService := service.NewService(trips, history, tx, idempotency)
	router, err := httpapi.NewRouter(logger, pool, cfg.DatabaseQueryTimeout, tripService)
	if err != nil {
		return err
	}

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           router,
		ReadTimeout:       cfg.HTTPReadTimeout,
		ReadHeaderTimeout: cfg.HTTPReadHeaderTimeout,
		WriteTimeout:      cfg.HTTPWriteTimeout,
		IdleTimeout:       cfg.HTTPIdleTimeout,
	}

	var listenConfig net.ListenConfig
	listener, err := listenConfig.Listen(ctx, "tcp", cfg.HTTPAddr)
	if err != nil {
		return fmt.Errorf("listen HTTP: %w", err)
	}

	serveErrors := make(chan error, 1)
	go func() {
		serveErrors <- server.Serve(listener)
	}()
	logger.Info("HTTP server started", "addr", listener.Addr().String())

	select {
	case err := <-serveErrors:
		switch {
		case errors.Is(err, http.ErrServerClosed):
			return nil
		default:
			return fmt.Errorf("serve HTTP: %w", err)
		}
	case <-ctx.Done():
		shutdownDeadline = time.Now().Add(cfg.ShutdownTimeout)
		logger.Info("shutting down HTTP server")
	}

	shutdownCtx, cancel := context.WithDeadline(context.WithoutCancel(ctx), shutdownDeadline)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		_ = server.Close()
		return fmt.Errorf("graceful shutdown failed, HTTP connections closed: %w", err)
	}

	logger.Info("HTTP server stopped")
	return nil
}
