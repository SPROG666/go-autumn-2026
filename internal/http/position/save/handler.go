package save

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/SPROG666/go-autumn-2026/api"
	"github.com/SPROG666/go-autumn-2026/internal/domain/position"
)

func (h *Handler) CreateTripPosition(w http.ResponseWriter, r *http.Request, id api.TripId) {
	var body api.CreateTripPositionJSONRequestBody
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		h.response.InvalidRequest(w, r)
		return
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		h.response.InvalidRequest(w, r)
		return
	}
	value, err := h.usecase.Execute(r.Context(), position.Position{
		TripID: id, Latitude: body.Coordinates.Latitude, Longitude: body.Coordinates.Longitude, RecordedAt: body.RecordedAt,
	})
	if err != nil {
		h.response.Error(w, r, err)
		return
	}
	h.queue.Cancel(id)
	h.response.Position(w, r, value)
}
