package get

import (
	"net/http"

	"github.com/SPROG666/go-autumn-2026/api"
)

func (h *Handler) GetTrip(w http.ResponseWriter, r *http.Request, tripID api.TripId) {
	value, err := h.usecase.Execute(r.Context(), tripID)
	if err != nil {
		h.response.Error(w, r, err)
		return
	}
	h.response.Trip(w, r, http.StatusOK, value)
}
