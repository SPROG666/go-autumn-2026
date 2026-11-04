package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/google/uuid"

	"github.com/SPROG666/go-autumn-2026/api"
	"github.com/SPROG666/go-autumn-2026/internal/trip"
)

func (h *Handler) CreateTrip(w http.ResponseWriter, r *http.Request, params api.CreateTripParams) {
	var body api.CreateTripJSONRequestBody
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		h.InvalidRequest(w, r)
		return
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		h.InvalidRequest(w, r)
		return
	}
	if body.UserId == uuid.Nil || body.DriverId == uuid.Nil {
		h.InvalidRequest(w, r)
		return
	}

	value, replay, err := h.service.CreateWithKey(r.Context(), trip.CreateParams{
		UserID: body.UserId, DriverID: body.DriverId,
		StartLatitude: body.StartPoint.Latitude, StartLongitude: body.StartPoint.Longitude,
		EndLatitude: body.EndPoint.Latitude, EndLongitude: body.EndPoint.Longitude,
		Price: body.Price,
	}, params.IdempotencyKey)
	if err != nil {
		switch {
		case errors.Is(err, trip.ErrIdempotencyConflict):
			h.Problem(w, r, http.StatusConflict, "idempotency_conflict", "idempotency-conflict", "Idempotency conflict", "Idempotency key was already used with a different request")
		case errors.Is(err, trip.ErrDriverBusy):
			h.Problem(w, r, http.StatusConflict, "driver_busy", "driver-busy", "Driver busy", "Driver already has an active trip")
		default:
			h.logger.ErrorContext(r.Context(), "create trip failed", "error", err)
			h.Problem(w, r, http.StatusInternalServerError, "internal_error", "internal-error", "Internal Server Error", "Internal server error")
		}
		return
	}

	w.Header().Set("Location", "/api/v1/trips/"+value.ID.String())
	status := http.StatusCreated
	if replay {
		status = http.StatusOK
	}
	h.writeTrip(w, r, status, value)
}
