package httpapi

import (
	"log/slog"
	"time"
)

type Handler struct {
	service      tripService
	logger       *slog.Logger
	pool         databasePinger
	queryTimeout time.Duration
}

func NewHandler(logger *slog.Logger, pool databasePinger, queryTimeout time.Duration, service tripService) *Handler {
	return &Handler{service: service, logger: logger, pool: pool, queryTimeout: queryTimeout}
}
