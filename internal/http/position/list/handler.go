package list

import (
	"net/http"

	"github.com/SPROG666/go-autumn-2026/api"
)

func (h *Handler) ListTripPositions(w http.ResponseWriter, r *http.Request, id api.TripId) {
	values, err := h.usecase.Execute(r.Context(), id)
	if err != nil {
		h.response.Error(w, r, err)
		return
	}
	h.response.Positions(w, r, values)
}
