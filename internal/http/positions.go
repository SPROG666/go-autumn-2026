package http

import (
	"net/http"

	"github.com/SPROG666/go-autumn-2026/api"
	positionlist "github.com/SPROG666/go-autumn-2026/internal/http/position/list"
	positionsave "github.com/SPROG666/go-autumn-2026/internal/http/position/save"
)

func (h *Handler) WithPositions(save *positionsave.Handler, list *positionlist.Handler) *Handler {
	h.savePosition = save
	h.getPositions = list
	return h
}

func (h *Handler) CreateTripPosition(w http.ResponseWriter, r *http.Request, id api.TripId) {
	h.savePosition.CreateTripPosition(w, r, id)
}

func (h *Handler) ListTripPositions(w http.ResponseWriter, r *http.Request, id api.TripId) {
	h.getPositions.ListTripPositions(w, r, id)
}
