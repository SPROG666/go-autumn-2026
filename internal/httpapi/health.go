package httpapi

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/SPROG666/go-autumn-2026/api"
)

func (h *tripHandler) Health(w http.ResponseWriter, r *http.Request) {
	h.writeHealth(w, r, http.StatusOK, api.Ok)
}

func (h *tripHandler) Ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), h.queryTimeout)
	defer cancel()
	if err := h.pool.Ping(ctx); err != nil {
		h.logger.WarnContext(r.Context(), "PostgreSQL readiness check failed", "error", err)
		h.writeHealth(w, r, http.StatusServiceUnavailable, api.Unavailable)
		return
	}
	h.writeHealth(w, r, http.StatusOK, api.Ok)
}

func (h *tripHandler) writeHealth(w http.ResponseWriter, r *http.Request, code int, status api.HealthResponseStatus) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(api.HealthResponse{Status: status}); err != nil {
		h.logger.ErrorContext(r.Context(), "write health response", "error", err)
	}
}
