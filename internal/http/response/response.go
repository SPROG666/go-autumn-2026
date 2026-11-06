package response

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/SPROG666/go-autumn-2026/api"
	"github.com/SPROG666/go-autumn-2026/internal/domain/trip"
)

func (h *Writer) Trip(w http.ResponseWriter, r *http.Request, status int, value trip.Trip) {
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

func (h *Writer) InvalidRequest(w http.ResponseWriter, r *http.Request) {
	h.Problem(w, r, http.StatusBadRequest, "invalid_request", "invalid-request", "Invalid request", "Request validation failed")
}

func (h *Writer) Problem(w http.ResponseWriter, r *http.Request, status int32, code, kind, title, detail string) {
	if status >= http.StatusBadRequest {
		span := trace.SpanFromContext(r.Context())
		span.SetStatus(codes.Error, code)
		span.SetAttributes(attribute.String("error.type", code))
	}
	instance := r.URL.Path
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(int(status))
	if err := json.NewEncoder(w).Encode(api.Problem{
		Type:  "https://tripgo.example/problems/" + kind,
		Title: title, Status: status, Code: code,
		Detail: &detail, Instance: &instance,
	}); err != nil {
		h.logger.ErrorContext(r.Context(), "write problem response", "error", err)
	}
}

func (h *Writer) Health(w http.ResponseWriter, r *http.Request, code int, status api.HealthResponseStatus) {
	if code >= http.StatusBadRequest {
		span := trace.SpanFromContext(r.Context())
		span.SetStatus(codes.Error, string(status))
		span.SetAttributes(attribute.String("error.type", string(status)))
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(api.HealthResponse{Status: status}); err != nil {
		h.logger.ErrorContext(r.Context(), "write health response", "error", err)
	}
}

type Writer struct{ logger *slog.Logger }

func New(logger *slog.Logger) *Writer { return &Writer{logger: logger} }
