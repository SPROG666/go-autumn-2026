package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/SPROG666/go-autumn-2026/internal/config"
	"github.com/SPROG666/go-autumn-2026/internal/container"
	"github.com/SPROG666/go-autumn-2026/internal/infra/telemetry"
)

func main() {
	if err := run(); err != nil {
		service := os.Getenv("OTEL_SERVICE_NAME")
		if service == "" {
			service = "trip-service"
		}
		logger := telemetry.NewLogger(os.Stdout, slog.LevelInfo, service)
		logger.Error("service stopped with error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("configuration error: %w", err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	app, err := container.New(ctx, cfg)
	if err != nil {
		return fmt.Errorf("initialize service: %w", err)
	}
	return app.Run(ctx)
}
