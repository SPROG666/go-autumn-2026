package response

import (
	"encoding/json"
	"net/http"

	"github.com/SPROG666/go-autumn-2026/api"
	"github.com/SPROG666/go-autumn-2026/internal/domain/position"
)

func positionBody(value position.Position) api.TripPosition {
	return api.TripPosition{
		Id: value.ID, TripId: value.TripID,
		Coordinates: api.Coordinates{Latitude: value.Latitude, Longitude: value.Longitude}, RecordedAt: value.RecordedAt,
	}
}

func (h *Writer) Position(w http.ResponseWriter, r *http.Request, value position.Position) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(positionBody(value)); err != nil {
		h.logger.ErrorContext(r.Context(), "write position response", "error", err)
	}
}

func (h *Writer) Positions(w http.ResponseWriter, r *http.Request, values []position.Position) {
	result := make([]api.TripPosition, 0, len(values))
	for _, value := range values {
		result = append(result, positionBody(value))
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(api.TripPositionsResponse{Positions: result}); err != nil {
		h.logger.ErrorContext(r.Context(), "write positions response", "error", err)
	}
}
