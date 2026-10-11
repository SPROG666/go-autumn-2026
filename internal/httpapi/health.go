package httpapi

import (
	"net/http"

	"github.com/SPROG666/go-autumn-2026/api"
)

func (h *tripHandler) Health(w http.ResponseWriter, r *http.Request) {
	h.writeHealth(w, r, http.StatusOK, api.Ok)
}
