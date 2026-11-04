package httpapi

import (
	"context"
	"net/http"

	"github.com/SPROG666/go-autumn-2026/api"
)

func (h *Handler) Ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), h.queryTimeout)
	defer cancel()
	if err := h.pool.Ping(ctx); err != nil {
		h.logger.WarnContext(r.Context(), "PostgreSQL readiness check failed", "error", err)
		h.writeHealth(w, r, http.StatusServiceUnavailable, api.Unavailable)
		return
	}
	h.writeHealth(w, r, http.StatusOK, api.Ok)
}
