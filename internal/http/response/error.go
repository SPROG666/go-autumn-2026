package response

import (
	"encoding/json"
	"net/http"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/SPROG666/go-autumn-2026/internal/http/domainerror"
)

func (h *Writer) Error(w http.ResponseWriter, r *http.Request, err error) {
	status, body := domainerror.Problem(err)
	span := trace.SpanFromContext(r.Context())
	span.SetAttributes(attribute.String("error.type", body.Code))
	span.SetStatus(codes.Error, body.Code)
	if status >= http.StatusInternalServerError {
		h.logger.ErrorContext(r.Context(), "request failed", "error", err)
	}
	instance := r.URL.Path
	body.Instance = &instance
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		h.logger.ErrorContext(r.Context(), "write problem response", "error", err)
	}
}
