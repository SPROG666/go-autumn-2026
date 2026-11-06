package domainerror

import (
	"errors"
	"net/http"

	"github.com/SPROG666/go-autumn-2026/api"
	"github.com/SPROG666/go-autumn-2026/internal/domain/trip"
)

func Problem(err error) (int, api.Problem) {
	status, code, kind, title, detail := http.StatusInternalServerError, "internal_error", "internal-error", "Internal Server Error", "Internal server error"
	var validation *trip.ValidationError
	switch {
	case errors.As(err, &validation):
		status, code, kind, title, detail = http.StatusBadRequest, "invalid_request", "invalid-request", "Invalid request", validation.Error()
	case errors.Is(err, trip.ErrTripNotFound):
		status, code, kind, title, detail = http.StatusNotFound, "trip_not_found", "trip-not-found", "Trip not found", "Trip was not found"
	case errors.Is(err, trip.ErrDriverBusy):
		status, code, kind, title, detail = http.StatusConflict, "driver_busy", "driver-busy", "Driver busy", "Driver already has an active trip"
	case errors.Is(err, trip.ErrTripCompleted):
		status, code, kind, title, detail = http.StatusConflict, "trip_completed", "trip-completed", "Trip completed", "Operation is not allowed for a completed trip"
	case errors.Is(err, trip.ErrIdempotencyConflict):
		status, code, kind, title, detail = http.StatusConflict, "idempotency_conflict", "idempotency-conflict", "Idempotency conflict", "Idempotency key was already used with a different request"
	}
	return status, api.Problem{Type: "https://tripgo.example/problems/" + kind, Title: title, Status: int32(status), Code: code, Detail: &detail}
}
