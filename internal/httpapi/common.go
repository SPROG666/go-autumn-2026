package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/SPROG666/go-autumn-2026/api"
	"github.com/SPROG666/go-autumn-2026/internal/trip"
)

func (h *Handler) writeTrip(w http.ResponseWriter, r *http.Request, status int, value trip.Trip) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(api.Trip{
		Id: value.ID, UserId: value.UserID, DriverId: value.DriverID,
		StartPoint: api.Coordinates{Latitude: value.StartLatitude, Longitude: value.StartLongitude},
		EndPoint:   api.Coordinates{Latitude: value.EndLatitude, Longitude: value.EndLongitude},
		Price:      value.Price, Status: api.TripStatus(value.Status),
		StartedAt: value.StartedAt, FinishedAt: value.FinishedAt,
	}); err != nil {
		h.logger.ErrorContext(r.Context(), "write trip response", "error", err)
	}
}

func (h *Handler) InvalidRequest(w http.ResponseWriter, r *http.Request) {
	h.Problem(w, r, http.StatusBadRequest, "invalid_request", "invalid-request", "Invalid request", "Request validation failed")
}

func (h *Handler) Problem(w http.ResponseWriter, r *http.Request, status int, code, kind, title, detail string) {
	instance := r.URL.Path
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(api.Problem{
		Type:  "https://tripgo.example/problems/" + kind,
		Title: title, Status: int32(status), Code: code,
		Detail: &detail, Instance: &instance,
	}); err != nil {
		h.logger.ErrorContext(r.Context(), "write problem response", "error", err)
	}
}

func (h *Handler) writeHealth(w http.ResponseWriter, r *http.Request, code int, status api.HealthResponseStatus) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(api.HealthResponse{Status: status}); err != nil {
		h.logger.ErrorContext(r.Context(), "write health response", "error", err)
	}
}
