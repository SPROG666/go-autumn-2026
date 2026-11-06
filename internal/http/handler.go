package http

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/SPROG666/go-autumn-2026/api"
	"github.com/SPROG666/go-autumn-2026/internal/http/response"
	"github.com/SPROG666/go-autumn-2026/internal/http/trip/create"
	"github.com/SPROG666/go-autumn-2026/internal/http/trip/finish"
	"github.com/SPROG666/go-autumn-2026/internal/http/trip/get"
)

type Handler struct {
	create       *create.Handler
	get          *get.Handler
	finish       *finish.Handler
	pool         databasePinger
	queryTimeout time.Duration
	writer       *response.Writer
	logger       *slog.Logger
}

func NewHandler(create *create.Handler, get *get.Handler, finish *finish.Handler, pool databasePinger, timeout time.Duration, writer *response.Writer, logger *slog.Logger) *Handler {
	return &Handler{create: create, get: get, finish: finish, pool: pool, queryTimeout: timeout, writer: writer, logger: logger}
}

func (h *Handler) CreateTrip(w http.ResponseWriter, r *http.Request, params api.CreateTripParams) {
	h.create.CreateTrip(w, r, params)
}

func (h *Handler) GetTrip(w http.ResponseWriter, r *http.Request, id api.TripId) {
	h.get.GetTrip(w, r, id)
}

func (h *Handler) FinishTrip(w http.ResponseWriter, r *http.Request, id api.TripId) {
	h.finish.FinishTrip(w, r, id)
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	h.writer.Health(w, r, http.StatusOK, api.Ok)
}

func (h *Handler) Ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), h.queryTimeout)
	defer cancel()
	if err := h.pool.Ping(ctx); err != nil {
		h.logger.WarnContext(r.Context(), "PostgreSQL readiness check failed", "error", err)
		h.writer.Health(w, r, http.StatusServiceUnavailable, api.Unavailable)
		return
	}
	h.writer.Health(w, r, http.StatusOK, api.Ok)
}
