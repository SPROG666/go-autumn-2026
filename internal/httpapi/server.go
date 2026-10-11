package httpapi

import (
	"log/slog"
	"time"
)

type tripHandler struct {
	service      tripService
	logger       *slog.Logger
	pool         databasePinger
	queryTimeout time.Duration
}
