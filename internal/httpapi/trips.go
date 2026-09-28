package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/SPROG666/go-autumn-2026/api"
	"github.com/SPROG666/go-autumn-2026/internal/trip"
)

type tripHandler struct {
	service      *trip.Service
	logger       *slog.Logger
	pool         *pgxpool.Pool
	queryTimeout time.Duration
}

var _ api.ServerInterface = (*tripHandler)(nil)

func (h *tripHandler) CreateTrip(w http.ResponseWriter, r *http.Request, params api.CreateTripParams) {
	var body api.CreateTripJSONRequestBody
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		h.invalidRequest(w, r)
		return
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		h.invalidRequest(w, r)
		return
	}
	if body.UserId == uuid.Nil || body.DriverId == uuid.Nil {
		h.invalidRequest(w, r)
		return
	}

	value, replay, err := h.service.CreateWithKey(r.Context(), trip.CreateParams{
		UserID: body.UserId, DriverID: body.DriverId,
		StartLatitude: body.StartPoint.Latitude, StartLongitude: body.StartPoint.Longitude,
		EndLatitude: body.EndPoint.Latitude, EndLongitude: body.EndPoint.Longitude,
		Price: body.Price,
	}, params.IdempotencyKey)
	if errors.Is(err, trip.ErrIdempotencyConflict) {
		h.problem(w, r, http.StatusConflict, "idempotency_conflict", "idempotency-conflict", "Idempotency conflict", "Idempotency key was already used with a different request")
		return
	}
	if errors.Is(err, trip.ErrDriverBusy) {
		h.problem(w, r, http.StatusConflict, "driver_busy", "driver-busy", "Driver busy", "Driver already has an active trip")
		return
	}
	if err != nil {
		h.logger.ErrorContext(r.Context(), "create trip failed", "error", err)
		h.problem(w, r, http.StatusInternalServerError, "internal_error", "internal-error", "Internal Server Error", "Internal server error")
		return
	}

	w.Header().Set("Location", "/api/v1/trips/"+value.ID.String())
	status := http.StatusCreated
	if replay {
		status = http.StatusOK
	}
	h.writeTrip(w, r, status, value)
}

func (h *tripHandler) GetTrip(w http.ResponseWriter, r *http.Request, tripID api.TripId) {
	value, err := h.service.Get(r.Context(), tripID)
	if errors.Is(err, trip.ErrNotFound) {
		h.problem(w, r, http.StatusNotFound, "trip_not_found", "trip-not-found", "Trip not found", "Trip was not found")
		return
	}
	if err != nil {
		h.logger.ErrorContext(r.Context(), "get trip failed", "error", err)
		h.problem(w, r, http.StatusInternalServerError, "internal_error", "internal-error", "Internal Server Error", "Internal server error")
		return
	}
	h.writeTrip(w, r, http.StatusOK, value)
}

func (h *tripHandler) FinishTrip(w http.ResponseWriter, r *http.Request, tripID api.TripId) {
	value, err := h.service.Finish(r.Context(), tripID)
	if errors.Is(err, trip.ErrNotFound) {
		h.problem(w, r, http.StatusNotFound, "trip_not_found", "trip-not-found", "Trip not found", "Trip was not found")
		return
	}
	if errors.Is(err, trip.ErrCompleted) {
		h.problem(w, r, http.StatusConflict, "trip_completed", "trip-completed", "Trip completed", "Operation is not allowed for a completed trip")
		return
	}
	if err != nil {
		h.logger.ErrorContext(r.Context(), "finish trip failed", "error", err)
		h.problem(w, r, http.StatusInternalServerError, "internal_error", "internal-error", "Internal Server Error", "Internal server error")
		return
	}
	h.writeTrip(w, r, http.StatusOK, value)
}

func (h *tripHandler) writeTrip(w http.ResponseWriter, r *http.Request, status int, value trip.Trip) {
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

func (h *tripHandler) invalidRequest(w http.ResponseWriter, r *http.Request) {
	h.problem(w, r, http.StatusBadRequest, "invalid_request", "invalid-request", "Invalid request", "Request validation failed")
}

func (h *tripHandler) problem(w http.ResponseWriter, r *http.Request, status int, code, kind, title, detail string) {
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
