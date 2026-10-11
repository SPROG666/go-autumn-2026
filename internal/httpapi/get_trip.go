package httpapi

import (
	"errors"
	"net/http"

	"github.com/SPROG666/go-autumn-2026/api"
	"github.com/SPROG666/go-autumn-2026/internal/trip"
)

func (h *tripHandler) GetTrip(w http.ResponseWriter, r *http.Request, tripID api.TripId) {
	value, err := h.service.Get(r.Context(), tripID)
	if err != nil {
		switch {
		case errors.Is(err, trip.ErrNotFound):
			h.problem(w, r, http.StatusNotFound, "trip_not_found", "trip-not-found", "Trip not found", "Trip was not found")
		default:
			h.logger.ErrorContext(r.Context(), "get trip failed", "error", err)
			h.problem(w, r, http.StatusInternalServerError, "internal_error", "internal-error", "Internal Server Error", "Internal server error")
		}
		return
	}
	h.writeTrip(w, r, http.StatusOK, value)
}
